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
	"time"

	"github.com/OpenNebula/one/src/oca/go/src/goca/dynamic"
	"github.com/OpenNebula/one/src/oca/go/src/goca/parameters"
	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/user"
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

// Transfer ownership of vm, public and private networks from old user to new.
func transferOwnership(c *ONeClient, inst *pb.Instance, newUserID int, oldUserID int, userGroup int) error {
	vmid, err := GetVMIDFromData(c, inst)
	if err != nil {
		return status.Error(codes.NotFound, "Can't get vm id by data")
	}

	if err := c.Chown("vm", vmid, newUserID, int(userGroup)); err != nil {
		return status.Error(codes.Internal, "Can't change ownership of the vm")
	}

	publicNet, err := c.GetUserPrivateVNet(oldUserID)
	if err != nil {
		return status.Errorf(codes.NotFound, "Can't find private net while creating new user. Old user id = %d", oldUserID)
	}

	privateNet, err := c.GetUserPublicVNet(oldUserID)
	if err != nil {
		return status.Errorf(codes.NotFound, "Can't find public net while creating new user. Old user id = %d", oldUserID)
	}

	if err := c.Chown("vn", privateNet, newUserID, int(userGroup)); err != nil {
		return status.Error(codes.Internal, "Can't change ownership of old private network")
	}

	if err := c.Chown("vn", publicNet, newUserID, int(userGroup)); err != nil {
		return status.Error(codes.Internal, "Can't change ownership of old public network")
	}

	return nil
}

// Check if user related to the Instance Group exists.
//
// If not, create a new user and change ownership of virtual machines to this user.
func (c *ONeClient) CheckOrphanInstanceGroup(instanceGroup *pb.InstancesGroup, userGroup float64) error {
	data := instanceGroup.Data

	oldUserID := int(data["userid"].GetNumberValue())
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

	for _, inst := range instanceGroup.GetInstances() {
		if err := transferOwnership(c, inst, newUserID, oldUserID, int(userGroup)); err != nil {
			uc := c.ctrl.User(newUserID)
			uc.Delete()
			return err
		}
	}

	// Should only happen if everything is ok
	data["userid"] = structpb.NewNumberValue(float64(newUserID))

	return nil
}
