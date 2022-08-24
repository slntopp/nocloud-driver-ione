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
	ipb "github.com/slntopp/nocloud/pkg/instances/proto"
	"github.com/slntopp/nocloud/pkg/nocloud/access"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *DriverServiceServer) Invoke(ctx context.Context, req *pb.InvokeRequest) (res *ipb.InvokeResponse, err error) {
	s.log.Debug("Invoke request received", zap.Any("instance", req.Instance.Uuid), zap.Any("action", req.Method))
	sp := req.GetServicesProvider()
	client, err := one.NewClientFromSP(sp, s.log)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Error making client: %v", err)
	}

	method := req.GetMethod()

	if _, ok := actions.AdminActions[method]; ok && (req.Instance.AccessLevel == nil || *req.Instance.AccessLevel < access.SUDO) {
		return nil, status.Errorf(codes.PermissionDenied, "Action %s is admin action", method)
	}

	action, ok := actions.Actions[method]

	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "Action '%s' not declared for %s", method, DRIVER_TYPE)
	}
	return action(client, req.Instance, req.GetParams())
}
