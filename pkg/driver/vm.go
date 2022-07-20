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
package one

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/vm"
	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/vm/keys"
	"github.com/slntopp/nocloud-driver-ione/pkg/datas"
	"github.com/slntopp/nocloud/pkg/hasher"
	pb "github.com/slntopp/nocloud/pkg/instances/proto"
	"github.com/slntopp/nocloud/pkg/nocloud/auth"
	stpb "github.com/slntopp/nocloud/pkg/states/proto"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

func (c *ONeClient) GetInstSnapshots(inst *pb.Instance) (map[string]interface{}, error) {
	vm, err := c.FindVMByInstance(inst)
	if err != nil {
		return nil, err
	}

	snaps := make(map[string]interface{}, 0)

	snapsV := vm.Template.GetVectors("SNAPSHOT")
	for _, snapV := range snapsV {
		id, err := snapV.GetStr("SNAPSHOT_ID")
		if err != nil {
			return nil, err
		}
		name, err := snapV.GetStr("NAME")
		if err != nil {
			return nil, err
		}
		time, err := snapV.GetInt("TIME")
		if err != nil {
			return nil, err
		}

		snap := make(map[string]interface{}, 0)
		snap["name"] = name
		snap["ts"] = time

		snaps[id] = snap
	}

	return snaps, nil
}

func (c *ONeClient) SnapCreate(name string, vmid int) error {
	vmc := c.ctrl.VM(vmid)
	return vmc.SnapshotCreate(name)
}

func (c *ONeClient) SnapDelete(snapId, vmid int) error {
	vmc := c.ctrl.VM(vmid)
	return vmc.SnapshotDelete(snapId)
}

func (c *ONeClient) SnapRevert(snapId, vmid int) error {
	vmc := c.ctrl.VM(vmid)
	return vmc.SnapshotRevert(snapId)
}

func (c *ONeClient) GetVMByName(name string) (id int, err error) {
	vmsc := c.ctrl.VMs()
	return vmsc.ByName(name)
}

func (c *ONeClient) GetVM(vmid int) (*vm.VM, error) {
	return c.ctrl.VM(vmid).Info(true)
}

func (c *ONeClient) TerminateVM(id int, hard bool) error {
	vmc := c.ctrl.VM(id)
	if hard {
		return vmc.TerminateHard()
	}
	return vmc.Terminate()
}

func (c *ONeClient) PoweroffVM(id int, hard bool) error {
	vmc := c.ctrl.VM(id)
	if hard {
		return vmc.PoweroffHard()
	}
	return vmc.Poweroff()
}

func (c *ONeClient) SuspendVM(id int) error {
	vmc := c.ctrl.VM(id)
	return vmc.Suspend()
}

func (c *ONeClient) RebootVM(id int, hard bool) error {
	vmc := c.ctrl.VM(id)
	if hard {
		return vmc.RebootHard()
	}
	return vmc.Reboot()
}

func (c *ONeClient) ResumeVM(id int) error {
	vmc := c.ctrl.VM(id)
	return vmc.Resume()
}

func (c *ONeClient) StateVM(id int) (state int, state_str string, lcm_state int, lcm_state_str string, err error) {
	vmc := c.ctrl.VM(id)

	vm, err := vmc.Info(false)
	if err != nil {
		return 0, "nil", 0, "nil", err
	}

	st, lcm_st, err := vm.State()
	if err != nil {
		return 0, "nil", 0, "nil", err
	}

	return int(st), st.String(), int(lcm_st), lcm_st.String(), nil
}

