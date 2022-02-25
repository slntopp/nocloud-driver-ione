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

	one "github.com/slntopp/nocloud-driver-ione/pkg/driver"
	instpb "github.com/slntopp/nocloud/pkg/instances/proto"
	srvpb "github.com/slntopp/nocloud/pkg/services/proto"

	sspb "github.com/slntopp/nocloud/pkg/statuses/proto"
	// sspb "../../../nocloud/pkg/statuses/proto"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

var (
	grpc_client sspb.PostServiceClient
	log  *zap.Logger
)

func ConfigureStatusesClient(logger *zap.Logger, client sspb.PostServiceClient) {
	log = logger.Named("Statuses")
	grpc_client = client
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

	if result.Meta == nil {
		result.Meta = make(map[string]*structpb.Value)
	}

	result.Meta = par.Meta

	_, err = grpc_client.State(context.Background(), &sspb.PostServiceStateRequest{
		Uuid: inst.GetUuid(),
		State: int32(result.Meta["state"].GetNumberValue()),
		Meta:  result.Meta,
	})
	if err != nil {
		log.Error("fail to send statuses:", zap.Error(err))
	}

	return &srvpb.PerformActionResponse{Result: result.Result, Meta: result.Meta}, nil
}
