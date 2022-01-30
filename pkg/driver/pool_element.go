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
func (c *ONeClient) Chmod(class string, oid, perm *shared.Permissions) error {
	args := append([]interface{}{oid}, perm.ToArgs()...)
	_, err := c.Client.Call(fmt.Sprintf("one.%s.chmod", class), args...)
	return err
}