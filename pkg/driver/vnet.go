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
	"math/big"

	"github.com/OpenNebula/one/src/oca/go/src/goca/parameters"
	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/shared"
	vnet "github.com/OpenNebula/one/src/oca/go/src/goca/schemas/virtualnetwork"
	sppb "github.com/slntopp/nocloud/pkg/services_providers/proto"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

var (
	USER_PUBLIC_VNET_NAME_PATTERN  = "user-%d-pub-vnet"
	USER_PRIVATE_VNET_NAME_PATTERN = "user-%d-private-vnet"
)

func (c *ONeClient) ReservePublicIP(u, n int) (pool_id int, err error) {
	public_pool_id, ok := c.vars[PUBLIC_IP_POOL]
	if !ok {
		return -1, errors.New("VNet ID is not set")
	}

	id, err := GetVarValue(public_pool_id, "default")
	if err != nil {
		return -1, err
	}
	public_pool, err := c.GetVNet(int(id.GetNumberValue()))
	if err != nil {
		return -1, err
	}
	user_pub_net_id, err := c.GetUserPublicVNet(u)
	if err != nil {
		user_pub_net_id = -1
	}
	for i := 0; i < n; i++ {
		user_pub_net_id, err = c.ReserveVNet(
			public_pool.ID, 1, user_pub_net_id,
			fmt.Sprintf(USER_PUBLIC_VNET_NAME_PATTERN, u))
		if err != nil {
			return -1, err
		}
	}

	c.Chown(
		"vn", user_pub_net_id,
		u, int(c.secrets["group"].GetNumberValue()))
	c.Chmod(
		"vn", user_pub_net_id,
		&shared.Permissions{
			OwnerU: 1, OwnerM: 1, OwnerA: 0,
			GroupU: 0, GroupM: 0, GroupA: 0,
			OtherU: 0, OtherM: 0, OtherA: 0,
		},
	)
	c.UpdateVNet(user_pub_net_id, "TYPE=\"PUBLIC\"", parameters.Merge)

	return user_pub_net_id, nil
}

