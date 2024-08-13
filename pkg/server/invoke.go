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
	"errors"
	"fmt"
	"time"

	"github.com/slntopp/nocloud-driver-ione/pkg/datas"
	"github.com/slntopp/nocloud-proto/ansible"
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

	runningPlaybook := instance.GetData()["running_playbook"].GetStringValue()

	if runningPlaybook != "" {
		get, err := s.ansibleClient.Get(s.ansibleCtx, &ansible.GetRunRequest{
			Uuid: runningPlaybook,
		})
		if err != nil {
			return nil, err
		}
		if get.GetStatus() == "running" {
			return nil, errors.New("playbook still running")
		}
		if get.GetStatus() == "successful" || get.GetStatus() == "failed" || get.GetStatus() == "undefined" {
			instance.Data["running_playbook"] = structpb.NewStringValue("")
			go datas.DataPublisher(datas.POST_INST_DATA)(instance.GetUuid(), instance.GetData())
		}
	}

	action, ok := actions.BillingActions[method]
	if ok {
		if method == "manual_renew" {
			time.Sleep(time.Duration(3) * time.Second)
			return &ipb.InvokeResponse{Result: true}, nil
			go handleManualRenewBilling(s.log, s.HandlePublishRecords, instance)
		} else {
			return action(client, instance, req.GetParams())
		}
		return &ipb.InvokeResponse{Result: true}, err
	}

	if req.GetInstance().GetData()["freeze"].GetBoolValue() && req.GetMethod() != "unfreeze" {
		return nil, status.Error(codes.Canceled, "Instance is freeze")
	}

	action, ok = actions.Actions[method]
	if ok {
		if method == "suspend" {
			go s.HandlePublishEvents(ctx, &epb.Event{
				Uuid: instance.GetUuid(),
				Key:  "instance_suspended",
				Data: map[string]*structpb.Value{},
			})
		}

		return action(client, instance, req.GetParams())
	}

	ansibleAction, ok := actions.AnsibleActions[method]
	if ok {
		secrets := sp.GetSecrets()
		ansibleSecret, ok := secrets["ansible"]
		if !ok {
			return nil, status.Errorf(codes.InvalidArgument, "No ansible config")
		}

		ansibleSecretValue := ansibleSecret.GetStructValue().AsMap()
		playbookUuid, ok := ansibleSecretValue["playbook_uuid"].(string)
		if !ok {
			return nil, status.Errorf(codes.InvalidArgument, "No ansible playbook")
		}
		hop, ok := ansibleSecretValue["hop"].(map[string]any)
		if !ok {
			return nil, status.Errorf(codes.InvalidArgument, "No ansible playbook")
		}

		return ansibleAction(s.ansibleCtx, s.ansibleClient, playbookUuid, hop, instance, req.GetParams())
	}

	return nil, status.Errorf(codes.PermissionDenied, "Action %s is not declared", method)
}

func (s *DriverServiceServer) SpInvoke(ctx context.Context, req *pb.SpInvokeRequest) (res *spb.InvokeResponse, err error) {
	s.log.Debug("Invoke request received", zap.Any("action", req.Method), zap.Any("data", req.Params))
	sp := req.GetServicesProvider()
	client, err := one.NewClientFromSP(sp, s.log)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Error making client: %v", err)
	}

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
