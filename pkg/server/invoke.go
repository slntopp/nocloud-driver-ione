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
	"fmt"

	epb "github.com/slntopp/nocloud-proto/events"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/slntopp/nocloud-driver-ione/pkg/actions"
	one "github.com/slntopp/nocloud-driver-ione/pkg/driver"
	accesspb "github.com/slntopp/nocloud-proto/access"
	pb "github.com/slntopp/nocloud-proto/drivers/instance/vanilla"
	ipb "github.com/slntopp/nocloud-proto/instances"
	"github.com/slntopp/nocloud-proto/services_providers"
	spb "github.com/slntopp/nocloud-proto/services_providers"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *DriverServiceServer) Invoke(ctx context.Context, req *pb.InvokeRequest) (res *ipb.InvokeResponse, err error) {
	s.log.Debug("Invoke request received", zap.Any("instance", req.Instance.Uuid), zap.Any("action", req.Method), zap.Any("data", req.Params))
	sp := req.GetServicesProvider()
	client, err := one.NewClientFromSP(sp, s.log)
	instance := req.GetInstance()
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Error making client: %v", err)
	}

	method := req.GetMethod()

	if _, ok := actions.AdminActions[method]; ok && instance.Access.GetLevel() < accesspb.Level_ROOT {
		return nil, status.Errorf(codes.PermissionDenied, "Action %s is admin action", method)
	}

	action, ok := actions.Actions[method]
	if !ok {
		s.log.Warn(fmt.Sprintf("Action %s not declared for %s", method, DRIVER_TYPE))
	} else {

		if method == "suspend" {
			go s.HandlePublishEvents(ctx, &epb.Event{
				Uuid: instance.GetUuid(),
				Key:  "instance_suspended",
				Data: map[string]*structpb.Value{},
			})
		}

		return action(client, instance, req.GetParams())
	}

	action, ok = actions.BillingActions[method]
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "Action '%s' not declared for %s", method, DRIVER_TYPE)
	}

	response, err := action(client, instance, req.GetParams())
	if err != nil {
		return nil, err
	} else {
		go handleManualRenewBilling(s.log, s.HandlePublishRecords, instance)
	}

	return response, err
}

func (s *DriverServiceServer) SpInvoke(ctx context.Context, req *pb.SpInvokeRequest) (res *spb.InvokeResponse, err error) {
	s.log.Debug("Invoke request received", zap.Any("action", req.Method), zap.Any("data", req.Params))
	sp := req.GetServicesProvider()
	client, err := one.NewClientFromSP(sp, s.log)

	method := req.GetMethod()

	action, ok := actions.SpActions[method]
	if !ok {
		return nil, fmt.Errorf("action '%s' not declared for %s", req.GetMethod(), DRIVER_TYPE)
	}

	response, err := action(client, req.GetParams())
	if err != nil {
		return nil, err
	}

	return response, err
}

func (s *DriverServiceServer) SpPrep(ctx context.Context, req *services_providers.PrepSP) (res *services_providers.PrepSP, err error) {
	log := s.log.Named("ServicesProvider Preparation")
	log.Debug("ServicesProvider Preparation request received", zap.Any("sp", req.Sp), zap.Any("extra", req.Extra))

	sp := req.GetSp()
	client, err := one.NewClientFromSP(sp, log)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Error making client: %v", err)
	}

	state, _, err := client.MonitorLocation(sp)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Error gathering Data: %v", err)
	}
	req.Extra = state.Meta

	return req, nil
}