func (c *ONeClient) FindFreeVlan(sp *sppb.ServicesProvider) (vnMad string, freeVlan int, err error) {
	vlans, ok := sp.Secrets["vlans"]
	if !ok {
		err := fmt.Errorf("no vlans in sp config, uuid: %s", sp.GetUuid())
		c.log.Error("Can't Reserve Private IPs", zap.Error(err))
		return "", -1, status.Error(codes.Internal, "Couldn't reserve Private IP addresses")
	}

	vnMad = ""
	freeVlan = -1
	if privateVNTmplVar, ok := sp.Vars[PRIVATE_VN_POOL]; ok {
		tmplId, err := GetVarValue(privateVNTmplVar, "default")
		if err != nil {
			c.log.Error("Can't Reserve Private IPs", zap.Error(err))
			return "", -1, status.Error(codes.Internal, "Couldn't reserve Private IP addresses")
		}

		vnt, err := c.ctrl.VNTemplate(int(tmplId.GetNumberValue())).Info(true)
		if err != nil {
			c.log.Error("Can't Reserve Private IPs", zap.Error(err))
			return "", -1, status.Error(codes.Internal, "Couldn't reserve Private IP addresses")
		}

		vnMad, err = vnt.Template.GetStr("VN_MAD")
		if err != nil {
			c.log.Error("Can't Reserve Private IPs", zap.Error(err))
			return "", -1, status.Error(codes.Internal, "Couldn't reserve Private IP addresses")
		}

		info := vlans.GetStructValue().Fields[vnMad]
		startValue, ok := info.GetStructValue().GetFields()["start"]
		if !ok {
			err := fmt.Errorf("no vlans' start in config, sp uuid: %s, vn_mad: %s", sp.GetUuid(), vnMad)
			c.log.Error("Can't Reserve Private IPs", zap.Error(err))
			return "", -1, status.Error(codes.Internal, "Couldn't reserve Private IP addresses")
		}
		start := int(startValue.GetNumberValue())

		sizeValue, ok := info.GetStructValue().GetFields()["size"]
		if !ok {
			err := fmt.Errorf("no vlans' size in config, sp uuid: %s, vn_mad: %s", sp.GetUuid(), vnMad)
			c.log.Error("Can't Reserve Private IPs", zap.Error(err))
			return "", -1, status.Error(codes.Internal, "Couldn't reserve Private IP addresses")
		}
		size := int(sizeValue.GetNumberValue())

		state := sp.GetState()
		if state == nil {
			err := fmt.Errorf("coulnd't get State of the ServicesProvider(%s)", sp.GetUuid())
			c.log.Error("Can't Reserve Private IPs", zap.Error(err))
			return "", -1, status.Error(codes.Internal, "Couldn't reserve Private IP addresses")
		}
		networking, ok := state.Meta["networking"]
		if !ok {
			err := fmt.Errorf("networking not found for ServicesProvider(%s)", sp.GetUuid())
			c.log.Error("Can't Reserve Private IPs", zap.Error(err))
			return "", -1, status.Error(codes.Internal, "Couldn't reserve Private IP addresses")
		}

		privateVnet, ok := networking.GetStructValue().Fields["private_vnet"]
		if !ok {
			err := fmt.Errorf("private VNet Template ID not found for ServicesProvider(%s)", sp.GetUuid())
			c.log.Error("Can't Reserve Private IPs", zap.Error(err))
			return "", -1, status.Error(codes.Internal, "Couldn't reserve Private IP addresses")
		}

		freeVlans, ok := privateVnet.GetStructValue().Fields["free_vlans"]
		if !ok {
			err := fmt.Errorf("VLans config not found for ServicesProvider(%s)", sp.GetUuid())
			c.log.Error("Can't Reserve Private IPs", zap.Error(err))
			return "", -1, status.Error(codes.Internal, "Couldn't reserve Private IP addresses")
		}

		vnMadFreeVlans, ok := freeVlans.GetStructValue().Fields[vnMad]
		if !ok {
			vnMadFreeVlans = structpb.NewStringValue("0")
		}

		freeVlansBitSet, ok := big.NewInt(0).SetString(vnMadFreeVlans.GetStringValue(), 10)
		if !ok {
			err := fmt.Errorf("can't convert free vlans info to big.Int, ServicesProvider(%s), vn_mad: %s", sp.GetUuid(), vnMad)
			c.log.Error("Can't Reserve Private IPs", zap.Error(err))
			return "", -1, status.Error(codes.Internal, "Couldn't reserve Private IP addresses")
		}

		for i := start; i < start+size; i++ {
			if freeVlansBitSet.Bit(i) == 0 {
				freeVlan = i
				break
			}
		}
	}

	if freeVlan == -1 {
		err := fmt.Errorf("free VLan not found, ServicesProvider(%s), vn_mad: %s", sp.GetUuid(), vnMad)
		c.log.Error("Can't Reserve Private IPs", zap.Error(err))
		return "", -1, status.Error(codes.Internal, "Couldn't reserve Private IP addresses")
	}

	return vnMad, freeVlan, nil
}

