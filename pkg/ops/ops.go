package ops

import (
	"fmt"
	"time"

	ipb "github.com/slntopp/nocloud-proto/instances"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	Key        = "pending_ops"
	AppliedKey = "applied_spec"
	Creating   = "creating"

	CPU  = "cpu"
	RAM  = "ram"
	Disk = "disk_resize"
	IPs  = "ips"
)

type Op struct {
	ID     string
	Name   string
	Field  string
	From   float64
	To     float64
	Status string
}

func SpecDiff(from, to map[string]*structpb.Value) []Op {
	if to == nil {
		return nil
	}
	if from == nil {
		from = map[string]*structpb.Value{}
	}
	var out []Op
	for _, field := range []string{"cpu", "ram", "drive_size", "ips_public", "ips_private"} {
		o := from[field].GetNumberValue()
		n := to[field].GetNumberValue()
		if o == n {
			continue
		}
		name := field
		switch field {
		case "drive_size":
			name = Disk
		case "ips_public", "ips_private":
			name = IPs
		}
		out = append(out, Op{
			ID:     fmt.Sprintf("%s-%d", name, time.Now().UnixNano()),
			Name:   name,
			Field:  field,
			From:   o,
			To:     n,
			Status: "queued",
		})
	}
	return out
}

func Parse(data map[string]*structpb.Value) []Op {
	if data == nil {
		return nil
	}
	var out []Op
	for _, v := range data[Key].GetListValue().GetValues() {
		f := v.GetStructValue().GetFields()
		out = append(out, Op{
			ID:     f["id"].GetStringValue(),
			Name:   f["op"].GetStringValue(),
			Field:  f["field"].GetStringValue(),
			From:   f["from"].GetNumberValue(),
			To:     f["to"].GetNumberValue(),
			Status: f["status"].GetStringValue(),
		})
	}
	return out
}

func Store(data map[string]*structpb.Value, list []Op) {
	if data == nil {
		return
	}
	vals := make([]*structpb.Value, 0, len(list))
	for _, op := range list {
		st, err := structpb.NewStruct(map[string]interface{}{
			"id":     op.ID,
			"op":     op.Name,
			"field":  op.Field,
			"from":   op.From,
			"to":     op.To,
			"status": op.Status,
		})
		if err != nil {
			continue
		}
		vals = append(vals, structpb.NewStructValue(st))
	}
	lv, err := structpb.NewValue(vals)
	if err != nil {
		return
	}
	data[Key] = lv
}

func Append(data map[string]*structpb.Value, extra []Op) {
	if data == nil || len(extra) == 0 {
		return
	}
	cur := Parse(data)
	cur = append(cur, extra...)
	Store(data, cur)
}

func Next(list []Op) *Op {
	for i := range list {
		if list[i].Status == "running" {
			return &list[i]
		}
	}
	for i := range list {
		if list[i].Status == "" || list[i].Status == "queued" {
			return &list[i]
		}
	}
	return nil
}

func AppliedResources(data map[string]*structpb.Value) map[string]*structpb.Value {
	if data == nil {
		return nil
	}
	return data[AppliedKey].GetStructValue().GetFields()
}

func SetApplied(inst *ipb.Instance) {
	if inst.Data == nil {
		inst.Data = map[string]*structpb.Value{}
	}
	st, err := structpb.NewStruct(map[string]interface{}{
		"cpu":         inst.GetResources()["cpu"].GetNumberValue(),
		"ram":         inst.GetResources()["ram"].GetNumberValue(),
		"drive_size":  inst.GetResources()["drive_size"].GetNumberValue(),
		"ips_public":  inst.GetResources()["ips_public"].GetNumberValue(),
		"ips_private": inst.GetResources()["ips_private"].GetNumberValue(),
	})
	if err != nil {
		return
	}
	inst.Data[AppliedKey] = structpb.NewStructValue(st)
}

func EnsureFromSpec(inst *ipb.Instance) {
	if inst.Data == nil {
		inst.Data = map[string]*structpb.Value{}
	}
	if len(Parse(inst.Data)) > 0 {
		return
	}
	applied := AppliedResources(inst.Data)
	if applied == nil {
		return
	}
	if inst.GetHash() != "" && inst.Data["applied_hash"].GetStringValue() == inst.GetHash() {
		return
	}
	Append(inst.Data, SpecDiff(applied, inst.GetResources()))
}

func HasVMID(inst *ipb.Instance) bool {
	return int(inst.GetData()["vmid"].GetNumberValue()) != 0
}

func AutoStart(inst *ipb.Instance) bool {
	return inst.GetConfig()["auto_start"].GetBoolValue() || inst.GetBillingPlan().GetMeta()["auto_start"].GetBoolValue()
}
