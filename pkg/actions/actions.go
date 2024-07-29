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
	"encoding/json"
	"fmt"
	"github.com/slntopp/nocloud-driver-ione/pkg/utils"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/slntopp/nocloud-proto/ansible"

	"github.com/slntopp/nocloud-driver-ione/pkg/datas"

	one "github.com/slntopp/nocloud-driver-ione/pkg/driver"
	billingpb "github.com/slntopp/nocloud-proto/billing"
	ipb "github.com/slntopp/nocloud-proto/instances"
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

type AnsibleAction func(
	context.Context,
	ansible.AnsibleServiceClient,
	string,
	map[string]any,
	*ipb.Instance,
	map[string]*structpb.Value,
) (*ipb.InvokeResponse, error)

var Actions = map[string]ServiceAction{
	"poweroff":        Poweroff,
	"suspend":         Suspend,
	"reboot":          Reboot,
	"resume":          Resume,
	"reinstall":       Reinstall,
	"monitoring":      Monitoring,
	"state":           State,
	"snapcreate":      SnapCreate,
	"snapdelete":      SnapDelete,
	"snaprevert":      SnapRevert,
	"start_vnc":       StartVNC,
	"get_backup_info": GetBackupInfo,
	"freeze":          Freeze,
	"unfreeze":        Unfreeze,
}

var BillingActions = map[string]ServiceAction{
	"manual_renew": nil,
	"cancel_renew": CancelRenew,
	"renew":        ManualRenew,
}

var AnsibleActions = map[string]AnsibleAction{
	"exec": BackupInstance,
}

var AdminActions = map[string]bool{
	"suspend":         true,
	"get_backup_info": true,
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

	inst.Data["suspended_manually"] = structpb.NewBoolValue(true)

	go datas.DataPublisher(datas.POST_INST_DATA)(inst.GetUuid(), inst.GetData())
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

	if _, ok := data["date"]; ok {
		inst.Data["immune_date"] = data["date"]
	}

	inst.Data["suspended_manually"] = structpb.NewBoolValue(false)

	go datas.DataPublisher(datas.POST_INST_DATA)(inst.GetUuid(), inst.GetData())
	// return &ipb.InvokeResponse{Result: true}, nil
	return StatusesClient(client, inst, data, &ipb.InvokeResponse{Result: true})
}

func Freeze(
	client one.IClient,
	inst *ipb.Instance,
	data map[string]*structpb.Value,
) (*ipb.InvokeResponse, error) {
	inst.Data["freeze"] = structpb.NewBoolValue(true)

	go datas.DataPublisher(datas.POST_INST_DATA)(inst.GetUuid(), inst.GetData())
	return StatusesClient(client, inst, data, &ipb.InvokeResponse{Result: true})
}

func Unfreeze(
	client one.IClient,
	inst *ipb.Instance,
	data map[string]*structpb.Value,
) (*ipb.InvokeResponse, error) {
	inst.Data["freeze"] = structpb.NewBoolValue(false)

	go datas.DataPublisher(datas.POST_INST_DATA)(inst.GetUuid(), inst.GetData())
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

	log.Debug("Creds", zap.Any("host", host), zap.Any("user", user), zap.Any("pass", pass))

	kind := "vnc"
	if _, ok := data["kind"]; ok {
		kind = data["kind"].GetStringValue()
	}

	log.Debug("Console", zap.Any("type", kind))

	vmid, err := one.GetVMIDFromData(client, inst)
	if err != nil {
		log.Debug("Error finding VM ID", zap.Error(err))
		return nil, status.Error(codes.InvalidArgument, "VM ID is not present or can't be gathered by name")
	}

	url := fmt.Sprintf("%s/vnc?kind=%s&vmid=%d", host, kind, vmid)

	log.Debug("Url", zap.String("Url", url))

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

	body, err := io.ReadAll(res.Body)
	if err != nil {
		log.Warn("Error reading response body", zap.Error(err))
		return nil, status.Error(codes.Internal, "Cannot read Body")
	}

	log.Debug("Body", zap.String("string", string(body)))

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

// Fetches data of the VM
func Monitoring(
	client one.IClient,
	inst *ipb.Instance,
	params map[string]*structpb.Value,
) (*ipb.InvokeResponse, error) {

	vmid, err := one.GetVMIDFromData(client, inst)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "VM ID is not present or can't be gathered by name")
	}

	m, err := client.Monitoring(vmid)
	if err != nil {
		log.Warn("Error getting monitoring data", zap.Int("vmid", vmid), zap.Error(err))
		return nil, status.Error(codes.Internal, "Cannot get monitoring data")
	}

	mon := map[string][]any{}
	if len(m.Records) != 0 {
		for _, el := range m.Records[0].Elements {
			if el.Key() != "TIMESTAMP" {
				mon[el.Key()] = []any{}
			}
		}
	}

	for _, record := range m.Records {
		pair, err := record.GetPair("TIMESTAMP")
		if err != nil {
			continue
		}
		timestamp := pair.Value

		for param := range mon {
			field, err := record.GetPair(param)
			if err != nil {
				continue
			}

			current := mon[field.Key()]
			mon[field.Key()] = append(current, []any{timestamp, field.Value})
		}
	}

	data := make(map[string]*structpb.Value)
	for param := range mon {
		log.Debug("Writing monitoring fields values", zap.String("field", param))
		value, err := structpb.NewValue(mon[param])
		if err != nil {
			log.Warn("Error while creating list value", zap.Error(err))
			continue
		}

		// Include all options if no params are provided
		if params == nil {
			data[param] = value
			continue
		}

		if _, ok := params[param]; ok {
			data[param] = value
		}
	}

	resp, err := StatusesClient(client, inst, data, &ipb.InvokeResponse{Result: true})
	if err != nil {
		return nil, err
	}

	// Merge meta and data
	for key, value := range data {
		if _, ok := resp.Meta[key]; !ok {
			resp.Meta[key] = value
		}
	}

	return resp, nil
}

