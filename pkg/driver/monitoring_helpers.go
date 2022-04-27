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
	"strconv"

	"github.com/OpenNebula/one/src/oca/go/src/goca/dynamic"
	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/host"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/structpb"
)

var VMMsRecordHelpers = map[string]func(state map[string]interface{}, rec dynamic.Template, host host.Host){
	"vcenter": vCenterRecordHelper,
}

func vCenterRecordHelper(state map[string]interface{}, rec dynamic.Template, host host.Host) {
	share := host.Share

	state["total_cpu"] = share.TotalCPU
	state["used_cpu"] = share.CPUUsage
	state["free_cpu"] = share.TotalCPU - share.CPUUsage

	state["total_ram"] = share.TotalMem
	state["used_ram"] = share.MemUsage
	state["free_ram"] = share.TotalMem - share.MemUsage

	ts, err := rec.GetInt("TIMESTAMP")
	if err == nil {
		state["ts"] = ts
	}
}

func MonitorHostsPool(log *zap.Logger, c *ONeClient) (res *structpb.Value, err error) {
	hsc := c.ctrl.Hosts()
	pool, err := hsc.Info()
	if err != nil {
		return nil, err
	}

	hosts := make(map[string]interface{})
	for _, host := range pool.Hosts {
		hc := c.ctrl.Host(host.ID)
		state := make(map[string]interface{})
		state["name"] = host.Name

		s, err := host.StateString()
		if err != nil {
			s = "UNKNOWN"
		}
		state["state"] = s

		state["im_mad"] = host.IMMAD
		state["vm_mad"] = host.VMMAD

		var rec dynamic.Template
		var recLen int
		mon, err := hc.Monitoring()
		helper, ok := VMMsRecordHelpers[host.VMMAD]
		if !ok {
			state["error"] = fmt.Sprintf("Host VM MAD %s unsupported", host.VMMAD)
			goto done
		}
		if err != nil {
			log.Error("Error getting Monitoring data", zap.Error(err))
			goto done
		}
		recLen = len(mon.Records)
		if recLen == 0 {
			goto done
		}
		rec = mon.Records[recLen-1]
		helper(state, rec, host)

	done:
		hosts[strconv.Itoa(host.ID)] = state
	}
	return structpb.NewValue(hosts)
}

func MonitorDatastoresPool(log *zap.Logger, c *ONeClient) (res *structpb.Value, err error) {
	dsc := c.ctrl.Datastores()
	pool, err := dsc.Info()
	if err != nil {
		return nil, err
	}
	dss := make(map[string]interface{})
	for _, ds := range pool.Datastores {
		if ds.Type != "1" {
			continue
		}
		state := make(map[string]interface{})
		state["name"] = ds.Name

		s, err := ds.StateString()
		if err != nil {
			s = "UNKNOWN"
		}
		state["state"] = s

		state["ds_mad"] = ds.DSMad
		state["tm_mad"] = ds.TMMad

		state["used"] = ds.UsedMB
		state["free"] = ds.FreeMB
		state["total"] = ds.TotalMB

		driveType, _ := ds.Template.GetStr("DRIVE_TYPE")
		state["drive_type"] = driveType

		dss[strconv.Itoa(ds.ID)] = state
	}
	return structpb.NewValue(dss)
}

func MonitorNetworks(log *zap.Logger, c *ONeClient) (res *structpb.Value, err error) {
	state := make(map[string]interface{})

	state["public_vnet"] = func() (state map[string]interface{}) {
		state = map[string]interface{}{}
		public_pool_id, ok := c.vars[PUBLIC_IP_POOL]
		if !ok {
			state["error"] = "VNet ID is not set"
			return state
		}

		id, err := GetVarValue(public_pool_id, "default")
		if err != nil {
			state["error"] = err.Error()
			return state
		}
		vnet, err := c.GetVNet(int(id.GetNumberValue()))
		if err != nil {
			state["error"] = err.Error()
			return state
		}

		state["id"] = vnet.ID
		state["name"] = vnet.Name
		state["vn_mad"] = vnet.VNMad
		total, used := 0, 0
		for _, ar := range vnet.ARs {
			total += ar.Size
			used += len(ar.Leases)
		}
		state["total"] = total
		state["used"] = used
		state["free"] = total - used
		log.Debug("public_vnet", zap.Any("state", state))
		return state
	}()

	state["private_vnet"] = map[string]interface{}{"error": "Private VNet Pool Monitoring not implemented"}

	return structpb.NewValue(state)
}

func MonitorTemplates(log *zap.Logger, c *ONeClient) (res *structpb.Value, err error) {
	pool, err := c.ListTemplates()
	if err != nil {
		return nil, err
	}

	templates := make(map[string]interface{})
	for _, tmpl := range pool {

		state := make(map[string]interface{})

		state["name"] = tmpl.Name

		desc, _ := tmpl.Template.GetStr("DESCRIPTION")
		state["desc"] = desc

		disk := tmpl.Template.GetDisks()[0]
		img_id, err := disk.GetInt("IMAGE_ID")

		img, err := c.GetImage(img_id)

		state["min_size"] = img.Size

		templates[strconv.Itoa(tmpl.ID)] = state
	}

	return structpb.NewValue(templates)
}