func (c *ONeClient) NetworkingVM(id int) (map[string]interface{}, error) {
	vmc := c.ctrl.VM(id)

	vm, err := vmc.Info(false)
	if err != nil {
		return nil, err
	}

	networking := make(map[string]interface{})

	publicIps := make([]interface{}, 0)
	privateIps := make([]interface{}, 0)

	nics := vm.Template.GetNICs()
	for _, nic := range nics {
		ip, err := nic.GetStr("IP")
		if err != nil {
			c.log.Error("Couldn't get IP", zap.Any("nic", nic))
			continue
		}

		vnet, err := nic.GetStr("NETWORK")
		if err != nil {
			c.log.Error("Couldn't get Network", zap.Any("nic", nic))
			continue
		}

		switch vnet {
		case fmt.Sprintf(USER_PUBLIC_VNET_NAME_PATTERN, vm.UID):
			publicIps = append(publicIps, ip)
		case fmt.Sprintf(USER_PRIVATE_VNET_NAME_PATTERN, vm.UID):
			privateIps = append(privateIps, ip)
		default:
			{
				c.log.Error("Invalid VNet Name", zap.Any("vnet", vnet))
				continue
			}
		}
	}

	networking["public"] = publicIps
	networking["private"] = privateIps

	return networking, nil
}

func (c *ONeClient) VMToInstance(id int) (*pb.Instance, error) {
	vmc := c.ctrl.VM(id)
	vm, err := vmc.Info(true)
	if err != nil {
		return nil, err
	}
	inst := pb.Instance{
		Uuid:      "",
		Title:     "",
		Status:    0,
		Config:    make(map[string]*structpb.Value),
		Resources: make(map[string]*structpb.Value),
		Data:      make(map[string]*structpb.Value),
		Hash:      "",
	}

	tmpl := vm.Template
	utmpl := vm.UserTemplate
	{
		tid, err := tmpl.GetFloat("TEMPLATE_ID")
		if err != nil {
			return nil, err
		}
		inst.Config["template_id"] = structpb.NewNumberValue(tid)
	}
	{
		pwd, err := utmpl.GetStr("PASSWORD")
		if err == nil {
			inst.Config["password"] = structpb.NewStringValue(pwd)
		}
	}
	{
		ctx, err := tmpl.GetVector("CONTEXT")
		if err == nil {
			goto cpu
		}
		ssh, err := ctx.GetStr(string(keys.SSHPubKey))
		if err != nil {
			inst.Config["ssh_public_key"] = structpb.NewStringValue(ssh)
		}
	}
cpu:
	{
		cpu, err := tmpl.GetCPU()
		if err != nil {
			return nil, err
		}
		inst.Resources["cpu"] = structpb.NewNumberValue(cpu)
	}
	{
		ram, err := tmpl.GetMemory()
		if err != nil {
			return nil, err
		}
		inst.Resources["ram"] = structpb.NewNumberValue(float64(ram))
	}
	{
		vmid, err := tmpl.GetFloat("VMID")
		if err != nil {
			return nil, err
		}
		inst.Data["vmid"] = structpb.NewNumberValue(float64(vmid))
	}
	{
		inst.Data["vm_name"] = structpb.NewStringValue(vm.Name)
	}
	{
		diskInfo, err := tmpl.GetVector("DISK")
		if err != nil {
			return nil, err
		}
		// if instance does not exist its template doesn't have DRIVE_TYPE & SIZE
		// that's why we don't return error
		driveType, err := diskInfo.GetStr("DRIVE_TYPE")
		if err != nil {
			c.log.Info("VMToInstance", zap.Any("Error", err))
			driveType = "NOT FOUND"
		}
		driveSize, err := diskInfo.GetFloat("SIZE")
		if err != nil {
			c.log.Info("VMToInstance", zap.Any("Error", err))
			driveSize = -1
		}
		inst.Resources["drive_type"] = structpb.NewStringValue(driveType)
		inst.Resources["drive_size"] = structpb.NewNumberValue(float64(driveSize))
	}
	{
		ips_public, ips_private := 0, 0
		NICs := tmpl.GetNICs()
		for _, nic := range NICs {
			vn_name, err := nic.GetStr("NETWORK")
			if err != nil {
				return nil, err
			}
			if strings.HasSuffix(vn_name, "pub-vnet") {
				ips_public++
				continue
			}
			if strings.HasSuffix(vn_name, "private-vnet") {
				ips_private++
				continue
			}
		}
		inst.Resources["ips_public"] = structpb.NewNumberValue(float64(ips_public))
		inst.Resources["ips_private"] = structpb.NewNumberValue(float64(ips_private))
	}

	return &inst, nil
}

