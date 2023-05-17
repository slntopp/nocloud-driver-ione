package actions

import (
	"encoding/json"
	one "github.com/slntopp/nocloud-driver-ione/pkg/driver"
	sppb "github.com/slntopp/nocloud-proto/services_providers"
	"google.golang.org/protobuf/types/known/structpb"
	"strconv"
	"strings"
)

type SPAction func(one.IClient, map[string]*structpb.Value) (*sppb.InvokeResponse, error)

var SpActions = map[string]SPAction{
	"get_users": GetUsers,
}

func GetUsers(client one.IClient, data map[string]*structpb.Value) (*sppb.InvokeResponse, error) {
	usersPool, err := client.GetUsers()
	if err != nil {
		return nil, err
	}

	var usersInfo []interface{}

	for _, val := range usersPool.Users {
		var userInfo = make(map[string]interface{})
		var userVmsInfo []interface{}
		userInfo["userid"] = val.ID
		userVms, _ := client.GetUserVMS(val.ID)
		userNetworks, _ := client.GetUserVNets(val.ID)
		for _, vm := range userVms.VMs {
			var vmInfo = make(map[string]interface{})
			var config = make(map[string]interface{})
			var data = make(map[string]interface{})
			var resources = make(map[string]interface{})
			data["vmid"] = vm.ID
			data["vm_name"] = vm.Name
			resources["cpu"], _ = vm.Template.GetVCPU()
			resources["ram"], _ = vm.Template.GetMemory()
			resources["drive_type"], _ = vm.Template.GetStrFromVec("DISK", "DRIVE_TYPE")
			driveSize, _ := vm.Template.GetStrFromVec("DISK", "SIZE")
			resources["drive_size"], _ = strconv.Atoi(driveSize)
			config["template_id"], _ = vm.Template.GetInt("TEMPLATE_ID")
			config["password"], _ = vm.UserTemplate.GetStr("PASSWORD")
			publicIps, privateIps := 0, 0
			nics := vm.Template.GetVectors("NIC")
			for _, nic := range nics {
				str, _ := nic.GetStr("NETWORK")
				if strings.HasSuffix(str, "pub-vnet") {
					publicIps += 1
				} else {
					privateIps += 1
				}
			}
			resources["ips_public"] = publicIps
			resources["ips_private"] = privateIps

			vmInfo["config"] = config
			vmInfo["data"] = data
			vmInfo["resources"] = resources

			userVmsInfo = append(userVmsInfo, vmInfo)
		}
		userInfo["vms"] = userVmsInfo
		for _, network := range userNetworks.VirtualNetworks {
			if strings.HasSuffix(network.Name, "pub-vnet") {
				userInfo["public_vn"] = network.ID
			} else {
				userInfo["public_vn"] = network.ID
			}
		}
		usersInfo = append(usersInfo, userInfo)
	}

	var response interface{}
	marshal, err := json.Marshal(usersInfo)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(marshal, &response)
	if err != nil {
		return nil, err
	}

	responcePb, err := structpb.NewValue(response)
	if err != nil {
		return nil, err
	}

	return &sppb.InvokeResponse{
		Result: true,
		Meta: map[string]*structpb.Value{
			"users": responcePb,
		},
	}, nil
}
