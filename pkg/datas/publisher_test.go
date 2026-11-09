package datas

import (
	"testing"

	"google.golang.org/protobuf/types/known/structpb"
)

func TestBuildInstanceDataPatchOnlyNamedKeys(t *testing.T) {
	t.Parallel()

	data := map[string]*structpb.Value{
		"vmid":            structpb.NewNumberValue(6788),
		"vm_name":         structpb.NewStringValue("vm"),
		"last_monitoring": structpb.NewNumberValue(100),
	}

	patch := BuildInstanceDataPatch(data, "vmid", "vm_name")
	if patch == nil {
		t.Fatal("expected patch")
	}
	if !patch[PatchFlagKey].GetBoolValue() {
		t.Fatal("_patch must be set")
	}
	if got := int(patch["vmid"].GetNumberValue()); got != 6788 {
		t.Fatalf("vmid = %d", got)
	}
	if _, ok := patch["last_monitoring"]; ok {
		t.Fatal("unrelated keys must not be copied")
	}
}

func TestBuildInstanceDataPatchEmpty(t *testing.T) {
	t.Parallel()

	if got := BuildInstanceDataPatch(nil, "vmid"); got != nil {
		t.Fatal("nil data must yield nil patch")
	}
	data := map[string]*structpb.Value{"vmid": structpb.NewNumberValue(1)}
	if got := BuildInstanceDataPatch(data, "vm_name"); got != nil {
		t.Fatal("missing keys must yield nil patch")
	}
}