func (c *ONeClient) ReservePrivateIP(u int, vnMad string, vlanID int) (pool_id int, err error) {
	private_tmpl_id, ok := c.vars[PRIVATE_VN_POOL]
	if !ok {
		return -1, errors.New("VNet Tmpl ID is not set")
	}

	id, err := GetVarValue(private_tmpl_id, "default")
	if err != nil {
		return -1, err
	}

	private_vnet_name := fmt.Sprintf(USER_PRIVATE_VNET_NAME_PATTERN, u)
	private_ar := "AR = [\n	IP = \"10.0.0.0\",\n	SIZE = \"255\",\n	TYPE = \"IP4\" ]"
	private_vlan := fmt.Sprintf("VLAN_ID = %d\nAUTOMATIC_VLAN_ID = \"NO\"", vlanID)
	private_vn_mad := fmt.Sprintf("VN_MAD = \"%s\"", vnMad)
	private_bridge := fmt.Sprintf("BRIDGE = user-%d-vlan-%d", u, vlanID)

	extra := private_ar + "\n" + private_vlan + "\n" + private_vn_mad + "\n" + private_bridge

	user_private_net_id, err := c.ctrl.VNTemplate(int(id.GetNumberValue())).Instantiate(private_vnet_name, extra)
	if err != nil {
		return -1, err
	}

	c.Chown(
		"vn", user_private_net_id,
		u, int(c.secrets["group"].GetNumberValue()))
	c.Chmod(
		"vn", user_private_net_id,
		&shared.Permissions{
			OwnerU: 1, OwnerM: 1, OwnerA: 0,
			GroupU: 0, GroupM: 0, GroupA: 0,
			OtherU: 0, OtherM: 0, OtherA: 0,
		},
	)
	c.UpdateVNet(user_private_net_id, "TYPE=\"PRIVATE\"", parameters.Merge)

	return user_private_net_id, nil
}

func (c *ONeClient) GetVNet(id int) (*vnet.VirtualNetwork, error) {
	vnc := c.ctrl.VirtualNetwork(id)
	return vnc.Info(true)
}

func (c *ONeClient) DeleteVNet(id int) error {
	vnc := c.ctrl.VirtualNetwork(id)
	return vnc.Delete()
}

func (c *ONeClient) DeleteUserAndVNets(id int) error {
	if pubVn, err := c.GetUserPublicVNet(id); err == nil {
		err = c.DeleteVNet(pubVn)
		if err != nil {
			c.log.Debug("Couldn't Delete Pub VNet", zap.Error(err), zap.Int("user", id), zap.Int("vnet_id", pubVn))
		}
	}

	if privateVn, err := c.GetUserPrivateVNet(id); err == nil {
		err = c.DeleteVNet(privateVn)
		if err != nil {
			c.log.Debug("Couldn't Delete Private VNet", zap.Error(err), zap.Int("user", id), zap.Int("vnet_id", privateVn))
		}
	}

	err := c.DeleteUser(id)
	if err != nil {
		c.log.Debug("Couldn't Delete User", zap.Error(err), zap.Int("user", id))
	}

	return err
}

func (c *ONeClient) GetUserPublicVNet(user int) (id int, err error) {
	vnsc := c.ctrl.VirtualNetworks()
	return vnsc.ByName(fmt.Sprintf(USER_PUBLIC_VNET_NAME_PATTERN, user), user)
}

func (c *ONeClient) GetUserPrivateVNet(user int) (id int, err error) {
	vnsc := c.ctrl.VirtualNetworks()
	return vnsc.ByName(fmt.Sprintf(USER_PRIVATE_VNET_NAME_PATTERN, user), user)
}

func (c *ONeClient) UpdateVNet(id int, tmpl string, uType parameters.UpdateType) error {
	vnc := c.ctrl.VirtualNetwork(id)
	return vnc.Update(tmpl, uType)
}

// Reserve Addresses to the other VNet
//
//	id - VNet ID to reserve from
//	size - amount of addresses to reserve
//	to - VNet ID to reserve to, if set to -1 new will be created
//	name - name of the new VNet, if set to "", either existing will be used or new - generated
func (c *ONeClient) ReserveVNet(id, size, to int, name string) (int, error) {
	vnc := c.ctrl.VirtualNetwork(id)
	tmpl := fmt.Sprintf("SIZE=%d\n", size)
	if name != "" {
		tmpl += fmt.Sprintf("NAME=%s\n", name)
	}
	if to != -1 {
		tmpl += fmt.Sprintf("NETWORK_ID=%d", to)
	}
	return vnc.Reserve(tmpl)
}
