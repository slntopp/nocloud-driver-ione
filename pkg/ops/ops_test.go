package ops

import (
	"testing"

	ipb "github.com/slntopp/nocloud-proto/instances"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestSpecDiffDiskDoesNotAddCPU(t *testing.T) {
	from := map[string]*structpb.Value{
		"cpu":        structpb.NewNumberValue(1),
		"ram":        structpb.NewNumberValue(2048),
		"drive_size": structpb.NewNumberValue(20480),
	}
	to := map[string]*structpb.Value{
		"cpu":        structpb.NewNumberValue(1),
		"ram":        structpb.NewNumberValue(2048),
		"drive_size": structpb.NewNumberValue(40960),
	}
	got := SpecDiff(from, to)
	if len(got) != 1 || got[0].Name != Disk {
		t.Fatalf("got %#v, want only disk_resize", got)
	}
}

func TestHasVMID(t *testing.T) {
	if HasVMID(&ipb.Instance{}) {
		t.Fatal("empty")
	}
	inst := &ipb.Instance{Data: map[string]*structpb.Value{"vmid": structpb.NewNumberValue(9)}}
	if !HasVMID(inst) {
		t.Fatal("expected vmid")
	}
}
