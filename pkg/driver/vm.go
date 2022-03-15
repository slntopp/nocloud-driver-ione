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
	pb "github.com/slntopp/nocloud/pkg/instances/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

func (c *ONeClient) GetVMByName(name string) (id int, err error) {
	vmsc := c.ctrl.VMs()
	return vmsc.ByName(name)
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

func (c *ONeClient) VMToInstance(id int) (*pb.Instance, error) {
	vmc := c.ctrl.VM(id)
	vm, err := vmc.Info(true)
	if err != nil {
		return nil, err
	}
	inst := pb.Instance{
		Config: make(map[string]*structpb.Value),
		Resources: make(map[string]*structpb.Value),
	}
	
	tmpl := vm.Template
	{
		tid, err := tmpl.GetFloat("TEMPLATE_ID")
		if err != nil {
			return nil, err
		}
		inst.Config["template_id"] = structpb.NewNumberValue(tid)
	}
	{
		pwd, err := vm.UserTemplate.GetStr("PASSWORD")
		if err != nil {
			return nil, err
		}
		inst.Config["password"] = structpb.NewStringValue(pwd)
	}
	{
		cpu, err := tmpl.GetCPU()
		if err != nil {
			return nil, err
		}
		inst.Resources["cpu"] = structpb.NewNumberValue(cpu)
	}
	{
		ram, err := tmpl.GetMemory()
		if err != nil {
			return nil, err
		}
		inst.Resources["ram"] = structpb.NewNumberValue(float64(ram))
	}

	return &inst, nil
}