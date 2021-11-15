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
)

type IONe struct {
	Host string
	Credentials string
	Authorization string

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

func NewIONeClient(host, cred string) (*IONe) {
	auth := fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(cred)))
	return &IONe{host, cred, auth, http.Client{}}
}

func (ione *IONe) Call(req IONeRequest) (r *IONeResponse, err error) {
	body, _ := json.Marshal(req)
	reqBody := bytes.NewBuffer(body)

	url := fmt.Sprintf("%s/%s", ione.Host, req.Method)
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


func (ione *IONe) Ping() bool {
	r, err := ione.Call(IONeRequest{
		Method: "ione/Test",
		Params: []interface{}{"PING"},
	})
	if err != nil {
		return false
	}
	return r.Response.(string) == "PONG"
}

func (ione *IONe) UserCreate(login, passwd string, group int64) (int64, error) {
	r, err := ione.Call(IONeRequest{
		Method: "ione/UserCreate",
		Params: []interface{}{
			login, passwd, group,
		},
	})
	if err != nil {
		return -1, err
	}
	return int64(r.Response.(float64)), nil
}