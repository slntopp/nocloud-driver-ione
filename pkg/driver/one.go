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
	"time"

	goca "github.com/OpenNebula/one/src/oca/go/src/goca"
	sppb "github.com/slntopp/nocloud/pkg/services_providers/proto"
	stpb "github.com/slntopp/nocloud/pkg/states/proto"
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
	State stpb.NoCloudState `json:"state"`
	Error string `json:"error"`

	Meta map[string]*structpb.Value
}

func (c *ONeClient) MonitorLocation(sp *sppb.ServicesProvider) (res *LocationState, err error) {
	log := c.log.Named("MonitorLocation").Named(sp.GetUuid())

	res = &LocationState{Uuid: sp.GetUuid(), State: stpb.NoCloudState_RUNNING, Meta: make(map[string]*structpb.Value)}
	
	hostsState, err := MonitorHostsPool(log.Named("MonitorHostsPool"), c)
	if err != nil {
		res.State = stpb.NoCloudState_FAILURE
		hostsState, _ = structpb.NewValue(map[string]interface{}{
			"error": err.Error(),
		})
	}
	res.Meta["hosts"] = hostsState

	dssState, err := MonitorDatastoresPool(log.Named("MonitorDatastoresPool"), c)
	if err != nil {
		res.State = stpb.NoCloudState_UNKNOWN
		dssState, _ = structpb.NewValue(map[string]interface{}{
			"error": err.Error(),
		})
	}
	res.Meta["datastores"] = dssState

	netsState, err := MonitorNetworks(log.Named("MonitorNetworks"), c)
	if err != nil {
		res.State = stpb.NoCloudState_UNKNOWN
		netsState, _ = structpb.NewValue(map[string]interface{}{
			"error": err.Error(),
		})
	}
	res.Meta["networking"] = netsState

	res.Meta["ts"] = structpb.NewNumberValue(float64(time.Now().Unix()))
	return res, nil
}
