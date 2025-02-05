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

	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/shared"

	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/vm"
	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/vm/keys"
	"github.com/slntopp/nocloud-driver-ione/pkg/datas"
	"github.com/slntopp/nocloud-proto/hasher"
	pb "github.com/slntopp/nocloud-proto/instances"
	stpb "github.com/slntopp/nocloud-proto/states"
	statuspb "github.com/slntopp/nocloud-proto/statuses"
	"github.com/slntopp/nocloud/pkg/nocloud/auth"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

func (c *ONeClient) GetUserVMS(userId int) (*vm.Pool, error) {
	return c.ctrl.VMs().InfoExtended(userId)
}

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

func (c *ONeClient) Reinstall(vmid int) error {
	vm := c.ctrl.VM(vmid)
	err := vm.RecoverDeleteRecreate()
	return err
}

func (c *ONeClient) Monitoring(vmid int) (*vm.Monitoring, error) {
	vm := c.ctrl.VM(vmid)
	return vm.Monitoring()
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
		cpu, err := tmpl.GetVCPU()
		if err != nil {
			return nil, err
		}
		inst.Resources["cpu"] = structpb.NewNumberValue(float64(cpu))
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
			c.log.Warn("Error getting Drive Type", zap.Error(err))
			driveType = "NOT FOUND"
		}
		driveSize, err := diskInfo.GetFloat("SIZE")
		if err != nil {
			c.log.Warn("Error getting Drive Size", zap.Error(err))
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
	log := c.log.Named("CheckInstancesGroup").Named(IG.Uuid)
	log.Debug("Begin Monitoring InstancesGroup")
	resp := CheckInstancesGroupResponse{
		ToBeCreated: make([]*pb.Instance, 0),
		ToBeDeleted: make([]*pb.Instance, 0),
		ToBeUpdated: make([]*pb.Instance, 0),
		Valid:       make([]*pb.Instance, 0),
	}

	var userId int
	if id, ok := IG.Data["userid"]; ok {
		userId = int(id.GetNumberValue())
		vmsc := c.ctrl.VMs()
		vms_pool, err := vmsc.Info(userId)
		if err != nil {
			log.Warn("Error Getting VMs Info by UserId", zap.Any("userId", userId), zap.Error(err))
			return nil, err
		}
		instMapForFastSearch := make(map[int]*pb.Instance, len(IG.GetInstances()))
		for _, inst := range IG.GetInstances() {
			log := log.With(zap.String("instance", inst.GetUuid()))
			vmid, err := GetVMIDFromData(c, inst)
			if err != nil {
				log.Warn("Coudln't get VM ID from Instance Data, Instance is not initialized", zap.String("instance", inst.Uuid))
				continue
			}
			instMapForFastSearch[vmid] = inst
		}
		log.Debug("Instances Sync Pre-Flight Check",
			zap.Int("total", len(IG.GetInstances())),
			zap.Int("initialized", len(instMapForFastSearch)),
			zap.Int("vms_found", len(vms_pool.VMs)))

		for _, vm := range vms_pool.VMs {
			inst, ok := instMapForFastSearch[vm.ID]
			log := log.With(zap.String("instance", inst.GetUuid()))
			if !ok || (ok && inst.GetStatus() == statuspb.NoCloudStatus_DEL) {
				log.Debug("VM not found among initialized Instances or should be deleted", zap.Int("vmid", vm.ID))
				vmInst, err := c.VMToInstance(vm.ID)
				if err != nil {
					log.Warn("Error Converting VM to Instance", zap.Error(err))
					continue
				}

				uuid := vmInst.GetData()["vm_name"].GetStringValue()
				vmInst.Uuid = uuid
				resp.ToBeDeleted = append(resp.ToBeDeleted, vmInst)
			}
		}
	}

	for _, inst := range IG.GetInstances() {
		log := log.With(zap.String("instance", inst.GetUuid()))
		log.Debug("Checking instance", zap.Any("body", inst))
		if inst.GetStatus() == statuspb.NoCloudStatus_DEL {
			continue
		}

		vm, err := c.FindVMByInstance(inst)
		if err != nil {
			log.Warn("VM is still not created", zap.Error(err), zap.Any("inst", inst))
			meta := inst.GetBillingPlan().GetMeta()
			if meta == nil {
				meta = make(map[string]*structpb.Value)
			}
			cfg := inst.GetConfig()
			if cfg == nil {
				cfg = make(map[string]*structpb.Value)
			}

			cfgAutoStart := cfg["auto_start"].GetBoolValue()
			metaAutoStart := meta["auto_start"].GetBoolValue()

			if metaAutoStart || cfgAutoStart {
				resp.ToBeCreated = append(resp.ToBeCreated, inst)
			}
			continue
		}

		res, err := c.VMToInstance(vm.ID)
		if err != nil {
			c.log.Error("Error Converting VM to Instance", zap.Error(err))
			continue
		}
		res.Uuid = ""
		res.Title = inst.GetTitle()
		res.Status = statuspb.NoCloudStatus_INIT
		res.BillingPlan = inst.BillingPlan
		res.Data = nil
		res.State = nil

		err = hasher.SetHash(res.ProtoReflect())
		if err != nil {
			c.log.Error("Error Setting Instance Hash", zap.Error(err))
			continue
		}

		c.log.Debug("instance for hash calculating while Monitoring Checking", zap.Any("inst", res))
		c.log.Debug("res.Hash", zap.String("hash", res.Hash))
		c.log.Debug("inst.Hash", zap.String("hash", inst.Hash))

		if res.Hash != inst.Hash {
			resp.ToBeUpdated = append(resp.ToBeUpdated, inst)
		} else {
			resp.Valid = append(resp.Valid, inst)
		}
	}

	return &resp, nil
}