func ManualRenew(
	client one.IClient,
	inst *ipb.Instance,
	data map[string]*structpb.Value,
) (*ipb.InvokeResponse, error) {
	instData := inst.GetData()
	instProduct := inst.GetProduct()
	billingPlan := inst.GetBillingPlan()

	kind := billingPlan.GetKind()
	if kind != billingpb.PlanKind_STATIC {
		return &ipb.InvokeResponse{Result: false}, status.Error(codes.Internal, "Not implemented for dynamic plan")
	}

	lastMonitoring, ok := instData["last_monitoring"]
	if !ok {
		return &ipb.InvokeResponse{Result: false}, status.Error(codes.Internal, "No last_monitoring data")
	}
	lastMonitoringValue := int64(lastMonitoring.GetNumberValue())

	period := billingPlan.GetProducts()[instProduct].GetPeriod()

	lastMonitoringValue = utils.AlignPaymentDate(lastMonitoringValue, lastMonitoringValue+period, period)
	instData["last_monitoring"] = structpb.NewNumberValue(float64(lastMonitoringValue))

	for _, resource := range billingPlan.GetResources() {
		period := resource.GetPeriod()
		key := fmt.Sprintf("%s_last_monitoring", resource.Key)
		lmValue, ok := instData[key]
		if _, has := inst.GetResources()[resource.Key]; !ok || period == 0 || !has {
			continue
		}
		lm := int64(lmValue.GetNumberValue())
		lm = utils.AlignPaymentDate(lm, lm+period, period)
		instData[key] = structpb.NewNumberValue(float64(lm))
	}

	for _, addonId := range inst.Addons {
		key := fmt.Sprintf("addon_%s_last_monitoring", addonId)
		lmValue, ok := instData[key]
		if ok {
			lm := int64(lmValue.GetNumberValue())
			lm = utils.AlignPaymentDate(lm, lm+period, period)
			instData[key] = structpb.NewNumberValue(float64(lm))
		}
	}

	datas.DataPublisher(datas.POST_INST_DATA)(inst.GetUuid(), instData)
	return &ipb.InvokeResponse{Result: true}, nil
}

func CancelRenew(
	client one.IClient,
	inst *ipb.Instance,
	data map[string]*structpb.Value,
) (*ipb.InvokeResponse, error) {
	instData := inst.GetData()
	instProduct := inst.GetProduct()
	billingPlan := inst.GetBillingPlan()

	kind := billingPlan.GetKind()
	if kind != billingpb.PlanKind_STATIC {
		return &ipb.InvokeResponse{Result: false}, status.Error(codes.Internal, "Not implemented for dynamic plan")
	}

	lastMonitoring, ok := instData["last_monitoring"]
	if !ok {
		return &ipb.InvokeResponse{Result: false}, status.Error(codes.Internal, "No last_monitoring data")
	}
	lastMonitoringValue := int64(lastMonitoring.GetNumberValue())

	period := billingPlan.GetProducts()[instProduct].GetPeriod()

	lastMonitoringValue = utils.AlignPaymentDate(lastMonitoringValue, lastMonitoringValue-period, period)
	lastMonitoringValue = int64(math.Max(float64(lastMonitoringValue), float64(inst.Created)))
	instData["last_monitoring"] = structpb.NewNumberValue(float64(lastMonitoringValue))

	for _, resource := range billingPlan.GetResources() {
		period := resource.GetPeriod()
		key := fmt.Sprintf("%s_last_monitoring", resource.Key)
		lmValue, ok := instData[key]
		if _, has := inst.GetResources()[resource.Key]; !ok || period == 0 || !has {
			continue
		}
		lm := int64(lmValue.GetNumberValue())
		lm = utils.AlignPaymentDate(lm, lm-period, period)
		lm = int64(math.Max(float64(lm), float64(inst.Created)))
		instData[key] = structpb.NewNumberValue(float64(lm))
	}

	for _, addonId := range inst.Addons {
		key := fmt.Sprintf("addon_%s_last_monitoring", addonId)
		lmValue, ok := instData[key]
		if ok {
			lm := int64(lmValue.GetNumberValue())
			lm = utils.AlignPaymentDate(lm, lm-period, period)
			instData[key] = structpb.NewNumberValue(float64(lm))
		}
	}

	datas.DataPublisher(datas.POST_INST_DATA)(inst.GetUuid(), instData)
	return &ipb.InvokeResponse{Result: true}, nil
}

