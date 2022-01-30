package one

import (
	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/group"
)

func (c *ONeClient) GetGroup(id int) (*group.Group, error) {
	gc := c.ctrl.Group(id)
	return gc.Info(true)
}