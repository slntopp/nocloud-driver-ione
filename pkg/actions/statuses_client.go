/*
Copyright © 2021-2022 Nikita Ivanovski info@slnt-opp.xyz

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package actions

import (
	"context"
	"strings"

	one "github.com/slntopp/nocloud-driver-ione/pkg/driver"
	instpb "github.com/slntopp/nocloud/pkg/instances/proto"
	srvpb "github.com/slntopp/nocloud/pkg/services/proto"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

var (
	grpc_client instpb.StatesServiceClient
	log  *zap.Logger
)

func ConfigureStatusesClient(logger *zap.Logger, client instpb.StatesServiceClient) {
	log = logger.Named("Statuses")
	grpc_client = client
}

var STATES_REF = map[int32]instpb.NoCloudState{
	0: instpb.NoCloudState_INIT, // INIT
	1: instpb.NoCloudState_INIT, // PENDING
	2: instpb.NoCloudState_INIT, // HOLD
	4: instpb.NoCloudState_STOPPED, // STOPPED
	5: instpb.NoCloudState_SUSPENDED, // SUSPENDED
	6: instpb.NoCloudState_DELETED, // DONE
	8: instpb.NoCloudState_STOPPED, // POWEROFF
	9: instpb.NoCloudState_INIT, // UNDEPLOYED
	10: instpb.NoCloudState_OPERATION, // CLONING
	11: instpb.NoCloudState_FAILURE, // CLONING_FAILURE
}

var LCM_STATE_REF = map[int32]instpb.NoCloudState{
	0: instpb.NoCloudState_INIT, // INIT
	1: instpb.NoCloudState_INIT, // PENDING
	2: instpb.NoCloudState_INIT, // HOLD
	3: instpb.NoCloudState_RUNNING, // RUNNING
}

// Returns the VM state of the VirtualMachine to statuses server
func StatusesClient(
	client *one.ONeClient,
	inst *instpb.Instance,
	data map[string]*structpb.Value,
	result *srvpb.PerformActionResponse,
) (*srvpb.PerformActionResponse, error) {
	log.Debug("StatusesClient request received")

	par, err := State(client, nil, inst, data)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Can't get State VM, error: %v", err)
	}

	result.Meta = par.Meta

	request := &instpb.PostStateRequest{
		Uuid:  inst.GetUuid(),
		State: &instpb.State{
			State: instpb.NoCloudState_UNKNOWN,
			Meta:  result.Meta,
		},
	}
	
	oneState := int32(result.Meta["state"].GetNumberValue())
	oneLcmState := int32(result.Meta["lcm_state"].GetNumberValue())

	res, ok := STATES_REF[oneState]
	if !ok {
		r, ok := LCM_STATE_REF[oneLcmState]
		if ok {
			res = r
			goto post
		}

		if strings.HasSuffix(result.Meta["lcm_state_str"].GetStringValue(), "FAILURE") {
			res = instpb.NoCloudState_FAILURE
		} else if strings.HasSuffix(result.Meta["lcm_state_str"].GetStringValue(), "UNKNOWN")  {
			res = instpb.NoCloudState_UNKNOWN
		} else {
			res = instpb.NoCloudState_OPERATION
		}
	}

	post:
	request.State.State = res
	_, err = grpc_client.PostState(context.Background(), request)
	if err != nil {
		log.Error("Failed to post Instance State", zap.Error(err))
	}

	return &srvpb.PerformActionResponse{Result: result.Result, Meta: result.Meta}, nil
}
