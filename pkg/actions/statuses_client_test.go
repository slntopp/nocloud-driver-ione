package actions

import (
	"testing"

	stpb "github.com/slntopp/nocloud-proto/states"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestMakePostStateRequestUsesLCM(t *testing.T) {
	meta := map[string]*structpb.Value{
		"state":     structpb.NewNumberValue(3),
		"lcm_state": structpb.NewNumberValue(3),
	}
	req := MakePostStateRequest("u", meta)
	if req.State.State != stpb.NoCloudState_RUNNING {
		t.Fatalf("state=%v", req.State.State)
	}
}
