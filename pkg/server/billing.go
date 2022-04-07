package server

import (
	"sync"
	"time"

	onevm "github.com/OpenNebula/one/src/oca/go/src/goca/schemas/vm"
	"github.com/slntopp/nocloud-driver-ione/pkg/actions"
	one "github.com/slntopp/nocloud-driver-ione/pkg/driver"
	billingpb "github.com/slntopp/nocloud/pkg/billing/proto"
	instpb "github.com/slntopp/nocloud/pkg/instances/proto"
	stpb "github.com/slntopp/nocloud/pkg/states/proto"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/slntopp/nocloud-driver-ione/pkg/shared"
)

func Lazy[T comparable](f func()(T)) (func()(T)) {
	var o T
	var once sync.Once
	return func() (T) {
		once.Do(func() {
			o = f()
			f = nil
		})
		return o
	}
}

type LazyVM func() (*onevm.VM, error)
func GetVM(f func() (*onevm.VM, error) ) LazyVM {
	var o *onevm.VM
	var err error
	var once sync.Once
	return func() (*onevm.VM, error) {
		once.Do(func() {
			o, err = f()
			f = nil
		})
		return o, err
	}
}

type LazyTimeline func() ([]one.Record)
func MakeTimeline(f func() ([]one.Record)) LazyTimeline {
	var o []one.Record
	var once sync.Once
	return func() ([]one.Record) {
		once.Do(func() {
			o = f()
			f = nil
		})
		return o
	}
}

func handleInstanceBilling(logger *zap.Logger, client *one.ONeClient, i *instpb.Instance) {
	log := logger.Named("InstanceBillingHandler").Named(i.GetUuid())
	log.Debug("Initiazing")

	plan := i.BillingPlan
	if plan == nil {
		log.Warn("Instance has no Billing Plan", zap.Any("instance", i))
		return
	}

	vmid, err := actions.GetVMIDFromData(client, i)
	if err != nil {
		log.Error("Failed to get VM ID", zap.Error(err))
	}

	vm := GetVM(func() (*onevm.VM, error) { return client.GetVM(vmid) })
	var created uint64
	if _, ok := i.Data[shared.VM_CREATED]; ok {
		created = uint64(i.Data[shared.VM_CREATED].GetNumberValue())
	} else {
		obj, err := vm()
		if err != nil {
			log.Error("Error getting VM", zap.Error(err))
			return
		}
		created = uint64(obj.STime)
	}
	
	timeline := MakeTimeline(func() ([]one.Record) {
		o, _ := vm()
		return one.MakeTimeline(o)
	})

	var records []*billingpb.Record
	for _, resource := range plan.Resources {
		var last uint64
		if _, ok := i.Data[resource.Key + "_last_monitoring"]; ok {
			last = uint64(i.Data[resource.Key + "_last_monitoring"].GetNumberValue())
		} else {
			last = created
		}

		handler, ok := handlers[resource.Key]
		if !ok {
			log.Warn("Handler not found", zap.String("resource", resource.Key))
		}
		log.Debug("Handling", zap.String("resource", resource.Key), zap.Uint64("last", last), zap.Uint64("created", created), zap.Any("kind", resource.Kind))
		new, last := handler(timeline, i, vm, resource, last)

		records = append(records, new...)
		i.Data[resource.Key + "_last_monitoring"] = structpb.NewNumberValue(float64(last))
	}

	log.Info("Putting new Records", zap.Any("records", records))
}

type BillingHandlerFunc func(
	LazyTimeline,
	*instpb.Instance,
	LazyVM,
	*billingpb.ResourceConf,
	uint64,
) ([]*billingpb.Record, uint64)

var handlers = map[string]BillingHandlerFunc {
	"cpu": handleCPUBilling,
	"ram": handleRAMBilling,
}

func handleCPUBilling(ltl LazyTimeline, i *instpb.Instance, vm LazyVM, res *billingpb.ResourceConf, last uint64) ([]*billingpb.Record, uint64) {
	o, _ := vm()
	cpu := Lazy(func () float64 {
		cpu, _ := o.Template.GetCPU()
		return cpu
	})
	return handleCapacityBilling(cpu, ltl, i, vm, res, last)
}

func handleRAMBilling(ltl LazyTimeline, i *instpb.Instance, vm LazyVM, res *billingpb.ResourceConf, last uint64) ([]*billingpb.Record, uint64) {
	o, _ := vm()
	ram := Lazy(func () float64 {
		ram, _ := o.Template.GetMemory()
		return float64(ram)
	})
	return handleCapacityBilling(ram, ltl, i, vm, res, last)
}

func handleCapacityBilling(amount func()(float64), ltl LazyTimeline, i *instpb.Instance, vm LazyVM, res *billingpb.ResourceConf, last uint64) ([]*billingpb.Record, uint64) {
	now := uint64(time.Now().Unix())
	timeline := one.FilterTimeline(ltl(), last, now)
	var records []*billingpb.Record

	if res.Kind == billingpb.Kind_POSTPAID {
		on := make(map[stpb.NoCloudState]bool)
		for _, s := range res.On {
			on[s] = true
		}

		for end := last + res.Period; end <= uint64(time.Now().Unix()); end += res.Period {
			tl := one.FilterTimeline(timeline, last, end)
			for _, rec := range tl {
				if _, ok := on[rec.State]; ok != res.Except {
					records = append(records, &billingpb.Record{
						Resource: res.Key,
						Instance: i.GetUuid(),
						Start: rec.Start, End: rec.End,
						Exec: rec.End,
						Total: float64((rec.End - rec.Start) / res.Period) * res.Price * amount(),
					})
				}
			}
			last = end
		}
	} else {
		for end := last + res.Period; end <= uint64(time.Now().Unix()); end += res.Period {
			records = append(records, &billingpb.Record{
				Resource: res.Key,
				Instance: i.GetUuid(),
				Start: last, End: end, Exec: last,
				Total: res.Price,
			})
			last = end
		}
	}

	return records, last
}