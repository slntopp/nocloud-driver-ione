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
	"errors"
	"time"

	one "github.com/slntopp/nocloud-driver-ione/pkg/driver"
	instpb "github.com/slntopp/nocloud/pkg/instances/proto"
	srvpb "github.com/slntopp/nocloud/pkg/services/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

type ServiceAction func(
	*one.ONeClient,
	*instpb.InstancesGroup,
	*instpb.Instance,
	map[string]*structpb.Value,
) (*srvpb.PerformActionResponse, error)

var Actions = map[string]ServiceAction{
	"poweroff": Poweroff,
	"suspend":  Suspend,
	"reboot":   Reboot,
	"resume":   Resume,
	"state":    State,
}

func GetVMIDFromData(client *one.ONeClient, inst *instpb.Instance) (vmid int, err error) {
	data := inst.GetData()
	if data == nil {
		return -1, errors.New("data is empty")
	}

	vmidVar, ok := data[one.DATA_VM_ID]
	if !ok {
		goto try_by_name
	}
	vmid = int(vmidVar.GetNumberValue())
	return vmid, nil

try_by_name:
	name, ok := data[one.DATA_VM_NAME]
	if !ok {
		return -1, errors.New("VM ID and VM Name aren't set in data")
	}
	vmid, err = client.GetVMByName(name.GetStringValue())
	if err != nil {
		return -1, err
	}
	return vmid, nil
}

// Powers off a running VM
func Poweroff(
	client *one.ONeClient,
	_ *instpb.InstancesGroup,
	inst *instpb.Instance,
	data map[string]*structpb.Value,
) (*srvpb.PerformActionResponse, error) {

	vmid, err := GetVMIDFromData(client, inst)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "VM ID is not present or can't be gathered by name")
	}

	hard := false
	if v, ok := data["hard"]; ok {
		hard = v.GetBoolValue()
	}

	err = client.PoweroffVM(vmid, hard)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Can't Power Off VM, error: %v", err)
	}

	// return &srvpb.PerformActionResponse{Result: true}, nil
	return StatusesClient(client, inst, data, &srvpb.PerformActionResponse{Result: true})
}

// Saves a running VM
func Suspend(
	client *one.ONeClient,
	_ *instpb.InstancesGroup,
	inst *instpb.Instance,
	data map[string]*structpb.Value,
) (*srvpb.PerformActionResponse, error) {

	vmid, err := GetVMIDFromData(client, inst)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "VM ID is not present or can't be gathered by name")
	}

	err = client.SuspendVM(vmid)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Can't Suspend VM, error: %v", err)
	}

	// return &srvpb.PerformActionResponse{Result: true}, nil
	return StatusesClient(client, inst, data, &srvpb.PerformActionResponse{Result: true})
}

// Reboots an already deployed VM
func Reboot(
	client *one.ONeClient,
	_ *instpb.InstancesGroup,
	inst *instpb.Instance,
	data map[string]*structpb.Value,
) (*srvpb.PerformActionResponse, error) {

	vmid, err := GetVMIDFromData(client, inst)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "VM ID is not present or can't be gathered by name")
	}

	hard := false
	if v, ok := data["hard"]; ok {
		hard = v.GetBoolValue()
	}

	err = client.RebootVM(vmid, hard)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Can't Reboot VM, error: %v", err)
	}

	// return &srvpb.PerformActionResponse{Result: true}, nil
	return StatusesClient(client, inst, data, &srvpb.PerformActionResponse{Result: true})
}

// Resumes the execution of a saved VM.
func Resume(
	client *one.ONeClient,
	_ *instpb.InstancesGroup,
	inst *instpb.Instance,
	data map[string]*structpb.Value,
) (*srvpb.PerformActionResponse, error) {

	vmid, err := GetVMIDFromData(client, inst)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "VM ID is not present or can't be gathered by name")
	}

	err = client.ResumeVM(vmid)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Can't Resume VM, error: %v", err)
	}

	// return &srvpb.PerformActionResponse{Result: true}, nil
	return StatusesClient(client, inst, data, &srvpb.PerformActionResponse{Result: true})
}

// Returns the VM state of the VirtualMachine
func State(
	client *one.ONeClient,
	_ *instpb.InstancesGroup,
	inst *instpb.Instance,
	data map[string]*structpb.Value,
) (*srvpb.PerformActionResponse, error) {

	vmid, err := GetVMIDFromData(client, inst)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "VM ID is not present or can't be gathered by name")
	}

	state, state_str, lcm_state, lcm_state_str, err := client.StateVM(vmid)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Can't get State VM, error: %v", err)
	}

	m, err := structpb.NewValue(map[string]interface{}{
		"uuid":          inst.Uuid,
		"state":         state,
		"state_str":     state_str,
		"lcm_state":     lcm_state,
		"lcm_state_str": lcm_state_str,
		"updated":       inst.State.Meta["updated"].GetListValue().AsSlice(),
		"ts":            time.Now().Unix(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Can't pass State VM, error: %v", err)
	}

	meta := m.GetStructValue().Fields
	return &srvpb.PerformActionResponse{Result: true, Meta: meta}, nil
}
