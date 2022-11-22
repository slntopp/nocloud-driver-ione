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
