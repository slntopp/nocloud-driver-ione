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
	"github.com/OpenNebula/one/src/oca/go/src/goca/parameters"
	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/group"
	img "github.com/OpenNebula/one/src/oca/go/src/goca/schemas/image"
	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/shared"
	tmpl "github.com/OpenNebula/one/src/oca/go/src/goca/schemas/template"
	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/user"
	vnet "github.com/OpenNebula/one/src/oca/go/src/goca/schemas/virtualnetwork"
	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/vm"
	instpb "github.com/slntopp/nocloud/pkg/instances/proto"
	sppb "github.com/slntopp/nocloud/pkg/services_providers/proto"
	stpb "github.com/slntopp/nocloud/pkg/states/proto"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/structpb"
)

type IClient interface {
	CheckInstancesGroup(IG *instpb.InstancesGroup) (*CheckInstancesGroupResponse, error)
	CheckInstancesGroupResponseProcess(resp *CheckInstancesGroupResponse, data map[string]*structpb.Value, group int)
	Chmod(class string, oid int, perm *shared.Permissions) error
	Chown(class string, oid, uid, gid int) error
	CreateUser(name, pass string, groups []int) (id int, err error)
	DeleteUser(id int) error
	DeleteUserAndVNets(id int) error
	DeleteVNet(id int) error
	FindFreeVlan(sp *sppb.ServicesProvider) (vnMad string, freeVlan int, err error)
	FindVMByInstance(inst *instpb.Instance) (*vm.VM, error)
	GetGroup(id int) (*group.Group, error)
	GetImage(id int) (*img.Image, error)
	GetInstSnapshots(inst *instpb.Instance) (map[string]interface{}, error)
	GetSecrets() map[string]*structpb.Value
	GetTemplate(id int) (*tmpl.Template, error)
	GetUser(id int) (*user.User, error)
	GetUserPrivateVNet(user int) (id int, err error)
	GetUserPublicVNet(user int) (id int, err error)
	GetUserVMsInstancesGroup(userId int) (*instpb.InstancesGroup, error)
	GetVM(vmid int) (*vm.VM, error)
	GetVMByName(name string) (id int, err error)
	GetVNet(id int) (*vnet.VirtualNetwork, error)
	InstantiateTemplate(id int, vmname, tmpl string, pending bool) (vmid int, err error)
	InstantiateTemplateHelper(instance *instpb.Instance, group_data map[string]*structpb.Value, token string) (vmid int, err error)
	ListImages() ([]img.Image, error)
	ListTemplates() ([]tmpl.Template, error)
	Logger(n string) *zap.Logger
	MonitorLocation(sp *sppb.ServicesProvider) (st *LocationState, pd *LocationPublicData, err error)
	NetworkingVM(id int) (map[string]interface{}, error)
	PoweroffVM(id int, hard bool) error
	RebootVM(id int, hard bool) error
	ReservePrivateIP(u int, vnMad string, vlanID int) (pool_id int, err error)
	ReservePublicIP(u, n int) (pool_id int, err error)
	ReserveVNet(id, size, to int, name string) (int, error)
	ResumeVM(id int) error
	Reinstall(id int) error
	Monitoring(id int) (*vm.Monitoring, error)
	SetSecrets(secrets map[string]*structpb.Value)
	SetVars(vars map[string]*sppb.Var)
	SnapCreate(name string, vmid int) error
	SnapDelete(snapId, vmid int) error
	SnapRevert(snapId, vmid int) error
	StateVM(id int) (state int, state_str string, lcm_state int, lcm_state_str string, err error)
	SuspendVM(id int) error
	TerminateVM(id int, hard bool) error
	UpdateVNet(id int, tmpl string, uType parameters.UpdateType) error
	UserAddAttribute(id int, data map[string]interface{}) error
	VMToInstance(id int) (*instpb.Instance, error)
}

type ONeClient struct {
	*goca.Client
	ctrl *goca.Controller
	log  *zap.Logger

	vars    map[string]*sppb.Var
	secrets map[string]*structpb.Value
}

func NewClient(user, password, endpoint string, log *zap.Logger) *ONeClient {
	conf := goca.NewConfig(user, password, endpoint)
	c := goca.NewClient(conf, nil)
	ctrl := goca.NewController(c)
	return &ONeClient{
		Client: c,
		ctrl:   ctrl,
		log:    log.Named("ONeClient"),
	}
}

func NewClientFromSP(sp *sppb.ServicesProvider, log *zap.Logger) (*ONeClient, error) {
	secrets := sp.GetSecrets()
	host := secrets["host"].GetStringValue()
	user := secrets["user"].GetStringValue()
	pass := secrets["pass"].GetStringValue()
	if host == "" || user == "" || pass == "" {
		return nil, errors.New("host or Credentials are empty")
	}
	c := NewClient(user, pass, host, log)
	c.secrets = secrets
	return c, nil
}

func (c *ONeClient) SetSecrets(secrets map[string]*structpb.Value) {
	c.secrets = secrets
}
func (c *ONeClient) GetSecrets() map[string]*structpb.Value {
	return c.secrets
}
func (c *ONeClient) SetVars(vars map[string]*sppb.Var) {
	c.vars = vars
}
func (c *ONeClient) Logger(n string) *zap.Logger {
	return c.log.Named(n)
}

type LocationState struct {
	Uuid  string            `json:"uuid"`
	State stpb.NoCloudState `json:"state"`
	Error string            `json:"error"`

	Meta map[string]*structpb.Value
}

type LocationPublicData struct {
	Uuid  string `json:"uuid"`
	Error string `json:"error"`

	PublicData map[string]*structpb.Value
}

func (c *ONeClient) MonitorLocation(sp *sppb.ServicesProvider) (st *LocationState, pd *LocationPublicData, err error) {
	log := c.log.Named("MonitorLocation").Named(sp.GetUuid())

	st = &LocationState{Uuid: sp.GetUuid(), State: stpb.NoCloudState_RUNNING, Meta: make(map[string]*structpb.Value)}
	pd = &LocationPublicData{Uuid: sp.GetUuid(), PublicData: make(map[string]*structpb.Value)}

	hostsState, err := MonitorHostsPool(log.Named("MonitorHostsPool"), c)
	if err != nil {
		st.State = stpb.NoCloudState_FAILURE
		hostsState, _ = structpb.NewValue(map[string]interface{}{
			"error": err.Error(),
		})
	}
	st.Meta["hosts"] = hostsState

	dssState, err := MonitorDatastoresPool(log.Named("MonitorDatastoresPool"), c)
	if err != nil {
		st.State = stpb.NoCloudState_UNKNOWN
		dssState, _ = structpb.NewValue(map[string]interface{}{
			"error": err.Error(),
		})
	}
	st.Meta["datastores"] = dssState

	netsState, err := MonitorNetworks(log.Named("MonitorNetworks"), c)
	if err != nil {
		st.State = stpb.NoCloudState_UNKNOWN
		netsState, _ = structpb.NewValue(map[string]interface{}{
			"error": err.Error(),
		})
	}
	st.Meta["networking"] = netsState

	templatesState, err := MonitorTemplates(log.Named("MonitorTemplates"), c)
	if err != nil {
		templatesState, _ = structpb.NewValue(map[string]interface{}{
			"error": err.Error(),
		})
	}
	pd.PublicData["templates"] = templatesState

	st.Meta["ts"] = structpb.NewNumberValue(float64(time.Now().Unix()))
	return st, pd, nil
}
