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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	one "github.com/slntopp/nocloud-driver-ione/pkg/driver"
	ipb "github.com/slntopp/nocloud/pkg/instances/proto"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

type ServiceAction func(
	one.IClient,
	*ipb.Instance,
	map[string]*structpb.Value,
) (*ipb.InvokeResponse, error)

var Actions = map[string]ServiceAction{
	"poweroff":   Poweroff,
	"suspend":    Suspend,
	"reboot":     Reboot,
	"resume":     Resume,
	"reinstall":  Reinstall,
	"state":      State,
	"snapcreate": SnapCreate,
	"snapdelete": SnapDelete,
	"snaprevert": SnapRevert,
	"start_vnc":  StartVNC,
}

var AdminActions = map[string]bool{
	"suspend": true,
	"resume":  true,
}

// Creates new snapshot of vm
func SnapCreate(
	client one.IClient,
	inst *ipb.Instance,
	data map[string]*structpb.Value,
) (*ipb.InvokeResponse, error) {

	vmid, err := one.GetVMIDFromData(client, inst)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "VM ID is not present or can't be gathered by name")
	}

	snapName := ""
	if v, ok := data["snap_name"]; ok {
		snapName = v.GetStringValue()
	}

	err = client.SnapCreate(snapName, vmid)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Can't Create Snapshot, error: %v", err)
	}

	return StatusesClient(client, inst, data, &ipb.InvokeResponse{Result: true})
}

// Deletes Snapshot by ID
func SnapDelete(
	client one.IClient,
	inst *ipb.Instance,
	data map[string]*structpb.Value,
) (*ipb.InvokeResponse, error) {

	vmid, err := one.GetVMIDFromData(client, inst)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "VM ID is not present or can't be gathered by name")
	}

	snapID := -1
	if v, ok := data["snap_id"]; ok {
		snapID = int(v.GetNumberValue())
	} else {
		return nil, status.Errorf(codes.InvalidArgument, "No Snapshot id, error: %v", err)
	}

	err = client.SnapDelete(snapID, vmid)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Can't Delete Snapshot, error: %v", err)
	}

	return StatusesClient(client, inst, data, &ipb.InvokeResponse{Result: true})
}

// Reverts Snapshot by ID
func SnapRevert(
	client one.IClient,
	inst *ipb.Instance,
	data map[string]*structpb.Value,
) (*ipb.InvokeResponse, error) {

	vmid, err := one.GetVMIDFromData(client, inst)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "VM ID is not present or can't be gathered by name")
	}

	snapID := -1
	if v, ok := data["snap_id"]; ok {
		snapID = int(v.GetNumberValue())
	} else {
		return nil, status.Errorf(codes.InvalidArgument, "No Snapshot id, error: %v", err)
	}

	err = client.SnapRevert(snapID, vmid)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Can't Create Snapshot, error: %v", err)
	}

	return StatusesClient(client, inst, data, &ipb.InvokeResponse{Result: true})
}

// Remove VM and create with same specs and user
func Reinstall(
	client one.IClient,
	inst *ipb.Instance,
	data map[string]*structpb.Value,
) (*ipb.InvokeResponse, error) {

	vmid, err := one.GetVMIDFromData(client, inst)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "VM ID is not present or can't be gathered by name")
	}

	err = client.Reinstall(vmid)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Can't Reinstall VM, error: %v", err)
	}

	return StatusesClient(client, inst, data, &ipb.InvokeResponse{Result: true})
}

// Powers off a running VM
func Poweroff(
	client one.IClient,
	inst *ipb.Instance,
	data map[string]*structpb.Value,
) (*ipb.InvokeResponse, error) {

	vmid, err := one.GetVMIDFromData(client, inst)
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

	// return &ipb.InvokeResponse{Result: true}, nil
	return StatusesClient(client, inst, data, &ipb.InvokeResponse{Result: true})
}

// Saves a running VM
func Suspend(
	client one.IClient,
	inst *ipb.Instance,
	data map[string]*structpb.Value,
) (*ipb.InvokeResponse, error) {

	vmid, err := one.GetVMIDFromData(client, inst)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "VM ID is not present or can't be gathered by name")
	}

	err = client.SuspendVM(vmid)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Can't Suspend VM, error: %v", err)
	}

	// return &ipb.InvokeResponse{Result: true}, nil
	return StatusesClient(client, inst, data, &ipb.InvokeResponse{Result: true})
}

