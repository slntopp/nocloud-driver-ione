package server

import (
	one "github.com/slntopp/nocloud-driver-ione/pkg/driver"
	ipb "github.com/slntopp/nocloud-proto/instances"
	"go.uber.org/zap"
)

func suspendMonitoring(log *zap.Logger, client *one.ONeClient, inst *ipb.Instance, status ipb.InstanceStatus) {
	vmid, err := one.GetVMIDFromData(client, inst)
	if err != nil {
		log.Warn("Failed to obtain VM ID", zap.String("instance", inst.Uuid), zap.Error(err))
	}

	_, state, _, _, err := client.StateVM(vmid)
	if err != nil {
		log.Warn("Could not get state for VM ID", zap.Int("vmid", vmid))
	}

	if status != ipb.InstanceStatus_SUS && state == "SUSPENDED" {
		if err := client.ResumeVM(vmid); err != nil {
			log.Warn("Could not resume VM with VMID", zap.Int("vmid", vmid))
		}
	}

	if status == ipb.InstanceStatus_SUS && state != "SUSPENDED" {
		if err := client.SuspendVM(vmid); err != nil {
			log.Warn("Could not suspend VM with VMID", zap.Int("vmid", vmid))
		}
	}
}
