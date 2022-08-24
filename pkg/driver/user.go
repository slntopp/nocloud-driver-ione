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
	"fmt"
	"strconv"
	"time"

	"github.com/OpenNebula/one/src/oca/go/src/goca/dynamic"
	"github.com/OpenNebula/one/src/oca/go/src/goca/parameters"
	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/shared"
	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/user"
	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/vm"
	pb "github.com/slntopp/nocloud/pkg/instances/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

func (c *ONeClient) GetUser(id int) (*user.User, error) {
	uc := c.ctrl.User(id)
	return uc.Info(true)
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
