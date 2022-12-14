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

	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/shared"
	tmpl "github.com/OpenNebula/one/src/oca/go/src/goca/schemas/template"
	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/vm"
	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/vm/keys"
	driver_shared "github.com/slntopp/nocloud-driver-ione/pkg/shared"
	pb "github.com/slntopp/nocloud-proto/instances"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/structpb"
)

func (c *ONeClient) GetTemplate(id int) (*tmpl.Template, error) {
	tc := c.ctrl.Template(id)
	return tc.Info(true, true)
}

func (c *ONeClient) ListTemplates() ([]tmpl.Template, error) {
	tc := c.ctrl.Templates()
	p, err := tc.Info()
	if err != nil {
		c.log.Named("Templates").Debug("Error while listing Templates", zap.Error(err))
		return nil, err
	}
	return p.Templates, nil
}

func (c *ONeClient) InstantiateTemplateHelper(instance *pb.Instance, ig *pb.InstancesGroup, token string) (vmid int, err error) {
	group_data := ig.GetData()
	resources := instance.GetResources()
	tmpl := vm.NewTemplate()
	data := make(map[string]*structpb.Value)
	conf := instance.GetConfig()

	tmpl.Add(driver_shared.NOCLOUD_VM, "TRUE")
	tmpl.Add(driver_shared.NOCLOUD_VM_TOKEN, token)
	tmpl.Add(driver_shared.NOCLOUD_INST_TITLE, instance.GetTitle())
	tmpl.Add(driver_shared.NOCLOUD_IG_TITLE, instance.GetTitle())

	if pass := conf["password"].GetStringValue(); pass != "" {
		tmpl.Add(keys.Template("PASSWORD"), pass)
		tmpl.AddCtx("PASSWORD", pass)
	}
	if ssh_key := conf["ssh_public_key"].GetStringValue(); ssh_key != "" {
		tmpl.AddCtx(keys.SSHPubKey, ssh_key)
	}

	var template_id int
	if conf["template_id"] != nil {
		template_id = int(conf["template_id"].GetNumberValue())
	} else {
		return 0, errors.New("template ID isn't given")
	}
	vm_tmpl, err := c.GetTemplate(template_id)
	if err != nil {
		return 0, err
	}

	if pair, err := vm_tmpl.Template.GetPair("NOCLOUD_ENABLED"); err == nil && pair.Value == "FALSE" {
		return -1, errors.New("cannot instantiate VM for template disabled by Nocloud")
	}

	if pair, err := vm_tmpl.Template.GetPair("LOGO"); err == nil {
		data[DATA_LOGO] = structpb.NewStringValue(pair.Value)
	}

	id := instance.GetUuid()
	vmname := id
	data[DATA_VM_NAME] = structpb.NewStringValue(vmname)

	// Set VCPU, is 1 by default
	/*vcpu := 1
	if resources["vcpu"] != nil {
		vcpu = int(resources["vcpu"].GetNumberValue())
	}*/

	tmpl.CPU(1)

	// Set CPU, must be provided by instance resources config
	if resources["cpu"] == nil {
		return 0, errors.New("amount of CPU is not given")
	}
	tmpl.VCPU(int(resources["cpu"].GetNumberValue()))

	// Set RAM, must be provided by instance resources config
	if resources["ram"] == nil {
		return 0, errors.New("amount of RAM is not given")
	}
	tmpl.Memory(int(resources["ram"].GetNumberValue()))

	if resources["drive_size"] != nil {
		id := 0
		disks := vm_tmpl.Template.GetDisks()
		if conf["template_disk_id"] != nil {
			id = int(conf["template_disk_id"].GetNumberValue())
		}
		c.log.Debug("Disks", zap.Int("disks_len", len(disks)), zap.Int("id", id))

		for i, disk := range disks {
			new := tmpl.AddDisk()
			for _, pair := range disk.Pairs {
				new.AddPair(pair.Key(), pair.Value)
			}
			if i == id {
				new.Del(string(shared.Size))
				new.Add(shared.Size, int(resources["drive_size"].GetNumberValue()))
				new.Add(driver_shared.DRIVE_TYPE, resources["drive_type"].GetStringValue())
			}
		}
	}

	sched, err := GetVarValue(c.vars[SCHED], "default")
	if err != nil {
		return -1, err
	}
	req := sched.GetStringValue()
	//req = strings.ReplaceAll(req, "\"", "\\\"")
	// Set Host(s) to deploy Instance to
	tmpl.Placement(keys.SchedRequirements, req)

	// Getting datastores types(like SSD, HDD, etc)
	ds_type := "default"
	if resources["drive_type"] != nil {
		ds_type = resources["drive_type"].GetStringValue()
	}
	sched_ds, err := GetVarValue(c.vars[SCHED_DS], ds_type)
	if err != nil {
		return -1, err
	}
	// Getting Datastores scheduler requirements
	ds_req := sched_ds.GetStringValue()
	//ds_req = strings.ReplaceAll(ds_req, "\"", "\\\"")
	// Setting Datastore(s) to deploy Instance to
	tmpl.Placement(keys.SchedDSRequirements, ds_req)

	public_vn := int(group_data["public_vn"].GetNumberValue())
	for i := 0; i < int(resources["ips_public"].GetNumberValue()); i++ {
		nic := tmpl.AddNIC()
		nic.Add(shared.NetworkID, public_vn)
	}

	private_vn := int(group_data["private_vn"].GetNumberValue())
	for i := 0; i < int(resources["ips_private"].GetNumberValue()); i++ {
		nic := tmpl.AddNIC()
		nic.Add(shared.NetworkID, private_vn)
	}
	// OpenNebula won't generate Networking context without this key set to YES
	// so most templates won't generate network interfaces inside the VM
	if int(resources["ips_public"].GetNumberValue())+int(resources["ips_private"].GetNumberValue()) > 0 {
		tmpl.AddCtx(keys.NetworkCtx, "YES")
	}

	tmpl_string := tmpl.String()
	c.log.Debug("Resulting Template", zap.String("template", tmpl_string))
	vmid, err = c.InstantiateTemplate(template_id, vmname, tmpl_string, false)
	if err != nil {
		return -1, err
	}
	data[DATA_VM_ID] = structpb.NewNumberValue(float64(vmid))

	instance.Data = data
	return vmid, nil
}

func (c *ONeClient) InstantiateTemplate(id int, vmname, tmpl string, pending bool) (vmid int, err error) {
	tc := c.ctrl.Template(id)
	return tc.Instantiate(vmname, pending, tmpl, false)
}
