package server

import (
	"errors"
	"testing"
	"time"

	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/shared"
	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/virtualnetwork"
	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/vm"
	one "github.com/slntopp/nocloud-driver-ione/pkg/driver"
	"github.com/slntopp/nocloud-driver-ione/pkg/utils"
	billingpb "github.com/slntopp/nocloud-proto/billing"
	ipb "github.com/slntopp/nocloud-proto/instances"
	stpb "github.com/slntopp/nocloud-proto/states"
	"github.com/slntopp/nocloud/pkg/nocloud"
)

type TestClock struct {
	time time.Time
}

type TestNetworkClient struct {
	one.IClient
}

func (c TestNetworkClient) GetVNet(id int) (*virtualnetwork.VirtualNetwork, error) {
	privateTemplate := virtualnetwork.Template{}
	privateTemplate.Add("TYPE", "PRIVATE")

	publicTemplate := virtualnetwork.Template{}
	publicTemplate.Add("TYPE", "PUBLIC")

	networks := map[int]*virtualnetwork.VirtualNetwork{
		1: {
			Template: privateTemplate,
		},
		2: {
			Template: publicTemplate,
		},
		3: {
			Template: privateTemplate,
		},
		4: {
			Template: publicTemplate,
		},
	}
	net, ok := networks[id]
	if !ok {
		return net, errors.New("Can't get network")
	}
	return net, nil
}

func (c *TestClock) Now() time.Time { return c.time }

func TestResourceKeyToDriveKind(t *testing.T) {
	type Test struct {
		input string
		want  string
		fails bool
	}
	tests := []Test{
		{input: "drive_ssd", want: "SSD"},
		{input: "drive_hdd", want: "HDD"},
		{input: "drive_nvme", want: "NVME"},
		{input: "drive_type_foo_bar_nvme", want: "NVME"},
		{input: "foo_nvme", want: "UNKNOWN"},
		{input: "drive", want: "UNKNOWN"},
	}
	for _, test := range tests {
		res, err := resourceKeyToDriveKind(test.input)
		if err != nil && !test.fails {
			t.Errorf("input: %s, wanted: %s, got: %v", test.input, test.want, err)
		}
		if res != test.want {
			t.Errorf("input: %s, wanted: %s, got: %s", test.input, test.want, res)
		}
	}
}

func TestSingletone(t *testing.T) {
	counter := 0
	f := func() int {
		counter += 1
		return counter
	}

	obj := Lazy(f)

	if obj() != obj() {
		t.Errorf("Singletone object has been created twice")
	}
}

func TestHandleCapacityBilling(t *testing.T) {
	type Test struct {
		// Input
		clock  utils.IClock
		amount func() float64
		res    *billingpb.ResourceConf
		ltl    LazyTimeline
		i      *ipb.Instance
		prev   int64
		// Want
		records []*billingpb.Record
		last    int64
	}

	tests := []Test{
		{
			last:   120,
			prev:   60,
			clock:  &TestClock{time: time.Unix(135, 0)},
			i:      &ipb.Instance{Uuid: "1"},
			amount: func() float64 { return 2.0 },
			ltl:    func() []one.Record { return []one.Record{{Start: 58, End: 131, State: 1}} },
			res: &billingpb.ResourceConf{
				On:     []stpb.NoCloudState{1},
				Kind:   1,
				Period: 60,
				Price:  1.0,
			},
			records: []*billingpb.Record{{Total: 2.0}},
		},
		{
			last:   960,
			prev:   60,
			clock:  &TestClock{time: time.Unix(1000, 0)},
			i:      &ipb.Instance{Uuid: "1"},
			amount: func() float64 { return 2.0 },
			ltl:    func() []one.Record { return []one.Record{{Start: 61, End: 62, State: 1}} },
			res: &billingpb.ResourceConf{
				On:     []stpb.NoCloudState{1},
				Kind:   1,
				Period: 60,
				Price:  1.0,
			},
			records: []*billingpb.Record{{Total: 0.0}},
		},
	}
	log := nocloud.NewLogger()
	for _, test := range tests {
		records, last := handleCapacityBilling(log, test.amount, test.ltl, test.i, test.res, test.prev, test.clock)
		if len(records) != len(test.records) {
			t.Error("Amount of payment records doesn't match")
		}
		if last != test.last {
			t.Errorf("Last billing handling timestamp doesn't match. Wanted %d, got %d", test.last, last)
		}

		wantSum := 0.0
		sum := 0.0

		for i := range records {
			sum += records[i].Total
			wantSum += test.records[i].Total
		}

		if wantSum != sum {
			t.Errorf("Total sums don't match. Wanted %f, got %f", wantSum, sum)
		}

	}
}

func TestHandleIPBilling(t *testing.T) {

	type Test struct {
		// Input
		ltl    LazyTimeline
		i      *ipb.Instance
		vm     LazyVM
		res    *billingpb.ResourceConf
		client one.IClient
		prev   int64
		clock  utils.IClock
		// Want
		records []*billingpb.Record
		last    int64
	}

	tests := []Test{
		{
			client: TestNetworkClient{},
			vm: func() (*vm.VM, error) {
				template := vm.NewTemplate()

				first := template.AddVector("NIC")
				first.AddPair(string(shared.NetworkID), 1)

				second := template.AddVector("NIC")
				second.AddPair(string(shared.NetworkID), 2)

				third := template.AddVector("NIC")
				third.AddPair(string(shared.NetworkID), 3)

				fourth := template.AddVector("NIC")
				fourth.AddPair(string(shared.NetworkID), 4)

				vm := &vm.VM{
					Template: *template,
				}

				return vm, nil
			},

			last:  120,
			prev:  60,
			clock: &TestClock{time: time.Unix(135, 0)},
			i:     &ipb.Instance{Uuid: "1"},
			ltl:   func() []one.Record { return []one.Record{{Start: 58, End: 131, State: 1}} },
			res: &billingpb.ResourceConf{
				On:     []stpb.NoCloudState{1},
				Kind:   1,
				Period: 60,
				Price:  1.0,
			},
			records: []*billingpb.Record{{Total: 2.0}},
		},
	}

	log := nocloud.NewLogger()
	for _, test := range tests {
		records, last := handleIPBilling(log, test.ltl, test.i, test.vm, test.res, test.client, test.prev, test.clock)
		if len(records) != len(test.records) {
			t.Error("Amount of payment records doesn't match")
		}
		if last != test.last {
			t.Errorf("Last billing handling timestamp doesn't match. Wanted %d, got %d", test.last, last)
		}

		wantSum := 0.0
		sum := 0.0

		for i := range records {
			sum += records[i].Total
			wantSum += test.records[i].Total
		}

		if wantSum != sum {
			t.Errorf("Total sums don't match. Wanted %f, got %f", wantSum, sum)
		}
	}
}
