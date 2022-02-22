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
package server

import (
	"context"

	"github.com/slntopp/nocloud-driver-ione/pkg/actions"
	one "github.com/slntopp/nocloud-driver-ione/pkg/driver"
	pb "github.com/slntopp/nocloud/pkg/drivers/instance/vanilla"
	instpb "github.com/slntopp/nocloud/pkg/instances/proto"
	srvpb "github.com/slntopp/nocloud/pkg/services/proto"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *DriverServiceServer) Invoke(ctx context.Context, req *pb.PerformActionRequest) (res *srvpb.PerformActionResponse, err error) {
	s.log.Debug("Invoke request received", zap.Any("req", req))
	sp := req.GetServicesProvider()
	igroup := req.GetGroup()
	client, err := one.NewClientFromSP(sp, s.log)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Error making client: %v", err)
	}

	request := req.GetRequest()
	var inst *instpb.Instance
	for _, i := range igroup.GetInstances() {
		if i.GetUuid() == request.GetInstance() {
			inst = i
		}
	}
	if inst == nil {
		return nil, status.Errorf(codes.NotFound, "Instance '%s' not found", request.GetInstance())
	}

	action, ok := actions.Actions[request.GetAction()]
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "Action '%s' not declared for %s", request.GetAction(), DRIVER_TYPE)
	}
	return action(client, igroup, inst, request.GetData())
}
