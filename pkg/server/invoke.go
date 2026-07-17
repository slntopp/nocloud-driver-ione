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
	"github.com/slntopp/nocloud-driver-ione/pkg/datas"
	"github.com/slntopp/nocloud-proto/ansible"
	epb "github.com/slntopp/nocloud-proto/events"
	"google.golang.org/protobuf/types/known/structpb"
	"time"

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

	action, ok := actions.BillingActions[method]
	if ok {
		if method == "manual_renew" {
			go func() {
				handleManualRenewBilling(s.log, s.HandlePublishRecords, instance)
				s.resumeAfterRenew(client, instance)
			}()
			return &ipb.InvokeResponse{Result: true}, nil
		}
		resp, err := action(client, instance, req.GetParams())
		// On a successful renewal, bring the VM back up immediately instead of
		// waiting for the next monitoring tick to notice the moved dates.
		if err == nil && resp.GetResult() && method == "free_renew" {
			s.resumeAfterRenew(client, instance)
		}
		return resp, err
	}

	// Check for running backup
	allowedActionsWhileBackup := map[string]bool{
		"monitoring": true,
	}
	runningPlaybook := instance.GetData()["running_playbook"].GetStringValue()
	runningPlaybookStart := instance.GetData()["running_playbook_start"].GetNumberValue()
	if allowed := allowedActionsWhileBackup[req.GetMethod()]; runningPlaybook != "" && !allowed {
		if runningPlaybookStart != 0 && int64(runningPlaybookStart)+86400*2 < time.Now().Unix() {
			instance.Data["running_playbook"] = structpb.NewStringValue("")
			instance.Data["running_playbook_start"] = structpb.NewNumberValue(0)
			go datas.DataPublisher(datas.POST_INST_DATA)(instance.GetUuid(), instance.GetData())
		} else {
			get, err := s.ansibleClient.Get(s.ansibleCtx, &ansible.GetRunRequest{
				Uuid: runningPlaybook,
			})
			if err != nil {
				return nil, err
			}
			if get.GetStatus() == "running" || get.GetStatus() == "init" {
				return nil, status.Error(codes.Unavailable, "backup is still running")
			}
			if get.GetStatus() == "successful" || get.GetStatus() == "failed" || get.GetStatus() == "undefined" {
				instance.Data["running_playbook"] = structpb.NewStringValue("")
				instance.Data["running_playbook_start"] = structpb.NewNumberValue(0)
				go datas.DataPublisher(datas.POST_INST_DATA)(instance.GetUuid(), instance.GetData())
			}
		}
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
		return ansibleAction(s.ansibleCtx, s.ansibleClient, ansibleSecretValue, instance, req.GetParams(), sp)
	}

	return nil, status.Errorf(codes.PermissionDenied, "Action %s is not declared", method)
}

// resumeAfterRenew powers a VM back on right after a renewal (free_renew /
// manual_renew) if it is currently suspended for non-payment. Without this the
// VM would stay down until the next monitoring tick re-evaluated the (now moved
// forward) last_monitoring date. It mirrors the auto-resume condition used by
// the monitoring billing handlers and does not read anything back from the DB —
// it operates on the instance snapshot already in hand.
func (s *DriverServiceServer) resumeAfterRenew(client one.IClient, inst *ipb.Instance) {
	log := s.log.Named("resumeAfterRenew").Named(inst.GetUuid())

	data := inst.GetData()
	if data == nil || data["freeze"].GetBoolValue() {
		return
	}

	vmid, err := one.GetVMIDFromData(client, inst)
	if err != nil {
		log.Warn("Skip auto-resume after renew: failed to get VM ID", zap.Error(err))
		return
	}

	_, state, _, _, err := client.StateVM(vmid)
	if err != nil {
		log.Warn("Skip auto-resume after renew: could not get VM state", zap.Int("vmid", vmid), zap.Error(err))
		return
	}
	if state != "SUSPENDED" {
		return
	}

	_, hasSuspendTime := data["suspend_time"]
	suspendedManually := data["suspended_manually"].GetBoolValue()
	// Don't touch a VM an admin suspended by hand (manual suspend with no
	// billing suspend_time). Auto-suspends always carry suspend_time.
	if !hasSuspendTime && suspendedManually {
		return
	}

	if err := client.ResumeVM(vmid); err != nil {
		log.Error("Failed to resume VM after renew", zap.Int("vmid", vmid), zap.Error(err))
		return
	}

	delete(inst.Data, "suspend_time")
	delete(inst.Data, "suspended_manually")
	go datas.DataPublisher(datas.POST_INST_DATA)(inst.GetUuid(), inst.GetData())
	go s.HandlePublishEvents(context.Background(), &epb.Event{
		Uuid: inst.GetUuid(),
		Key:  "instance_unsuspended",
		Data: map[string]*structpb.Value{},
	})
	log.Info("Auto-resumed VM after renew", zap.Int("vmid", vmid))
}

func (s *DriverServiceServer) SpInvoke(ctx context.Context, req *pb.SpInvokeRequest) (res *spb.InvokeResponse, err error) {
	s.log.Debug("Invoke request received", zap.Any("action", req.Method), zap.Any("data", req.Params))
	sp := req.GetServicesProvider()
	client, err := one.NewClientFromSP(sp, s.log)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Error making client: %v", err)
	}

	method := req.GetMethod()

	action, ok := actions.SpAdminActions[method]
	if !ok || !req.AdminAccess {
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
