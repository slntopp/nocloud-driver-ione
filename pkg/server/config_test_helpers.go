package server

import (
	"fmt"
	"strings"

	one "github.com/slntopp/nocloud-driver-ione/pkg/driver"
	ipb "github.com/slntopp/nocloud-proto/instances"
	"go.uber.org/zap"

	pb "github.com/slntopp/nocloud-proto/services_providers"
)

func EnsureSPLimits(log *zap.Logger, instance *ipb.Instance, sp *pb.ServicesProvider) error {
	log.Debug("Running bounds check")
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
	if ok {
		min, err := one.GetVarValue(minVar, driveType)
		if err == nil && size.GetNumberValue() < min.GetNumberValue() {
			return fmt.Errorf("requested Drive Size is smaller than ServicesProvider Min limit(%.f)", min.GetNumberValue())
		}
		log.Warn("Couldn't get limits(min) config", zap.String("drive_type", driveType), zap.String("sp", sp.GetUuid()))
	}

	maxVar, ok := sp.GetVars()[one.MAX_DRIVE_SIZE]
	if ok {
		max, err := one.GetVarValue(maxVar, driveType)
		if err == nil && size.GetNumberValue() > max.GetNumberValue() {
			return fmt.Errorf("requested Drive Size is larger than ServicesProvider Min limit(%.f)", max.GetNumberValue())
		}
		log.Warn("Couldn't get limits(max) config", zap.String("drive_type", driveType), zap.String("sp", sp.GetUuid()))
	}

	return nil
}
