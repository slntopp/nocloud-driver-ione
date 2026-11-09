package datas

import (
	stpb "github.com/slntopp/nocloud-proto/states"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	POST_INST_DATA      = "POST_INST_DATA"
	POST_IG_DATA        = "POST_IG_DATA"
	POST_INST_STATE     = "POST_INST_STATE"
	POST_SP_STATE       = "POST_SP_STATE"
	POST_SP_PUBLIC_DATA = "POST_SP_PUBLIC_DATA"
)

const PatchFlagKey = "_patch"

func BuildInstanceDataPatch(data map[string]*structpb.Value, keys ...string) map[string]*structpb.Value {
	if data == nil {
		return nil
	}
	patch := make(map[string]*structpb.Value, len(keys)+1)
	for _, k := range keys {
		if k == "" || k == PatchFlagKey {
			continue
		}
		if v, ok := data[k]; ok {
			patch[k] = v
		}
	}
	if len(patch) == 0 {
		return nil
	}
	patch[PatchFlagKey] = structpb.NewBoolValue(true)
	return patch
}

func PublishInstanceDataPatch(uuid string, data map[string]*structpb.Value, keys ...string) {
	if uuid == "" {
		return
	}
	patch := BuildInstanceDataPatch(data, keys...)
	if patch == nil {
		return
	}
	DataPublisher(POST_INST_DATA)(uuid, patch)
}

func DataPublisher(pubType string) func(string, map[string]*structpb.Value) {
	if pubType == POST_INST_DATA {
		return postInstData
	}
	if pubType == POST_IG_DATA {
		return postIGData
	}
	if pubType == POST_SP_PUBLIC_DATA {
		return postSPPublicData
	}
	return nil
}

func StatePublisher(pubType string) func(string, *stpb.State) {
	if pubType == POST_INST_STATE {
		return postInstState
	}
	if pubType == POST_SP_STATE {
		return postSPState
	}
	return nil
}
