package one

import (
	"errors"
	"strings"

	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/shared"
	tmpl "github.com/OpenNebula/one/src/oca/go/src/goca/schemas/template"
	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/vm"
	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/vm/keys"
	"github.com/gofrs/uuid"
	instpb "github.com/slntopp/nocloud/pkg/instances/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

func (c *ONeClient) GetTemplate(id int) (*tmpl.Template, error) {
	tc := c.ctrl.Template(id)
	return tc.Info(true, true)
}

func (c *ONeClient) InstantiateTemplateHelper(instance *instpb.Instance, group_data map[string]*structpb.Value) (vmid int, err error) {
	resources := instance.GetResources()
	tmpl := vm.NewTemplate()
	data := make(map[string]*structpb.Value)

	id, err := uuid.NewV4()
	if err != nil {
		return 0, errors.New("couldn't generate UUID")
	}
	vmname := id.String()
	data["vm_name"] = structpb.NewStringValue(vmname)

	// Set VCPU, is 1 by default
	vcpu := 1
	if resources["vcpu"] != nil {
		vcpu = int(resources["vcpu"].GetNumberValue())
	}
	tmpl.VCPU(vcpu)

	// Set CPU, must be provided by instance resources config
	if resources["cpu"] == nil {
		return 0, errors.New("amount of CPU is not given")
	}
	tmpl.CPU(resources["cpu"].GetNumberValue())

	// Set RAM, must be provided by instance resources config
	if resources["ram"] == nil {
		return 0, errors.New("amount of RAM is not given")
	}
	tmpl.Memory(int(resources["ram"].GetNumberValue()))

	sched, err := GetVarValue(c.vars[SCHED], "default")
	if err != nil {
		return -1, err
	}
	req := sched.GetStringValue()
	req = strings.ReplaceAll(req, "\"", "\\\"")
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
	ds_req = strings.ReplaceAll(ds_req, "\"", "\\\"")
	// Setting Datastore(s) to deploy Instance to
	tmpl.Placement(keys.SchedDSRequirements, ds_req)

	public_vn := int(group_data["public_vn"].GetNumberValue())
	for i := 0; i < int(resources["ips_public"].GetNumberValue()); i++ {
		nic := tmpl.AddNIC()
		nic.Add(shared.NetworkID, public_vn)
	}

	conf := instance.GetConfig()

	tmpl.Add(keys.Template("PASSWORD"), conf["password"].GetStringValue())

	var template_id int
	if conf["template_id"] != nil {
		template_id = int(conf["template_id"].GetNumberValue())
	} else {
		return 0, errors.New("template ID isn't given")
	}

	vmid, err = c.InstantiateTemplate(template_id, vmname, tmpl.String(), false)
	if err != nil {
		return -1, err
	}
	data["vmid"] = structpb.NewNumberValue(float64(vmid))

	instance.Data = data
	return vmid, nil
}

func (c *ONeClient) InstantiateTemplate(id int, vmname, tmpl string, pending bool) (vmid int, err error ){
	tc := c.ctrl.Template(id)
	return tc.Instantiate(vmname, pending, tmpl, false)
}