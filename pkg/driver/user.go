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
	"crypto/sha256"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/OpenNebula/one/src/oca/go/src/goca/dynamic"
	"github.com/OpenNebula/one/src/oca/go/src/goca/parameters"
	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/shared"
	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/user"
	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/vm"
	pb "github.com/slntopp/nocloud-proto/instances"
	sppb "github.com/slntopp/nocloud-proto/services_providers"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

func (c *ONeClient) GetUser(id int) (*user.User, error) {
	uc := c.ctrl.User(id)
	return uc.Info(true)
}

func (c *ONeClient) GetUsers() (*user.Pool, error) {
	return c.ctrl.Users().Info()
}

func (c *ONeClient) CreateUser(name, pass string, groups []int) (id int, err error) {
	uc := c.ctrl.Users()
	return uc.Create(name, pass, "core", groups)
}

func (c *ONeClient) DeleteUser(id int) error {
	uc := c.ctrl.User(id)
	return uc.Delete()
}

func (c *ONeClient) UserAddAttribute(id int, data map[string]interface{}) error {
	uc := c.ctrl.User(id)
	tmpl := dynamic.NewTemplate()
	for k, v := range data {
		tmpl.AddPair(k, v)
	}
	return uc.Update(tmpl.String(), parameters.Merge)
}

func (c *ONeClient) waitForHotplugFinish(vmid int) {
	for {
		_, _, lcmState, _, _ := c.StateVM(vmid)
		if lcmState == int(vm.HotplugNic) {
			time.Sleep(time.Second * 1)
		} else {
			return
		}
	}
}

func (c *ONeClient) WaitForPoweroff(vmid int) {
	for {
		vmState, _, _, _, _ := c.StateVM(vmid)
		if vmState == int(vm.Poweroff) {
			return
		}
		time.Sleep(1 * time.Second)
	}
}

