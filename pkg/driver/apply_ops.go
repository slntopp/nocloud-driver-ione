package one

import (
	"strconv"
	"strings"

	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/shared"
	vm "github.com/OpenNebula/one/src/oca/go/src/goca/schemas/vm"
	"github.com/slntopp/nocloud-driver-ione/pkg/ops"
	ipb "github.com/slntopp/nocloud-proto/instances"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/structpb"
)

// ApplyPendingOps tries every queued op in this tick. A failed op stays queued and does not block the rest.
func (c *ONeClient) ApplyPendingOps(ig *ipb.InstancesGroup, inst *ipb.Instance) (committed []ops.Op, err error) {
	if inst.Data == nil {
		return nil, nil
	}
	ops.EnsureFromSpec(inst)
	list := ops.Parse(inst.Data)
	if ops.Next(list) == nil {
		return nil, nil
	}
	vmid, err := GetVMIDFromData(c, inst)
	if err != nil {
		return nil, err
	}
	left := make([]ops.Op, 0, len(list))
	var firstErr error
	for i := range list {
		cur := list[i]
		if cur.Status != "" && cur.Status != "queued" && cur.Status != "running" {
			left = append(left, cur)
			continue
		}
		cur.Status = "running"
		done, aerr := c.applyOp(ig, inst, vmid, cur)
		if aerr != nil {
			c.log.Error("apply op", zap.String("op", cur.Name), zap.String("id", cur.ID), zap.Error(aerr))
			cur.Status = "queued"
			left = append(left, cur)
			if firstErr == nil {
				firstErr = aerr
			}
			continue
		}
		if !done {
			left = append(left, cur)
			continue
		}
		committed = append(committed, cur)
	}
	ops.Store(inst.Data, left)
	if len(committed) > 0 {
		ops.SetApplied(inst)
		inst.Data["applied_hash"] = structpb.NewStringValue(inst.GetHash())
	}
	return committed, firstErr
}

func (c *ONeClient) applyOp(ig *ipb.InstancesGroup, inst *ipb.Instance, vmid int, op ops.Op) (bool, error) {
	switch op.Name {
	case ops.CPU, ops.RAM:
		return c.applyCompute(vmid, inst, op)
	case ops.Disk:
		return c.applyDisk(vmid, inst)
	case ops.IPs:
		return c.applyIPs(ig, inst, vmid, op)
	default:
		return true, nil
	}
}

func (c *ONeClient) applyCompute(vmid int, inst *ipb.Instance, op ops.Op) (bool, error) {
	vmc := c.ctrl.VM(vmid)
	VM, err := vmc.Info(true)
	if err != nil {
		return false, err
	}
	st, lcm, err := VM.State()
	if err != nil {
		return false, err
	}
	tmpl := vm.NewTemplate()
	wantCPU := int(inst.Resources["cpu"].GetNumberValue())
	wantRAM := int(inst.Resources["ram"].GetNumberValue())
	if op.Name == ops.CPU || wantCPU > 0 {
		tmpl.VCPU(wantCPU)
	}
	if op.Name == ops.RAM || wantRAM > 0 {
		tmpl.Memory(wantRAM)
	}
	haveCPU, cpuErr := VM.Template.GetVCPU()
	haveRAM, ramErr := VM.Template.GetMemory()
	cpuDone := op.Name == ops.CPU && cpuErr == nil && haveCPU == wantCPU
	ramDone := op.Name == ops.RAM && ramErr == nil && haveRAM == wantRAM
	if cpuDone || ramDone {
		c.log.Info("compute already matches, skipping resize",
			zap.String("op", op.Name), zap.Int("have_cpu", haveCPU), zap.Int("want_cpu", wantCPU),
			zap.Int("have_ram", haveRAM), zap.Int("want_ram", wantRAM))
		return true, nil
	}
	if lcm == vm.Running {
		if err := vmc.Poweroff(); err != nil {
			return false, err
		}
		return false, nil
	}
	if st != vm.Poweroff {
		return false, nil
	}
	if err := vmc.Resize(tmpl.String(), true); err != nil {
		return false, err
	}
	_ = c.ResumeVM(vmid)
	return true, nil
}

func (c *ONeClient) applyDisk(vmid int, inst *ipb.Instance) (bool, error) {
	vmc := c.ctrl.VM(vmid)
	want := int(inst.Resources["drive_size"].GetNumberValue())
	if VM, err := vmc.Info(true); err == nil {
		if disk, derr := VM.Template.GetVector("DISK"); derr == nil {
			if have, serr := disk.GetInt("SIZE"); serr == nil && have >= want {
				c.log.Info("disk already at least requested size, skipping resize",
					zap.Int("have", have), zap.Int("want", want), zap.Int("vmid", vmid))
				return true, nil
			}
		}
	}
	err := vmc.Disk(0).Resize(strconv.Itoa(want))
	if err != nil && strings.Contains(err.Error(), "greater than current") {
		c.log.Info("disk resize refused because size is already applied", zap.Int("want", want), zap.Int("vmid", vmid))
		return true, nil
	}
	return err == nil, err
}

func (c *ONeClient) applyIPs(ig *ipb.InstancesGroup, inst *ipb.Instance, vmid int, op ops.Op) (bool, error) {
	data := ig.GetData()
	vmc := c.ctrl.VM(vmid)
	VM, err := vmc.Info(true)
	if err != nil {
		return false, err
	}
	wantPub := int(inst.Resources["ips_public"].GetNumberValue())
	wantPriv := int(inst.Resources["ips_private"].GetNumberValue())
	havePub, havePriv := 0, 0
	nics := VM.Template.GetNICs()
	for _, nic := range nics {
		net, _ := nic.GetStr("NETWORK")
		if strings.Contains(net, "pub-vnet") {
			havePub++
		} else {
			havePriv++
		}
	}
	if op.Field == "ips_public" && havePub < wantPub {
		uid := int(data["userid"].GetNumberValue())
		if _, err := c.ReservePublicIP(uid, 1); err != nil {
			return false, err
		}
		networkTemplate := vm.NewTemplate()
		nic := networkTemplate.AddNIC()
		nic.Add(shared.NetworkID, int(data["public_vn"].GetNumberValue()))
		if err := vmc.AttachNIC(networkTemplate.String()); err != nil {
			return false, err
		}
		return havePub+1 >= wantPub, nil
	}
	if op.Field == "ips_public" && havePub > wantPub {
		for i := len(nics) - 1; i > 0; i-- {
			net, _ := nics[i].GetStr("NETWORK")
			if !strings.Contains(net, "pub-vnet") {
				continue
			}
			nicId, _ := nics[i].GetInt("NIC_ID")
			if err := vmc.DetachNIC(nicId); err != nil {
				return false, err
			}
			return havePub-1 <= wantPub, nil
		}
	}
	if op.Field == "ips_private" && havePriv < wantPriv {
		networkTemplate := vm.NewTemplate()
		nic := networkTemplate.AddNIC()
		nic.Add(shared.NetworkID, int(data["private_vn"].GetNumberValue()))
		if err := vmc.AttachNIC(networkTemplate.String()); err != nil {
			return false, err
		}
		return havePriv+1 >= wantPriv, nil
	}
	return true, nil
}