// returns instances of all VMs belonged to User
func (c *ONeClient) GetUserVMsInstancesGroup(userId int) (*pb.InstancesGroup, error) {
	vmsc := c.ctrl.VMs()
	vms_pool, err := vmsc.Info(userId)
	if err != nil {
		return nil, err
	}
	ig := &pb.InstancesGroup{
		Uuid:      "",
		Type:      "",
		Config:    make(map[string]*structpb.Value),
		Instances: make([]*pb.Instance, 0, len(vms_pool.VMs)),
		Resources: make(map[string]*structpb.Value),
		Hash:      "",
	}

	for _, vm := range vms_pool.VMs {
		inst, err := c.VMToInstance(vm.ID)
		if err != nil {
			return nil, err
		}
		ig.Instances = append(ig.Instances, inst)
	}
	return ig, nil
}

// O(n) search of Instance in InstancesGroup by VMID
// I think the best way is to use map[vmid]inst, but it can be redundant, maybe remaked in future
/*func findInstanceByVMID(c *ONeClient, vmid int, ig *pb.InstancesGroup) (*pb.Instance, error) {
	for _, inst := range ig.GetInstances() {
		instVMID, err := GetVMIDFromData(c, inst)
		if err != nil {
			c.log.Error("Error Getting VMID from Data", zap.Error(err))
			continue
		}
		if vmid == instVMID {
			return inst, nil
		}
	}
	return nil, errors.New("instance not found")
}*/

/*func (c *ONeClient) FindInstanceByVM(vm *vm.VM, ig *pb.InstancesGroup) (*pb.Instance, error) {

	for _, inst := range ig.GetInstances() {
		instVMid, err := GetVMIDFromData(c, inst)
		if err != nil || instVMid != vm.ID {
			continue
		}

		return inst, nil
	}

	return nil, status.Errorf(codes.InvalidArgument, "Error Instance Not Found, VM Name: %s, VM ID: %d", vm.Name, vm.ID)
}*/

func (c *ONeClient) FindVMByInstance(inst *pb.Instance) (*vm.VM, error) {
	var st vm.State
	var VM *vm.VM

	vmid, err := GetVMIDFromData(c, inst)
	if err != nil {
		goto byName
	}

	VM, err = c.ctrl.VM(vmid).Info(true)
	if err != nil {
		goto byName
	}

	st, _, err = VM.State()
	if err != nil {
		goto byName
	}

	if st != vm.Done {
		return VM, nil
	}

byName:
	vmid, err = c.GetVMByName(inst.GetUuid())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Error searching VM %v", err)
	}

	return c.ctrl.VM(vmid).Info(true)
}

type CheckInstancesGroupResponse struct {
	ToBeCreated []*pb.Instance
	ToBeDeleted []*pb.Instance
	ToBeUpdated []*pb.Instance
	Valid       []*pb.Instance
}

