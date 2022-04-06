package server

import (
	"sync"
	"time"

	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/vm"
	"github.com/slntopp/nocloud-driver-ione/pkg/actions"
	one "github.com/slntopp/nocloud-driver-ione/pkg/driver"
	billingpb "github.com/slntopp/nocloud/pkg/billing/proto"
	instpb "github.com/slntopp/nocloud/pkg/instances/proto"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/slntopp/nocloud-driver-ione/pkg/shared"
)

type LazyVM func() (*vm.VM, error)
func GetVM(f func() (*vm.VM, error) ) LazyVM {
	var o *vm.VM
	var err error
	var once sync.Once
	return func() (*vm.VM, error) {
		once.Do(func() {
			o, err = f()
			f = nil
		})
		return o, err
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

	vm := GetVM(func() (*vm.VM, error) { return client.GetVM(vmid) })
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
		new, last := handler(log, i, vm, resource, last)

		records = append(records, new...)
		i.Data[resource.Key + "_last_monitoring"] = structpb.NewNumberValue(float64(last))
	}

	log.Info("Putting new Records", zap.Any("records", records))
}

type BillingHandlerFunc func(
	*zap.Logger,
	*instpb.Instance,
	LazyVM,
	*billingpb.ResourceConf,
	uint64,
) ([]*billingpb.Record, uint64)

var handlers = map[string]BillingHandlerFunc {
	"cpu": handleCPUBilling,
}

func handleCPUBilling(logger *zap.Logger, i *instpb.Instance, vm LazyVM, res *billingpb.ResourceConf, last uint64) ([]*billingpb.Record, uint64) {
	var records []*billingpb.Record

	if res.Kind == billingpb.Kind_POSTPAID {
		for end := last + res.Period; end <= uint64(time.Now().Unix()); end += res.Period {
			records = append(records, &billingpb.Record{
				Resource: res.Key,
				Instance: i.GetUuid(),
				Start: last, End: end, Exec: end,
				Total: res.Price,
			})
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