/*
Copyright © 2021 Nikita Ivanovski info@slnt-opp.xyz

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
package ione

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"encoding/base64"
	"encoding/json"
	"encoding/xml"

	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/group"
	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/shared"
	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/user"
	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/vm"
	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/vm/keys"
	"go.uber.org/zap"

	"github.com/gofrs/uuid"
	instpb "github.com/slntopp/nocloud/pkg/instances/proto"
	sppb "github.com/slntopp/nocloud/pkg/services_providers/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

type IONe struct {
	Host string
	Credentials string
	Authorization string

	Vars map[string]*sppb.Var

	Client http.Client

	log *zap.Logger
}

type IONeRequest struct {
	OID int64 `json:"oid,omitempty"`
	Params []interface{} `json:"params"`
}

type IONeResponse struct {
	Response interface{} `json:"response"`
	Error string `json:"error"`
}

func NewIONeClient(host, cred string, vars map[string]*sppb.Var, log *zap.Logger) (*IONe) {
	auth := fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(cred)))
	return &IONe{host, cred, auth, vars, http.Client{}, log.Named("IONe")}
}

func (ione *IONe) Invoke(method string, req IONeRequest) (r *IONeResponse, err error) {
	body, _ := json.Marshal(req)
	reqBody := bytes.NewBuffer(body)

	ione.log.Debug("Sending request", zap.String("host", ione.Host), zap.String("method", method), zap.Any("request", req))
	url := fmt.Sprintf("%s/%s", ione.Host, method)
	request, err := http.NewRequest("POST", url, reqBody)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", ione.Authorization)

	res, err := ione.Client.Do(request)
	if err != nil {
		return nil, err
	}

	body, err = ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if string(body) == "False Credentials given" || string(body) == "No Credentials given"{
		return nil, errors.New(string(body))
	}

	err = json.Unmarshal(body, &r)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (ione *IONe) Call(method string, params ...interface{}) (r *IONeResponse, err error) {
	return ione.Invoke(method, IONeRequest{Params: params})
}

func (ione *IONe) ONeCall(method string, oid int64, params ...interface{}) (r *IONeResponse, err error) {
	return ione.Invoke(method, IONeRequest{OID: oid, Params: params})
}

func (ione *IONe) Ping() (bool, error) {
	r, err := ione.Call("ione/Test", "PING")
	if err != nil {
		return false, err
	}
	return r.Response.(string) == "PONG", nil
}

func (ione *IONe) UserCreate(login, passwd string, group int64) (int64, error) {
	r, err := ione.Call("ione/UserCreate", login, passwd, group)
	if err != nil {
		return -1, err
	}
	return int64(r.Response.(float64)), nil
}

func (ione *IONe) UserDelete(id int64) (error) {
	r, err := ione.Call("ione/Delete", id)
	if err != nil {
		return err
	}
	if r.Error != "" {
		return errors.New(r.Error)
	}
	return nil
}

type VNet struct {
	Id string
	Type string
}

func (ione *IONe) GetUserNetworks() (res []VNet, err error) {
	r, err := ione.Call("ione/get_user_vnets")
	if err != nil {
		return nil, err
	}
	if r.Error != "" {
		return nil, errors.New(r.Error)
	}
	pool := r.Response.([]interface{})

	for _, vnet_obj := range pool {
		ivnet := vnet_obj.(map[string]interface{})["VNET"].(map[string]interface{})
		vnet := VNet{Id: ivnet["ID"].(string)}

		vnet_tmpl := ivnet["TEMPLATE"].(map[string]interface{})
		vnet.Type = vnet_tmpl["TYPE"].(string)
		
		res = append(res, vnet)
	}
	return res, nil
}

func (ione *IONe) ReservePublicIP(user, amount float64) (vn float64, err error) {
	r, err := ione.Call(
		"ione/reserve_public_ip",
		map[string]float64{
			"u": user, "n": amount,
		},
	)
	if err != nil {
		return -1, err
	}
	if r.Error != "" {
		return -1, errors.New(r.Error)
	}

	return r.Response.(float64), nil
}

func (ione *IONe) TemplateInstantiate(instance *instpb.Instance, group_data map[string]*structpb.Value) (error) {
	resources := instance.GetResources()
	tmpl := vm.NewTemplate()
	data := make(map[string]*structpb.Value)

	id, err := uuid.NewV4()
	if err != nil {
		return errors.New("Couldn't generate UUID")
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
		return errors.New("Amount of CPU is not given")
	}
	tmpl.CPU(resources["cpu"].GetNumberValue())

	// Set RAM, must be provided by instance resources config
	if resources["ram"] == nil {
		return errors.New("Amount of RAM is not given")
	}
	tmpl.Memory(int(resources["ram"].GetNumberValue()))

	req := ione.Vars["sched"].GetValue()["default"].GetStringValue()
	req = strings.ReplaceAll(req, "\"", "\\\"")
	// Set Host(s) to deploy Instance to
	tmpl.Placement(keys.SchedRequirements, req)

	// Getting datastores types(like SSD, HDD, etc)
	datastores := ione.Vars["sched_ds"].GetValue()
	ds_type := "default"
	if resources["drive_type"] != nil {
		ds_type = resources["drive_type"].GetStringValue()
	}
	// Getting Datastores scheduler requirements
	ds_req := datastores["default"].GetStringValue()
	if datastores[ds_type] != nil {
		ds_req = datastores[ds_type].GetStringValue()
	}
	ds_req = strings.ReplaceAll(ds_req, "\"", "\\\"")
	// Setting Datastore(s) to deploy Instance to
	tmpl.Placement(keys.SchedDSRequirements, ds_req)

	public_vn := int(group_data["public_vn"].GetNumberValue())
	for i := 0; i < int(resources["ips_public"].GetNumberValue()); i++ {
		nic := tmpl.AddNIC()
		nic.Add(shared.NetworkID, public_vn)
	}

	conf := instance.GetConfig()
	var template_id int64
	if conf["template_id"] != nil {
		template_id = int64(conf["template_id"].GetNumberValue())
	} else {
		return errors.New("Template ID isn't given")
	}

	r, err := ione.ONeCall("one.t.instantiate", template_id, vmname, false, tmpl.String())
	if err != nil {
		return err
	}

	switch r.Response.(type) {
	case map[string]interface{}:
		return errors.New(r.Response.(map[string]interface{})["error"].(string))
	}
	vm_id := r.Response.(float64)
	data["vmid"] = structpb.NewNumberValue(vm_id)

	instance.Data = data
	return nil
}

func (ione *IONe) GetUser(id int64) (res *user.UserShort, err error) {
	r, err := ione.ONeCall(
		"one.u.to_xml!", id)
	if err != nil {
		return nil, err
	}
	if r.Error != "" {
		return nil, errors.New(r.Error)
	}

	switch r.Response.(type) {
	case string:
		xmlData := r.Response.(string)
		xml.Unmarshal([]byte(xmlData), &res)
		return res, nil
	case map[string]interface{}:
		r := r.Response.(map[string]interface{})
		return nil, errors.New(r["error"].(string))
	}
	return nil, errors.New("Unexpected error while getting user")
}

func (ione *IONe) GetGroup(id int64) (res *group.GroupShort, err error) {
	r, err := ione.ONeCall(
		"one.g.to_xml!", id)
	if err != nil {
		return nil, err
	}
	if r.Error != "" {
		return nil, errors.New(r.Error)
	}

	switch r.Response.(type) {
	case string:
		xmlData := r.Response.(string)
		xml.Unmarshal([]byte(xmlData), &res)
		return res, nil
	case map[string]interface{}:
		r := r.Response.(map[string]interface{})
		return nil, errors.New(r["error"].(string))
	}
	return nil, errors.New("Unexpected error while getting group")
}

func (ione *IONe) TerminateVM(id int64, hard bool) (error) {
	r, err := ione.ONeCall("one.vm.terminate", id, hard)
	if err != nil {
		return err
	}
	if r.Error != "" {
		return errors.New(r.Error)
	}
	return nil
}