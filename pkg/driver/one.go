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
	"strconv"

	goca "github.com/OpenNebula/one/src/oca/go/src/goca"
	"github.com/OpenNebula/one/src/oca/go/src/goca/dynamic"
	instpb "github.com/slntopp/nocloud/pkg/instances/proto"
	sppb "github.com/slntopp/nocloud/pkg/services_providers/proto"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/structpb"
)

type ONeClient struct {
	*goca.Client
	ctrl *goca.Controller
	log *zap.Logger

	vars map[string]*sppb.Var
	secrets map[string]*structpb.Value
}

func NewClient(user, password, endpoint string, log *zap.Logger) *ONeClient {
	conf := goca.NewConfig(user, password, endpoint)
	c    := goca.NewClient(conf, nil)
	ctrl := goca.NewController(c)
	return &ONeClient{
		Client: c,
		ctrl: ctrl,
		log: log.Named("ONeClient"),
	}
}

func NewClientFromSP(sp *sppb.ServicesProvider, log *zap.Logger) (*ONeClient, error) {
	secrets := sp.GetSecrets()
	host  := secrets["host"].GetStringValue()
	user  := secrets["user"].GetStringValue()
	pass  := secrets["pass"].GetStringValue()
	if host == "" || user == "" || pass == "" {
		return nil, errors.New("host or Credentials are empty")
	}
	return NewClient(user, pass, host, log), nil
}

func (c *ONeClient) SetSecrets(secrets map[string]*structpb.Value) {
	c.secrets = secrets
}
func (c *ONeClient) SetVars(vars map[string]*sppb.Var) {
	c.vars = vars
}

type LocationState struct {
	Uuid string `json:"uuid"`
	State instpb.NoCloudState `json:"state"`
	Hosts map[string]*structpb.Value `json:"hosts"`
}

func (c *ONeClient) MonitorLocation(sp *sppb.ServicesProvider) (res *LocationState, err error) {
	hsc := c.ctrl.Hosts()
	hosts, err := hsc.Info()
	if err != nil {
		return nil, err
	}
	res = &LocationState{Uuid: sp.GetUuid(), State: instpb.NoCloudState_RUNNING}
	res.Hosts = make(map[string]*structpb.Value)
	for _, host := range hosts.Hosts {
		hc := c.ctrl.Host(host.ID)
		state := make(map[string]interface{})
		s, err := host.StateString()
		if err != nil {
			s = "UNKNOWN"
			res.State = instpb.NoCloudState_UNKNOWN
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
			c.log.Error("Error getting Monitoring data", zap.Error(err))
			goto done
		}
		recLen = len(mon.Records)
		if recLen == 0 {
			goto done
		}
		rec = mon.Records[recLen - 1]
		c.log.Debug("Last Monitoring Record", zap.Any("record", rec))
		helper(state, rec)

		done:
		hs_struct, err := structpb.NewValue(state)
		if err != nil {
			c.log.Error("Error converting HostState to StructPB", zap.Any("state", state), zap.Error(err))
		}
		res.Hosts[strconv.Itoa(host.ID)] = hs_struct
	}

	return res, nil
}



