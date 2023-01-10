package one

import (
	"github.com/OpenNebula/one/src/oca/go/src/goca"
	stpb "github.com/slntopp/nocloud-proto/states"
	"testing"
)

func TestFilterTimeline(t *testing.T) {
	conf := goca.NewConfig("user", "pass", "host")
	c := goca.NewClient(conf, nil)
	ctrl := goca.NewController(c)

	machine, err := ctrl.VM(6165).Info(true)

	if err != nil {
		t.Errorf("Cannot get vm")
	}

	timeline := MakeTimeline(machine)

	expectedLen := 4

	if len(timeline) != expectedLen {
		t.Errorf("Wrong num of records")
	}

	filtered := FilterTimeline(timeline, 1673376000, 1673378500)
	expectedLenFiltered := 2

	if len(filtered) != expectedLenFiltered {
		t.Errorf("Wrong num of filtered records")
	}

	if filtered[0].State != stpb.NoCloudState_SUSPENDED {
		t.Errorf("Wrong state of record")
	}

	if filtered[len(filtered)-1].State != stpb.NoCloudState_RUNNING {
		t.Errorf("Wrong state of record")
	}
}
