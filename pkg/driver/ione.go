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

	"encoding/base64"
	"encoding/json"

	"github.com/OpenNebula/goca"
	instpb "github.com/slntopp/nocloud/pkg/instances/proto"
	sppb "github.com/slntopp/nocloud/pkg/services_providers/proto"
)

type IONe struct {
	Host string
	Credentials string
	Authorization string

	Vars map[string]*sppb.Var

	Client http.Client
}

type IONeRequest struct {
	Method string
	Params []interface{} `json:"params"`
}

type IONeResponse struct {
	Response interface{} `json:"response"`
	Error string `json:"error"`
}

func NewIONeClient(host, cred string, vars map[string]*sppb.Var) (*IONe) {
	auth := fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(cred)))
	return &IONe{host, cred, auth, vars, http.Client{}}
}

func (ione *IONe) Invoke(method string, req IONeRequest) (r *IONeResponse, err error) {
	body, _ := json.Marshal(req)
	reqBody := bytes.NewBuffer(body)

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

func (ione *IONe) ONeCall(method string, oid int, params ...interface{}) (r *IONeResponse, err error) {
	return ione.Invoke(method, IONeRequest{OID: oid, Params: params})
}

func (ione *IONe) Ping() bool {
	r, err := ione.Call("ione/Test","PING")
	if err != nil {
		return false
	}
	return r.Response.(string) == "PONG"
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

	resources := instance.GetResources()
	tmpl := goca.NewTemplateBuilder()

	if resources["vcpu"] == nil {
		tmpl.AddValue("vcpu", 1)
	} else {
		tmpl.AddValue("vcpu", resources["vcpu"].GetNumberValue())
	}

	if resources["cpu"] == nil {
		return 0, errors.New("Amount of CPU is not given")
	}
	tmpl.AddValue("cpu", resources["cpu"].GetNumberValue())

	if resources["ram"] == nil {
		return 0, errors.New("Amount of RAM is not given")
	}
	tmpl.AddValue("ram", resources["ram"].GetNumberValue())

	tmpl.AddValue("SCHED_REQUIREMENTS", ione.Vars["sched"].GetValue()["default"])
	
	datastores := ione.Vars["sched_ds"].GetValue()
	ds_type := "default"
	if resources["drive_type"] != nil {
		ds_type = resources["drive_type"].GetStringValue()
	}
	
	if datastores[ds_type] != nil {
		tmpl.AddValue("SCHED_DS_REQUIREMENTS", datastores[ds_type].GetStringValue())
	} else {
		tmpl.AddValue("SCHED_DS_REQUIREMENTS", datastores["default"].GetStringValue())
	}

	return 0, nil
}