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
	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/user"
)

func (c *ONeClient) GetUser(id int) (*user.User, error) {
	uc := c.ctrl.User(id)
	return uc.Info(true)
}

func (c *ONeClient) CreateUser(name, pass string, groups []int) (id int, err error) {
	uc := c.ctrl.Users()
	return uc.Create(name, pass, "core", groups)
}

func (c *ONeClient) DeleteUser(id int) error {
	uc := c.ctrl.User(id)
	return uc.Delete()
}