// Check if user related to the Instance Group exists.
//
// If not, create a new user and change ownership of virtual machines to this user.
func (c *ONeClient) CheckOrphanInstanceGroup(instanceGroup *pb.InstancesGroup, userGroup float64) error {
	instances := instanceGroup.GetInstances()
	if len(instances) == 0 || instanceGroup.Data["userid"] == nil {
		return nil
	}

	vmid, err := GetVMIDFromData(c, instances[0])
	vmc := c.ctrl.VM(vmid)
	if err != nil {
		return status.Error(codes.NotFound, "Can't get VM id by data")
	}

	vmInfo, err := vmc.Info(true)
	if err != nil {
		return err
	}

	oldUserID := vmInfo.UID
	username := instanceGroup.GetUuid()
	if _, err := c.ctrl.Users().ByName(username); err == nil {
		return nil
	}

	c.log.Warn("Old user not found. Changing user to new user", zap.Any("usergroup", userGroup), zap.String("ig", instanceGroup.GetUuid()))
	hasher := sha256.New()
	hasher.Write([]byte(username + time.Now().String()))
	pass := base64.URLEncoding.EncodeToString(hasher.Sum(nil))
	newUserID, err := c.CreateUser(username, pass, []int{int(userGroup)})
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}

	privateNet, err := c.GetUserPrivateVNet(oldUserID)
	if err != nil {
		return status.Errorf(codes.NotFound, "Can't find private net. Old user id = %d", oldUserID)
	}

	publicNet, err := c.GetUserPublicVNet(oldUserID)
	if err != nil {
		return status.Errorf(codes.NotFound, "Can't find public net. Old user id = %d", oldUserID)
	}

	if err := c.Chown("vn", privateNet, newUserID, int(userGroup)); err != nil {
		return status.Error(codes.Internal, "Can't change ownership of old private network")
	}

	if err := c.Chown("vn", publicNet, newUserID, int(userGroup)); err != nil {
		return status.Error(codes.Internal, "Can't change ownership of old public network")
	}

	for _, inst := range instances {
		vmid, err := GetVMIDFromData(c, inst)
		vmc := c.ctrl.VM(vmid)
		if err != nil {
			return status.Error(codes.NotFound, "Can't get VM id by data")
		}

		vm, err := vmc.Info(true)
		if err != nil {
			return err
		}
		for _, nic := range vm.Template.GetNICs() {

			value, _ := nic.Get(shared.NICID)
			id, _ := strconv.Atoi(value)

			if err := vmc.DetachNIC(id); err != nil {
				return err
			}
			c.waitForHotplugFinish(vmid)
		}
	}

	privateNetController := c.ctrl.VirtualNetwork(privateNet)
	publicNetController := c.ctrl.VirtualNetwork(publicNet)

	if err := privateNetController.Rename(fmt.Sprintf(USER_PRIVATE_VNET_NAME_PATTERN, newUserID)); err != nil {
		c.DeleteUser(newUserID)
		return status.Error(codes.Internal, err.Error())
	}
	if err := publicNetController.Rename(fmt.Sprintf(USER_PUBLIC_VNET_NAME_PATTERN, newUserID)); err != nil {
		c.DeleteUser(newUserID)
		privateNetController.Rename(fmt.Sprintf(USER_PRIVATE_VNET_NAME_PATTERN, newUserID))
		return status.Error(codes.Internal, err.Error())
	}

	for _, inst := range instances {
		vmid, err := GetVMIDFromData(c, inst)
		resources := instanceGroup.Resources
		vmc := c.ctrl.VM(vmid)
		if err != nil {
			return status.Error(codes.NotFound, "Can't get VM id by data")
		}

		if err := c.Chown("vm", vmid, newUserID, int(userGroup)); err != nil {
			return status.Error(codes.Internal, "Can't change ownership of the vm")
		}

		for i := 0; i < int(resources["ips_private"].GetNumberValue()); i++ {
			template := vm.NewTemplate()
			nic := template.AddNIC()
			nic.Add(shared.NetworkID, privateNet)
			if err := vmc.AttachNIC(template.String()); err != nil {
				return err
			}
			c.waitForHotplugFinish(vmid)
		}

		for i := 0; i < int(resources["ips_public"].GetNumberValue()); i++ {
			template := vm.NewTemplate()
			nic := template.AddNIC()
			nic.Add(shared.NetworkID, publicNet)
			if err := vmc.AttachNIC(template.String()); err != nil {
				return err
			}
			c.waitForHotplugFinish(vmid)
		}
	}

	// Should only happen if everything is ok
	instanceGroup.Data["userid"] = structpb.NewNumberValue(float64(newUserID))

	return nil
}

type VMQuota struct {
	XMLName xml.Name `xml:"VM_QUOTA"`
	CPU     float64  `xml:"VM>CPU"`
	Memory  int      `xml:"VM>MEMORY"`
}

type DataStoreQuota struct {
	XMLName xml.Name `xml:"DATASTORE"`
	ID      int      `xml:"ID"`
	Size    int      `xml:"SIZE"`
}
type DataStoreQuotaList struct {
	XMLName xml.Name `xml:"DATASTORE_QUOTA"`
	DS      []DataStoreQuota
}

type NetworkQuota struct {
	XMLName xml.Name `xml:"NETWORK_QUOTA"`
	ID      int      `xml:"NETWORK>ID"`
	Leases  int      `xml:"NETWORK>LEASES"`
}

