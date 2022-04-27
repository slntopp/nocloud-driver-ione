package one

import (
	img "github.com/OpenNebula/one/src/oca/go/src/goca/schemas/image"
)

func (c *ONeClient) GetImage(id int) (*img.Image, error) {
	ic := c.ctrl.Image(id)
	return ic.Info(true)
}

func (c *ONeClient) ListImages() ([]img.Image, error) {
	ic := c.ctrl.Images()
	p, err := ic.Info()
	if err != nil {
		return nil, err
	}
	return p.Images, nil
}