// Reboots an already deployed VM
func Reboot(
	client one.IClient,
	inst *ipb.Instance,
	data map[string]*structpb.Value,
) (*ipb.InvokeResponse, error) {

	vmid, err := one.GetVMIDFromData(client, inst)
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

	// return &ipb.InvokeResponse{Result: true}, nil
	return StatusesClient(client, inst, data, &ipb.InvokeResponse{Result: true})
}

// Resumes the execution of a saved VM.
func Resume(
	client one.IClient,
	inst *ipb.Instance,
	data map[string]*structpb.Value,
) (*ipb.InvokeResponse, error) {

	vmid, err := one.GetVMIDFromData(client, inst)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "VM ID is not present or can't be gathered by name")
	}

	err = client.ResumeVM(vmid)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Can't Resume VM, error: %v", err)
	}

	// return &ipb.InvokeResponse{Result: true}, nil
	return StatusesClient(client, inst, data, &ipb.InvokeResponse{Result: true})
}

// Returns the VM state of the VirtualMachine
func State(
	client one.IClient,
	inst *ipb.Instance,
	data map[string]*structpb.Value,
) (*ipb.InvokeResponse, error) {

	vmid, err := one.GetVMIDFromData(client, inst)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "VM ID is not present or can't be gathered by name")
	}

	state, state_str, lcm_state, lcm_state_str, err := client.StateVM(vmid)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Can't get State VM, error: %v", err)
	}

	m := map[string]interface{}{
		"uuid":          inst.Uuid,
		"state":         state,
		"state_str":     state_str,
		"lcm_state":     lcm_state,
		"lcm_state_str": lcm_state_str,
		"ts":            time.Now().Unix(),
	}

	if inst.State == nil || inst.State.Meta == nil {
		goto make_value
	}

	if upd, ok := inst.State.Meta["updated"]; ok {
		m["updated"] = upd.GetListValue().AsSlice()
	}

	m["networking"], err = client.NetworkingVM(vmid)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Can't get Networking VM, error: %v", err)
	}

	m["snapshots"], err = client.GetInstSnapshots(inst)
	if err != nil {
		return nil, err
	}

make_value:
	meta, err := structpb.NewValue(m)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Can't pass State VM, error: %v", err)
	}

	return &ipb.InvokeResponse{Result: true, Meta: meta.GetStructValue().Fields}, nil
}

func StartVNC(
	client one.IClient,
	inst *ipb.Instance,
	data map[string]*structpb.Value,
) (*ipb.InvokeResponse, error) {

	log := client.Logger("StartVNC")

	secrets := client.GetSecrets()
	host := secrets["host"].GetStringValue()
	user := secrets["user"].GetStringValue()
	pass := secrets["pass"].GetStringValue()

	kind := "vnc"
	if _, ok := data["kind"]; ok {
		kind = data["kind"].GetStringValue()
	}

	vmid, err := one.GetVMIDFromData(client, inst)
	if err != nil {
		log.Debug("Error finding VM ID", zap.Error(err))
		return nil, status.Error(codes.InvalidArgument, "VM ID is not present or can't be gathered by name")
	}

	url := fmt.Sprintf("%s/vnc?kind=%s&vmid=%d", host, kind, vmid)

	hc := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Warn("Cannot build request", zap.Error(err))
		return nil, status.Error(codes.Internal, "Cannot build request")
	}
	req.SetBasicAuth(user, pass)

	res, err := hc.Do(req)
	if err != nil {
		log.Warn("Error performing request", zap.Error(err))
		return nil, status.Error(codes.Internal, "Error performing Request")
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Warn("Error reading response body", zap.Error(err))
		return nil, status.Error(codes.Internal, "Cannot read Body")
	}

	var token_data map[string]interface{}
	err = json.Unmarshal(body, &token_data)
	if err != nil {
		log.Warn("Error Unmarshaling response", zap.ByteString("body", body), zap.Error(err))
		return nil, status.Error(codes.Internal, "Cannot Unmarshal Body")
	}

	res_struct, _ := structpb.NewStruct(token_data)
	return &ipb.InvokeResponse{
		Result: true, Meta: res_struct.GetFields(),
	}, nil
}
