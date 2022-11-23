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
	"fmt"

	"github.com/slntopp/nocloud-proto/services_providers"
	"google.golang.org/protobuf/types/known/structpb"
)

var (
	// OpenNebula Scheduler Requirements (nodes provisioning rules for VMs deploy)
	SCHED = "sched"
	// OpenNebula DataStore Scheduler Requirements (datastores provisioning rules for VMs deploy)
	SCHED_DS = "sched_ds"
	// OpenNebula Super VNet public IP addresses to be reserved from
	PUBLIC_IP_POOL = "public_ip_pool"
	// OpenNebula Super VNet private IP addresses to be reserved from
	PRIVATE_VN_TEMPLATE = "private_vnet_tmpl"
	// OpenNebula Super VNet private IP addresses to be reserved from
	PRIVATE_VN_BAN = "private_vnet_ban"

	// OpenNebula VM Name Data Key
	DATA_VM_NAME = "vm_name"
	// OpenNebula VM ID Data Key
	DATA_VM_ID = "vmid"

	// Logo of OpenNebula Template
	DATA_LOGO = "logo"

	// OpenNebula minimal drive size
	MIN_DRIVE_SIZE = "min_drive_size"
	// OpenNebula maximum drive size
	MAX_DRIVE_SIZE = "max_drive_size"
)

func GetVarValue(in *services_providers.Var, key string) (r *structpb.Value, err error) {
	let := in.GetValue()
	r, ok := let[key]
	if ok {
		return r, nil
	}
	r, ok = let["default"]
	if ok {
		return r, nil
	}
	return nil, fmt.Errorf("keys '%s' and 'default' are not set", key)
}
