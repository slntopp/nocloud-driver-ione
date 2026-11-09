package utils

import (
	"testing"

	"google.golang.org/protobuf/types/known/structpb"
)

func TestBillingDatePatchDropsVMID(t *testing.T) {
	data := map[string]*structpb.Value{
		"vmid":            structpb.NewNumberValue(1),
		"last_monitoring": structpb.NewNumberValue(10),
	}
	patch := BillingDatePatch(data)
	if _, ok := patch["vmid"]; ok {
		t.Fatal("vmid must not be in billing patch")
	}
	if patch["last_monitoring"].GetNumberValue() != 10 {
		t.Fatal("dates must be kept")
	}
	if !patch["_patch"].GetBoolValue() {
		t.Fatal("_patch")
	}
}