func (c *ONeClient) CheckInstancesGroup(IG *pb.InstancesGroup) (*CheckInstancesGroupResponse, error) {
	resp := CheckInstancesGroupResponse{
		ToBeCreated: make([]*pb.Instance, 0),
		ToBeDeleted: make([]*pb.Instance, 0),
		ToBeUpdated: make([]*pb.Instance, 0),
		Valid:       make([]*pb.Instance, 0),
	}

	var userId int
	if id, ok := IG.Data["userid"]; ok {
		userId = int(id.GetNumberValue())
	} else {
		return nil, status.Errorf(codes.InvalidArgument, "Error User ID Not Found, IG, %v", IG)
	}

	vmsc := c.ctrl.VMs()
	vms_pool, err := vmsc.Info(userId)
	if err != nil {
		c.log.Error("Error Getting VMs Info by UserId", zap.Any("userId", userId), zap.Error(err))
		return nil, err
	}
	instMapForFastSearch := make(map[int]*pb.Instance, len(IG.GetInstances()))
	for _, inst := range IG.GetInstances() {
		vmid, err := GetVMIDFromData(c, inst)
		if err != nil {
			continue
		}
		instMapForFastSearch[vmid] = inst
	}

	for _, vm := range vms_pool.VMs {
		if _, ok := instMapForFastSearch[vm.ID]; !ok {
			vmInst, err := c.VMToInstance(vm.ID)
			if err != nil {
				c.log.Error("Error Converting VM to Instance", zap.Error(err))
				continue
			}

			resp.ToBeDeleted = append(resp.ToBeDeleted, vmInst)
		}
	}

	for _, inst := range IG.GetInstances() {
		vm, err := c.FindVMByInstance(inst)
		if err != nil {
			resp.ToBeCreated = append(resp.ToBeCreated, inst)
			continue
		}

		res, err := c.VMToInstance(vm.ID)
		if err != nil {
			c.log.Error("Error Converting VM to Instance", zap.Error(err))
			continue
		}
		res.Uuid = inst.GetUuid()
		res.Title = inst.GetTitle()
		res.Status = inst.GetStatus()

		err = hasher.SetHash(res.ProtoReflect())
		if err != nil {
			c.log.Error("Error Setting Instance Hash", zap.Error(err))
			continue
		}

		if res.Hash != inst.Hash {
			resp.ToBeUpdated = append(resp.ToBeUpdated, inst)
		} else {
			resp.Valid = append(resp.Valid, inst)
		}
	}

	return &resp, nil
}

func (c *ONeClient) CheckInstancesGroupResponseProcess(resp *CheckInstancesGroupResponse, data map[string]*structpb.Value, group int) {
	userid := int(data["userid"].GetNumberValue())
	for _, inst := range resp.ToBeCreated {
		token, err := auth.MakeTokenInstance(inst.GetUuid())
		if err != nil {
			c.log.Error("Error generating VM token", zap.String("instance", inst.GetUuid()), zap.Error(err))
			continue
		}
		vmid, err := c.InstantiateTemplateHelper(inst, data, token)
		if err != nil {
			c.log.Error("Error deploying VM", zap.String("instance", inst.GetUuid()), zap.Error(err))
			continue
		}
		c.Chown("vm", vmid, userid, group)

		go datas.Pub(&pb.ObjectData{
			Uuid: inst.Uuid,
			Data: inst.Data,
		})
	}

	for _, inst := range resp.ToBeDeleted {
		vmid, err := GetVMIDFromData(c, inst)
		if err != nil {
			c.log.Error("Error Getting VMID from Data", zap.Error(err))
			continue
		}
		c.TerminateVM(vmid, true)
	}

	for _, inst := range resp.ToBeUpdated {
		vmid, err := GetVMIDFromData(c, inst)
		if err != nil {
			c.log.Error("Error Getting VMID from Data", zap.Error(err))
			continue
		}
		vmc := c.ctrl.VM(vmid)
		VM, err := vmc.Info(true)
		if err != nil {
			c.log.Error("Error Getting VM Info", zap.Error(err))
			continue
		}

		vmInst, err := c.VMToInstance(vmid)
		if err != nil {
			c.log.Error("Error Converting VM to Instance", zap.Error(err))
			continue
		}
		updated := make([]interface{}, 0)

		// Resizing using template
		_, lcmState, err := VM.State()
		if err != nil {
			c.log.Error("Error Recieving VM State", zap.Error(err))
			continue
		}
		tmpl := vm.NewTemplate()
		if vmInst.Resources["cpu"].GetNumberValue() != inst.Resources["cpu"].GetNumberValue() {
			tmpl.CPU(inst.Resources["cpu"].GetNumberValue())
			updated = append(updated, "cpu")
		}

		if vmInst.Resources["ram"].GetNumberValue() != inst.Resources["ram"].GetNumberValue() {
			tmpl.Memory(int(inst.Resources["ram"].GetNumberValue()))
			updated = append(updated, "ram")
		}

		if len(updated) > 0 {
			if lcmState == vm.Running {
				err = vmc.Poweroff()
				if err != nil {
					c.log.Error("Error VM Poweroff()", zap.Error(err))
					continue
				}
				for {
					VM, err = vmc.Info(true)
					if err != nil {
						c.log.Error("Error Getting VM Info", zap.Error(err))
						continue
					}
					state, _, err := VM.State()
					if err != nil {
						c.log.Error("Error Recieving VM State", zap.Error(err))
						continue
					}
					if state == vm.Poweroff {
						break
					} else {
						t := time.NewTimer(1 * time.Second)
						<-t.C
					}
				}
			}
			err = vmc.Resize(tmpl.String(), true)
			if err != nil {
				c.log.Error("Error Resizing using template", zap.Error(err))
				updated = make([]interface{}, 0)
			}
		}

		// Resizing without template
		if vmInst.Resources["drive_size"].GetNumberValue() != inst.Resources["drive_size"].GetNumberValue() {
			err = vmc.Disk(0).Resize(strconv.Itoa(int(inst.Resources["drive_size"].GetNumberValue())))
			if err != nil {
				c.log.Error("Error Disk Resizing", zap.Error(err))
			} else {
				updated = append(updated, "drive_size")
			}
		}

		updlist, err := structpb.NewValue(updated)
		if err != nil {
			c.log.Error("Error Converting Updated To Structpb.List", zap.Error(err))
			continue
		}
		inst.State.Meta["updated"] = updlist
	}

	for _, inst := range resp.Valid {
		if inst.GetState() == nil || inst.GetState().Meta == nil {
			continue
		}
		_, updated := inst.State.Meta["updated"]
		if updated {
			delete(inst.State.Meta, "updated")
		}
	}
}

