package one

import (
	"errors"
	"strings"
	tmpl "github.com/OpenNebula/one/src/oca/go/src/goca/schemas/template"
)

func (c *ONeClient) GetTemplate(id int) (*tmpl.Template, error) {
	tc := c.ctrl.Template(id)
	return tc.Info(true, true)
}

func (c *ONeClient) InstantiateTemplate(id int, vmname, tmpl string, pending bool) (vmid int, err error ){
	tc := c.ctrl.Template(id)
	return tc.Instantiate(vmname, pending, tmpl, false)
}