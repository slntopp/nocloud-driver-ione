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

	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/shared"
)

// Chown common method
// 	class - shorthand for entity type, possible values:
// 		vm - VirtualMachine,
//		vn - VirtualNetwork,
//		template - VM Template,
//		image - Image,
//		datastore - DataStore
func (c *ONeClient) Chown(class string, oid, uid, gid int) error {
	_, err := c.Client.Call(fmt.Sprintf("one.%s.chown", class), oid, uid, gid)
	return err
}

// Chmod common method
// 	class - shorthand for entity type, possible values:
// 		vm - VirtualMachine,
//		vn - VirtualNetwork,
//		template - VM Template,
//		image - Image,
//		datastore - DataStore
func (c *ONeClient) Chmod(class string, oid int, perm *shared.Permissions) error {
	args := append([]interface{}{oid}, perm.ToArgs()...)
	_, err := c.Client.Call(fmt.Sprintf("one.%s.chmod", class), args...)
	return err
}