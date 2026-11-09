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
	"google.golang.org/protobuf/types/known/structpb"
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

func TestVMIsLive(t *testing.T) {
	t.Parallel()

	running := &vm.VM{ID: 6788, Name: "9eca09b0-69d5-4e0c-877d-c315d1e85d1c", StateRaw: int(vm.Active), LCMStateRaw: int(vm.Running)}
	finished := &vm.VM{ID: 6788, Name: "9eca09b0-69d5-4e0c-877d-c315d1e85d1c", StateRaw: int(vm.Done), LCMStateRaw: int(vm.LcmInit)}
	unknown := &vm.VM{ID: 1, StateRaw: -1, LCMStateRaw: int(vm.LcmInit)}

	if !vmIsLive(running, nil) {
		t.Fatal("running VM must count as live")
	}
	if vmIsLive(finished, nil) {
		t.Fatal("finished VM must not count as live")
	}
	if !vmIsLive(unknown, nil) {
		t.Fatal("VM with an unreadable LCM state must still count as live")
	}
	if vmIsLive(nil, nil) {
		t.Fatal("missing VM must not count as live")
	}
	if vmIsLive(running, errors.New("not found")) {
		t.Fatal("lookup error must not count as live")
	}
}

func TestRememberLiveVMKeepsExistingData(t *testing.T) {
	t.Parallel()

	inst := &ipb.Instance{
		Uuid: "9eca09b0-69d5-4e0c-877d-c315d1e85d1c",
		Data: map[string]*structpb.Value{
			"last_monitoring":      structpb.NewNumberValue(100),
			"pending_notification": structpb.NewBoolValue(true),
		},
	}

	if !rememberLiveVM(inst, 6788, inst.Uuid) {
		t.Fatal("first bind must report a link write")
	}

	if got := int(inst.Data["vmid"].GetNumberValue()); got != 6788 {
		t.Fatalf("vmid = %d, want 6788", got)
	}
	if got := inst.Data["vm_name"].GetStringValue(); got != inst.Uuid {
		t.Fatalf("vm_name = %q, want instance uuid", got)
	}
	if got := inst.Data["last_monitoring"].GetNumberValue(); got != 100 {
		t.Fatalf("last_monitoring = %v, want 100", got)
	}
	if !inst.Data["pending_notification"].GetBoolValue() {
		t.Fatal("pending_notification was dropped")
	}
}

func TestRememberLiveVMDoesNotReplaceLink(t *testing.T) {
	t.Parallel()

	inst := &ipb.Instance{
		Data: map[string]*structpb.Value{
			"vmid":    structpb.NewNumberValue(1),
			"vm_name": structpb.NewStringValue("kept"),
		},
	}

	if rememberLiveVM(inst, 6788, "9eca09b0-69d5-4e0c-877d-c315d1e85d1c") {
		t.Fatal("existing vmid/vm_name must not be replaced")
	}

	if got := int(inst.Data["vmid"].GetNumberValue()); got != 1 {
		t.Fatalf("vmid = %d, want existing 1", got)
	}
	if got := inst.Data["vm_name"].GetStringValue(); got != "kept" {
		t.Fatalf("vm_name = %q, want kept", got)
	}
}

func TestRememberLiveVMInitializesData(t *testing.T) {
	t.Parallel()

	inst := &ipb.Instance{}
	if !rememberLiveVM(inst, 6788, "") {
		t.Fatal("vmid write must report a link write")
	}

	if inst.Data == nil {
		t.Fatal("data map was not created")
	}
	if got := int(inst.Data["vmid"].GetNumberValue()); got != 6788 {
		t.Fatalf("vmid = %d, want 6788", got)
	}
	if _, ok := inst.Data["vm_name"]; ok {
		t.Fatal("empty VM name must not be stored")
	}
}
