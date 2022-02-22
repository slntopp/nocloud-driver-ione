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

	"github.com/OpenNebula/one/src/oca/go/src/goca/parameters"
	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/shared"
	vnet "github.com/OpenNebula/one/src/oca/go/src/goca/schemas/virtualnetwork"
)

var (
	USER_PUBLIC_VNET_NAME_PATTERN = "user-%d-pub-vnet"
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
		user_pub_net_id, err = c.ReseveVNet(
			public_pool.ID, 1, user_pub_net_id,
			fmt.Sprintf(USER_PUBLIC_VNET_NAME_PATTERN, u))
		if err != nil {
			return -1, err
		}
	}
	
	c.Chown(
		"vn", user_pub_net_id,
		u, int(c.secrets["group"].GetNumberValue()) )
	c.Chmod(
		"vn", user_pub_net_id,
		&shared.Permissions{
			1, 1, 0,
			0, 0, 0,
			0, 0, 0 },
	)
	c.UpdateVNet(user_pub_net_id, "TYPE=\"PUBLIC\"", parameters.Merge)

	return user_pub_net_id, nil
}

func (c *ONeClient) GetVNet(id int) (*vnet.VirtualNetwork, error) {
	vnc := c.ctrl.VirtualNetwork(id)
	return vnc.Info(true)
}

func (c *ONeClient) GetUserPublicVNet(user int) (id int, err error) {
	vnsc := c.ctrl.VirtualNetworks()
	return vnsc.ByName(fmt.Sprintf(USER_PUBLIC_VNET_NAME_PATTERN, user))
}

func(c *ONeClient) UpdateVNet(id int, tmpl string, uType parameters.UpdateType) error {
	vnc := c.ctrl.VirtualNetwork(id)
	return vnc.Update(tmpl, uType)
}

// Reserve Addresses to the other VNet
// 	id - VNet ID to reserve from
// 	size - amount of addresses to reserve
// 	to - VNet ID to reserve to, if set to -1 new will be created
// 	name - name of the new VNet, if set to "", either existing will be used or new - generated
func (c *ONeClient) ReseveVNet(id, size, to int, name string) (int, error) {
	vnc := c.ctrl.VirtualNetwork(id)
	tmpl := fmt.Sprintf("SIZE=%d\n", size)
	if name != "" {
		tmpl += fmt.Sprintf("NAME=%s\n", name)
	}
	if to != -1 {
		tmpl += fmt.Sprintf("VNET=%d", to)
	}
	return vnc.Reserve(tmpl)
}