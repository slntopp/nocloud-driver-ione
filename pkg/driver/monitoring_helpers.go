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

import "github.com/OpenNebula/one/src/oca/go/src/goca/dynamic"

var VMMsRecordHelpers = map[string]func(state map[string]interface{}, rec dynamic.Template){
	"vcenter": vCenterRecordHelper,
}

func vCenterRecordHelper(state map[string]interface{}, rec dynamic.Template) {
	capacity, err := rec.GetVector("CAPACITY")
	if err == nil {
		freeCpu, err := capacity.GetInt("FREE_CPU")
		if err == nil {
			state["free_cpu"] = freeCpu
		}
		freeRam, err := capacity.GetInt("FREE_MEMORY")
		if err == nil {
			state["free_ram"] = freeRam
		}
		usedCpu, err := capacity.GetInt("USED_CPU")
		if err == nil {
			state["total_cpu"] = freeCpu + usedCpu
		}
		usedRam, err := capacity.GetInt("USED_MEMORY")
		if err == nil {
			state["total_ram"] = freeRam + usedRam
		}
	}
	ts, err := rec.GetInt("TIMESTAMP")
	if err == nil {
		state["ts"] = ts
	}
}