func GetVMIDFromData(client *ONeClient, inst *pb.Instance) (vmid int, err error) {
	data := inst.GetData()
	if data == nil {
		return -1, errors.New("data is empty")
	}

	vmidVar, ok := data[DATA_VM_ID]
	if !ok {
		goto try_by_name
	}
	vmid = int(vmidVar.GetNumberValue())
	return vmid, nil

try_by_name:
	name, ok := data[DATA_VM_NAME]
	if !ok {
		return -1, errors.New("VM ID and VM Name aren't set in data")
	}
	vmid, err = client.GetVMByName(name.GetStringValue())
	if err != nil {
		return -1, err
	}
	return vmid, nil
}

type Record struct {
	Start int64
	End   int64

	State stpb.NoCloudState
}

func MakeRecord(from, to int, state stpb.NoCloudState) (res Record) {
	return Record{int64(from), int64(to), state}
}

func MakeTimeline(vm *vm.VM) (res []Record) {
	history := vm.HistoryRecords
	for _, record := range history {
		res = append(res, MakeTimelineRecords(record)...)
	}
	return res
}

func FilterTimeline(tl []Record, from, to int64) (res []Record) {
	for _, cr := range tl {
		r := Record(cr)
		if r.End == 0 {
			if r.Start == 0 {
				continue
			}
			r.End = to
		}
		if r.End < from || r.Start > to {
			continue
		}
		if r.Start < from {
			r.Start = from
		}
		if r.End > to {
			r.End = to
		}
		res = append(res, r)
	}
	return res
}

func MakeTimelineRecords(r vm.HistoryRecord) (res []Record) {
	res = append(res, MakeRecord(r.RSTime, r.RETime, stpb.NoCloudState_RUNNING))
	switch r.Action {
	case 9, 10: // suspended
		res = append(res, MakeRecord(r.RETime, r.RETime, stpb.NoCloudState_SUSPENDED))
	case 20: // powered off
		res = append(res, MakeRecord(r.RETime, r.RETime, stpb.NoCloudState_STOPPED))
	case 27, 28: // terminated (hard)
		res = append(res, MakeRecord(r.RETime, r.RETime, stpb.NoCloudState_DELETED))
	}

	return res
}