func (c *ONeClient) CheckInstancesGroupResponseProcess(resp *CheckInstancesGroupResponse, ig *pb.InstancesGroup, group int, balance map[string]float64) *CheckInstancesGroupResponse {
	data := ig.GetData()
	userid := int(data["userid"].GetNumberValue())

	instDatasPublisher := datas.DataPublisher(datas.POST_INST_DATA)
	igDatasPublisher := datas.DataPublisher(datas.POST_IG_DATA)

	successResp := CheckInstancesGroupResponse{
		ToBeCreated: make([]*pb.Instance, 0),
		ToBeDeleted: make([]*pb.Instance, 0),
	}

	created := resp.ToBeCreated
	for i := 0; i < len(created); i++ {
		/*
			bp := created[i].GetBillingPlan()

			if bp.GetKind() == billing.PlanKind_STATIC {
				price := bp.GetProducts()[created[i].GetProduct()].GetPrice()

				resources := created[i].GetResources()
				ram := resources["ram"].GetNumberValue() / 1024
				drive_size := resources["drive_size"].GetNumberValue() / 1024
				drive_type := strings.ToLower(resources["drive_type"].GetStringValue())

				for _, res := range bp.GetResources() {
					if res.GetKey() == "ram" {
						price += ram * res.GetPrice()
					} else if res.GetKey() == drive_type {
						price += drive_size * res.GetPrice()
					}
				}

				if price < balance[ig.GetUuid()] {
					continue
				}

				balance[ig.GetUuid()] -= price
			}
		*/

		token, err := auth.MakeTokenInstance(created[i].GetUuid())
		if err != nil {
			c.log.Error("Error generating VM token", zap.String("instance", created[i].GetUuid()), zap.Error(err))
			continue
		}
		vmid, err := c.InstantiateTemplateHelper(created[i], ig, token)
		if err != nil {
			c.log.Error("Error deploying VM", zap.String("instance", created[i].GetUuid()), zap.Error(err))
			continue
		}
		c.Chown("vm", vmid, userid, group)

		created[i].Data["creation"] = structpb.NewNumberValue(float64(time.Now().Unix()))

		go instDatasPublisher(created[i].Uuid, created[i].Data)
		successResp.ToBeCreated = append(successResp.ToBeCreated, created[i])
	}

	deleted := resp.ToBeDeleted
	for i := 0; i < len(deleted); i++ {
		log := c.log.With(zap.String("instance", deleted[i].GetUuid()))
		vmid, err := GetVMIDFromData(c, deleted[i])
		if err != nil {
			log.Error("Error Getting VMID from Data", zap.Error(err))
			continue
		}
		c.TerminateVM(vmid, true)

		go igDatasPublisher(ig.Uuid, data)
		successResp.ToBeDeleted = append(successResp.ToBeDeleted, deleted[i])
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
			tmpl.VCPU(int(inst.Resources["cpu"].GetNumberValue()))
			updated = append(updated, "cpu")
		}

		if vmInst.Resources["ram"].GetNumberValue() != inst.Resources["ram"].GetNumberValue() {
			tmpl.Memory(int(inst.Resources["ram"].GetNumberValue()))
			updated = append(updated, "ram")
		}

		vmInstIpsPublic := int(vmInst.Resources["ips_public"].GetNumberValue())
		instIpsPublic := int(inst.Resources["ips_public"].GetNumberValue())

		vmInstIpsPrivate := int(vmInst.Resources["ips_private"].GetNumberValue())
		instIpsPrivate := int(inst.Resources["ips_private"].GetNumberValue())

		public_vn := int(data["public_vn"].GetNumberValue())
		private_vn := int(data["private_vn"].GetNumberValue())

		publicNetwork := c.ctrl.VirtualNetwork(public_vn)
		publicNetworkInfo, err := publicNetwork.Info(true)
		if err != nil {
			c.log.Error("Failed to get public networks info", zap.Error(err))
			continue
		}

		for _, val := range publicNetworkInfo.ARs {
			if val.UsedLeases == "0" {
				ipId, _ := strconv.Atoi(val.ID)
				err := publicNetwork.FreeAR(ipId)
				if err != nil {
					c.log.Error("Bruh free ip in public network", zap.Int("ip_id", ipId))
				}
			}
		}

		if vmInstIpsPublic != instIpsPublic {
			if vmInstIpsPublic < instIpsPublic {
				uid := int(data["userid"].GetNumberValue())
				_, err := c.ReservePublicIP(uid, 1)
				if err != nil {
					c.log.Error("Wrong ip reserv")
				}
				networkTemplate := vm.NewTemplate()
				nic := networkTemplate.AddNIC()
				nic.Add(shared.NetworkID, public_vn)
				err = vmc.AttachNIC(networkTemplate.String())
				if err != nil {
					c.log.Error("Wrong ip attach")
				}

				go igDatasPublisher(ig.Uuid, data)

			} else {
				nics := VM.Template.GetNICs()
				for i := len(nics) - 1; i > 0; i-- {
					nicId, netType := -1, ""
					pairs := nics[i].Vector.Pairs
					for j := 0; j < len(pairs); j++ {
						if pairs[j].XMLName.Local == "NETWORK" {
							if strings.Contains(pairs[j].Value, "pub-vnet") {
								netType = "pub-vnet"
							} else {
								netType = "private-vnet"
							}
							continue
						}
						if pairs[j].XMLName.Local == "NIC_ID" {
							nicId, _ = strconv.Atoi(pairs[j].Value)
						}
					}

					if netType == "pub-vnet" {
						err := vmc.DetachNIC(nicId)
						if err != nil {
							c.log.Error("id", zap.Int("id", nicId))
							c.log.Error("Wrong ip detach")
						}

						go igDatasPublisher(ig.Uuid, data)

						break
					}
				}
			}
		} else if vmInstIpsPrivate != instIpsPrivate {
			private_vn_ban, ok := c.vars[PRIVATE_VN_BAN]
			if !ok {
				break
			}
			private_vn_ban_value, err := GetVarValue(private_vn_ban, "default")
			if err != nil {
				break
			}

			if !private_vn_ban_value.GetBoolValue() {
				if vmInstIpsPrivate < instIpsPrivate {
					networkTemplate := vm.NewTemplate()
					nic := networkTemplate.AddNIC()
					nic.Add(shared.NetworkID, private_vn)
					err = vmc.AttachNIC(networkTemplate.String())
					if err != nil {
						c.log.Error("Wrong ip attach")
					}
				} else {
					nics := VM.Template.GetNICs()

					for i := len(nics) - 1; i > 0; i-- {
						nicId, netType := -1, ""
						pairs := nics[i].Vector.Pairs
						for j := 0; j < len(pairs); j++ {
							if pairs[j].XMLName.Local == "NETWORK" {
								if strings.Contains(pairs[j].Value, "pub-vnet") {
									netType = "pub-vnet"
								} else {
									netType = "private-vnet"
								}
								continue
							}
							if pairs[j].XMLName.Local == "NIC_ID" {
								nicId, _ = strconv.Atoi(pairs[j].Value)
							}
						}

						if netType == "private-vnet" {
							err := vmc.DetachNIC(nicId)
							if err != nil {
								c.log.Error("id", zap.Int("id", nicId))
								c.log.Error("Wrong ip detach")
							}
							break
						}
					}
				}
			}
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
		if inst.GetState() == nil {
			inst.State = &stpb.State{
				Meta: map[string]*structpb.Value{},
			}
		}
		if inst.GetState().GetMeta() == nil {
			inst.State.Meta = make(map[string]*structpb.Value)
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

	return &successResp
}

func GetVMIDFromData(client IClient, inst *pb.Instance) (vmid int, err error) {
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
		res = append(res, MakeRecord(r.RETime, r.ETime, stpb.NoCloudState_SUSPENDED))
	case 20: // powered off
		res = append(res, MakeRecord(r.RETime, r.ETime, stpb.NoCloudState_STOPPED))
	case 27, 28: // terminated (hard)
		res = append(res, MakeRecord(r.RETime, r.ETime, stpb.NoCloudState_DELETED))
	}

	return res
}

type VmResourceDiff struct {
	ResName     string
	OldResCount float64
	NewResCount float64
}

func (c *ONeClient) GetVmResourcesDiff(inst *pb.Instance) []*VmResourceDiff {
	var res []*VmResourceDiff

	vmid, err := GetVMIDFromData(c, inst)
	if err != nil {
		c.log.Error("Error Getting VMID from Data", zap.Error(err))
		return res
	}

	vmInst, err := c.VMToInstance(vmid)
	if err != nil {
		c.log.Error("Error Converting VM to Instance", zap.Error(err))
		return res
	}

	vmInstIpsPublic := int(vmInst.Resources["ips_public"].GetNumberValue())
	instIpsPublic := int(inst.Resources["ips_public"].GetNumberValue())

	if vmInstIpsPublic != instIpsPublic {
		res = append(res, &VmResourceDiff{
			ResName:     "ips_public",
			OldResCount: float64(vmInstIpsPublic),
			NewResCount: float64(instIpsPublic),
		})
	}

	vmInstDriveSize := vmInst.Resources["drive_size"].GetNumberValue()
	instDriveSize := inst.Resources["drive_size"].GetNumberValue()

	if vmInstDriveSize != instDriveSize {
		driveType := inst.Resources["drive_type"].GetStringValue()

		res = append(res, &VmResourceDiff{
			ResName:     strings.ToLower(fmt.Sprintf("drive_%s", driveType)),
			OldResCount: vmInstDriveSize / 1024.0,
			NewResCount: instDriveSize / 1024.0,
		})
	}

	return res
}