// Generates Quotas Templates from InstancesGroup resources and SP configuration
// Quota consists of 3 parts:
// 1. VM_QUOTA - VM -> { CPU, MEMORY }
// 2. NETWORK_QUOTA - NETWORK -> { ID, LEASES }
// 3. DATASTORE_QUOTA - DATASTORE -> { ID, SIZE }
func GenerateQuotaFromIGroup(_log *zap.Logger, igroup *pb.InstancesGroup, sp *sppb.ServicesProvider) (quotas []string) {
	log := _log.Named("GenerateQuotaFromIGroup")
	resources := igroup.GetResources()

	var bytes []byte
	var err error

	vm_quota := VMQuota{}
	if cpus := resources["cpu"]; cpus != nil && cpus.GetNumberValue() > 0 {
		vm_quota.CPU = cpus.GetNumberValue()
	}
	if memory := resources["ram"]; memory != nil && memory.GetNumberValue() > 0 {
		vm_quota.Memory = int(memory.GetNumberValue())
	}

	bytes, err = xml.Marshal(vm_quota)
	if err != nil {
		log.Error("Can't marshal VM quota", zap.Error(err))
		return
	}

	quotas = append(quotas, string(bytes))

	vn_quota := NetworkQuota{}
	if resources["ips_public"] != nil { // Private IPs don't need quota as the separate network is created for each User
		public_pool_id, ok := sp.GetVars()[PUBLIC_IP_POOL]
		if ok {
			id, err := GetVarValue(public_pool_id, "default") // Seeking for super vnet id
			if err == nil {
				vn_quota.ID = int(id.GetNumberValue())
				vn_quota.Leases = int(resources["ips_public"].GetNumberValue())

				bytes, err := xml.Marshal(vn_quota)
				if err != nil {
					log.Error("Can't marshal Netowrks quota", zap.Error(err))
					return
				}

				quotas = append(quotas, string(bytes))
			} else {
				log.Warn("PUBLIC_IP_POOL is not set or malformed", zap.Error(err))
			}
		} else {
			log.Warn("PUBLIC_IP_POOL is not set")
		}
	}

	ds_quotas := []DataStoreQuota{}

	sched_ds, ok := sp.GetVars()[SCHED_DS]
	if !ok || sched_ds == nil {
		log.Warn("SCHED_DS is not set")
		return quotas
	}

	dss := map[string]int{}

	// Seeking for Datastores IDs in SCHED_DS
	// TODO: refactor this and extend to use several datastores if rule is set so
	re := regexp.MustCompile("[0-9]+")
	for key, ds := range sched_ds.GetValue() {
		ids := re.FindAllString(ds.GetStringValue(), 1)
		if len(ids) == 0 {
			log.Warn("Can't parse datastore id from SCHED_DS", zap.String("key", key), zap.String("value", ds.GetStringValue()))
			continue
		}
		dss[strings.ToLower(key)], err = strconv.Atoi(ids[0])
		if err != nil {
			log.Warn("Can't parse datastore id from SCHED_DS (not an integer)", zap.String("key", key), zap.String("value", ds.GetStringValue()))
			continue
		}
	}

	for resource, val := range resources {
		if !strings.HasPrefix(resource, "drive_") {
			continue
		}

		key := strings.TrimPrefix(resource, "drive_")
		if err != nil {
			log.Error("Can't parse drive type key", zap.Error(err))
			continue
		}

		ds_quota := DataStoreQuota{
			ID:   dss[key],
			Size: int(val.GetNumberValue()),
		}

		ds_quotas = append(ds_quotas, ds_quota)
	}

	bytes, err = xml.Marshal(DataStoreQuotaList{DS: ds_quotas})
	if err != nil {
		log.Error("Can't marshal VM quota", zap.Error(err))
		return
	}

	quotas = append(quotas, string(bytes))

	return quotas
}

func (c *ONeClient) SetQuotaFromConfig(one_id int, igroup *pb.InstancesGroup, sp *sppb.ServicesProvider) error {
	tpl := GenerateQuotaFromIGroup(c.log, igroup, sp)
	c.log.Debug("SetQuotaFromConfig", zap.Strings("quotas", tpl))

	for _, quota := range tpl {
		if err := c.ctrl.User(one_id).Quota(quota); err != nil {
			return err
		}
	}

	return nil
}