func GetBackupInfo(
	client one.IClient,
	inst *ipb.Instance,
	data map[string]*structpb.Value,
) (*ipb.InvokeResponse, error) {
	instData := inst.GetData()

	if instData == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Instance data is nil")
	}

	vmid, ok := instData["vmid"]
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "No vm id")
	}
	vmidVal := int(vmid.GetNumberValue())

	vm, err := client.GetVM(vmidVal)
	if err != nil {
		return nil, err
	}

	str, err := vm.MonitoringInfos.GetStr("DISK_0_ACTUAL_PATH")
	if err != nil {
		return nil, err
	}

	split := strings.Split(str, " ")
	if len(split) != 2 {
		return nil, status.Errorf(codes.InvalidArgument, "Failed to get info")
	}

	datastore := split[0][1 : len(split[0])-1]
	split = strings.Split(split[1], "/")

	if len(split) == 1 {
		return nil, status.Errorf(codes.InvalidArgument, "Failed to get dir")
	}
	dir := split[0]

	return &ipb.InvokeResponse{
		Result: true,
		Meta: map[string]*structpb.Value{
			"datastore": structpb.NewStringValue(datastore),
			"dir":       structpb.NewStringValue(dir),
		},
	}, nil
}

func BackupInstance(
	ctx context.Context,
	client ansible.AnsibleServiceClient,
	playbookUuid string,
	hop map[string]any,
	inst *ipb.Instance,
	data map[string]*structpb.Value,
) (*ipb.InvokeResponse, error) {
	host := data["host"].GetStringValue()
	vm_dir := data["vm_dir"].GetStringValue()
	snapshot_date := data["snapshot_date"].GetStringValue()

	ansibleInstance := &ansible.Instance{
		Host: host,
	}

	hopHost, ok := hop["host"].(string)
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "No hop host")
	}
	hopPort, ok := hop["port"].(string)
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "No hop port")
	}
	hopSsh, ok := hop["ssh"].(string)
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "No hop ssh")
	}

	info := hop["info"]
	if info != nil {
		infoVal, ok := info.(map[string]any)
		if !ok {
			return nil, status.Errorf(codes.InvalidArgument, "Failed to parse info")
		}

		hostInfo, ok := infoVal[host].(map[string]any)
		if !ok {
			return nil, status.Errorf(codes.InvalidArgument, "Failed to host info")
		}
		ansibleHost, ok := hostInfo["ansible_host"].(string)
		if !ok {
			return nil, status.Errorf(codes.InvalidArgument, "Failed to get host")
		}
		python, ok := hostInfo["python"].(string)
		if !ok {
			return nil, status.Errorf(codes.InvalidArgument, "Failed to get python")
		}
		ansibleInstance.Python = &python
		ansibleInstance.AnsibleHost = &ansibleHost
	}

	log.Debug("inst", zap.Any("inst", ansibleInstance))

	create, err := client.Create(ctx, &ansible.CreateRunRequest{
		Run: &ansible.Run{
			SshKey: &hopSsh,
			Instances: []*ansible.Instance{
				ansibleInstance,
			},
			PlaybookUuid: playbookUuid,
			Vars: map[string]string{
				"vm_dir":        vm_dir,
				"snapshot_date": snapshot_date,
			},
			Hop: &ansible.Instance{
				Host: hopHost,
				Port: &hopPort,
			},
		},
	})

	if err != nil {
		return nil, err
	}

	_, err = client.Exec(ctx, &ansible.ExecRunRequest{
		Uuid: create.GetUuid(),
	})

	if err != nil {
		return nil, err
	}

	inst.Data["running_playbook"] = structpb.NewStringValue(create.GetUuid())
	go datas.DataPublisher(datas.POST_INST_DATA)(inst.GetUuid(), inst.GetData())

	return &ipb.InvokeResponse{
		Result: true,
	}, nil
}
