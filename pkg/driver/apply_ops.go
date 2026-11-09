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

// ApplyPendingOps runs at most one queued op. done=true means the op finished this tick.
func (c *ONeClient) ApplyPendingOps(ig *ipb.InstancesGroup, inst *ipb.Instance) (committed *ops.Op, err error) {
	if inst.Data == nil {
		return nil, nil
	}
	ops.EnsureFromSpec(inst)
	list := ops.Parse(inst.Data)
	cur := ops.Next(list)
	if cur == nil {
		return nil, nil
	}
	cur.Status = "running"
	vmid, err := GetVMIDFromData(c, inst)
	if err != nil {
		return nil, err
	}
	done, err := c.applyOp(ig, inst, vmid, *cur)
	if err != nil {
		c.log.Error("apply op", zap.String("op", cur.Name), zap.Error(err))
		cur.Status = "queued"
		ops.Store(inst.Data, list)
		return nil, err
	}
	if !done {
		ops.Store(inst.Data, list)
		return nil, nil
	}
	cur.Status = "done"
	left := make([]ops.Op, 0, len(list))
	for _, op := range list {
		if op.ID == cur.ID {
			continue
		}
		left = append(left, op)
	}
	ops.Store(inst.Data, left)
	ops.SetApplied(inst)
	inst.Data["applied_hash"] = structpb.NewStringValue(inst.GetHash())
	return cur, nil
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
	if op.Name == ops.CPU || inst.Resources["cpu"].GetNumberValue() > 0 {
		tmpl.VCPU(int(inst.Resources["cpu"].GetNumberValue()))
	}
	if op.Name == ops.RAM || inst.Resources["ram"].GetNumberValue() > 0 {
		tmpl.Memory(int(inst.Resources["ram"].GetNumberValue()))
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
	err := vmc.Disk(0).Resize(strconv.Itoa(int(inst.Resources["drive_size"].GetNumberValue())))
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
