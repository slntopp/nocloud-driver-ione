package server

import (
	"errors"
	"strings"

	one "github.com/slntopp/nocloud-driver-ione/pkg/driver"
	"github.com/slntopp/nocloud/pkg/instances/proto"
	"go.uber.org/zap"

	pb "github.com/slntopp/nocloud/pkg/services_providers/proto"
)

func EnsureSPBounds(log *zap.Logger, instance *proto.Instance, sp *pb.ServicesProvider) error {
	log.Info("Running bounds check")
	resources := instance.GetResources()
	size, ok := resources["drive_size"]
	if !ok {
		return nil
	}
	drive, ok := resources["drive_type"]
	if !ok {
		return nil
	}
	driveType := strings.ToLower(drive.String())

	minVar, ok := sp.GetVars()[one.MIN_DRIVE_SIZE]
	if !ok {
		return nil
	}
	maxVar, ok := sp.GetVars()[one.MAX_DRIVE_SIZE]
	if !ok {
		return nil
	}

	min, err := one.GetVarValue(minVar, driveType)
	if err != nil {
		return nil
	}
	max, err := one.GetVarValue(maxVar, driveType)
	if err != nil {
		return nil
	}

	if size.GetNumberValue() < min.GetNumberValue() {
		log.Warn("requested drive size is smaller than sp min limit")
		return errors.New("requested drive size is smaller than sp min limit")
	}

	if size.GetNumberValue() > max.GetNumberValue() {
		log.Warn("requested drive size is larger than sp max limit")
		return errors.New("requested drive size is larger than sp max limit")
	}

	return nil
}
