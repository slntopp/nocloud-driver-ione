package one

import (
	"testing"

	pb "github.com/slntopp/nocloud-proto/instances"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestInstanceDataKeepsExistingKeys(t *testing.T) {
	t.Parallel()

	inst := &pb.Instance{
		Data: map[string]*structpb.Value{
			"last_monitoring": structpb.NewNumberValue(100),
			"serviceId":       structpb.NewNumberValue(42),
		},
	}

	data := instanceData(inst)
	data[DATA_VM_NAME] = structpb.NewStringValue(inst.GetUuid())
	data[DATA_VM_ID] = structpb.NewNumberValue(6788)

	if got := inst.Data["last_monitoring"].GetNumberValue(); got != 100 {
		t.Fatalf("last_monitoring = %v, want 100", got)
	}
	if got := inst.Data["serviceId"].GetNumberValue(); got != 42 {
		t.Fatalf("serviceId = %v, want 42", got)
	}
	if got := int(inst.Data[DATA_VM_ID].GetNumberValue()); got != 6788 {
		t.Fatalf("vmid = %d", got)
	}
}

func TestInstanceDataInitializesNil(t *testing.T) {
	t.Parallel()

	inst := &pb.Instance{}
	data := instanceData(inst)
	if data == nil || inst.Data == nil {
		t.Fatal("data map was not created")
	}
	data[DATA_VM_ID] = structpb.NewNumberValue(1)
	if inst.Data[DATA_VM_ID].GetNumberValue() != 1 {
		t.Fatal("writes must land on instance.Data")
	}
}
