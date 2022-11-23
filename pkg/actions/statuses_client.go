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
	"github.com/slntopp/nocloud-driver-ione/pkg/datas"
	one "github.com/slntopp/nocloud-driver-ione/pkg/driver"
	ipb "github.com/slntopp/nocloud-proto/instances"
	stpb "github.com/slntopp/nocloud-proto/states"
	"strings"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

var (
	log *zap.Logger
)

func ConfigureStatusesClient(logger *zap.Logger) {
	log = logger.Named("Client statuses")
}

var STATES_REF = map[int32]stpb.NoCloudState{
	0:  stpb.NoCloudState_INIT,      // INIT
	1:  stpb.NoCloudState_INIT,      // PENDING
	2:  stpb.NoCloudState_INIT,      // HOLD
	4:  stpb.NoCloudState_STOPPED,   // STOPPED
	5:  stpb.NoCloudState_SUSPENDED, // SUSPENDED
	6:  stpb.NoCloudState_DELETED,   // DONE
	8:  stpb.NoCloudState_STOPPED,   // POWEROFF
	9:  stpb.NoCloudState_INIT,      // UNDEPLOYED
	10: stpb.NoCloudState_OPERATION, // CLONING
	11: stpb.NoCloudState_FAILURE,   // CLONING_FAILURE
}

var LCM_STATE_REF = map[int32]stpb.NoCloudState{
	0: stpb.NoCloudState_INIT,    // INIT
	1: stpb.NoCloudState_INIT,    // PENDING
	2: stpb.NoCloudState_INIT,    // HOLD
	3: stpb.NoCloudState_RUNNING, // RUNNING
}

// Returns the VM state of the VirtualMachine to statuses server
func StatusesClient(
	client one.IClient,
	inst *ipb.Instance,
	data map[string]*structpb.Value,
	result *ipb.InvokeResponse,
) (*ipb.InvokeResponse, error) {
	log.Debug("StatusesClient request received")

	par, err := State(client, inst, data)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Can't get State VM, error: %v", err)
	}

	result.Meta = par.Meta

	request := MakePostStateRequest(inst.GetUuid(), par.Meta)
	err = datas.StIPub(request)
	if err != nil {
		log.Error("Failed to post State", zap.Error(err))
	}

	return &ipb.InvokeResponse{Result: result.Result, Meta: result.Meta}, nil
}

func MakePostStateRequest(uuid string, meta map[string]*structpb.Value) *stpb.ObjectState {
	request := &stpb.ObjectState{
		Uuid: uuid,
		State: &stpb.State{
			State: stpb.NoCloudState_UNKNOWN,
			Meta:  meta,
		},
	}

	oneState := int32(meta["state"].GetNumberValue())
	oneLcmState := int32(meta["lcm_state"].GetNumberValue())

	res, ok := STATES_REF[oneState]
	if !ok {
		r, ok := LCM_STATE_REF[oneLcmState]
		if ok {
			request.State.State = r
			return request
		}

		if strings.HasSuffix(meta["lcm_state_str"].GetStringValue(), "FAILURE") {
			res = stpb.NoCloudState_FAILURE
		} else if strings.HasSuffix(meta["lcm_state_str"].GetStringValue(), "UNKNOWN") {
			res = stpb.NoCloudState_UNKNOWN
		} else {
			res = stpb.NoCloudState_OPERATION
		}
	}
	request.State.State = res
	return request
}
