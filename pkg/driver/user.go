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