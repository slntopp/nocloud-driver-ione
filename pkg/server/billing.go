package server

import (
	"regexp"
	"strings"
	"sync"
	"time"

	onevm "github.com/OpenNebula/one/src/oca/go/src/goca/schemas/vm"
	"github.com/slntopp/nocloud-driver-ione/pkg/datas"
	one "github.com/slntopp/nocloud-driver-ione/pkg/driver"
	billingpb "github.com/slntopp/nocloud/pkg/billing/proto"
	ipb "github.com/slntopp/nocloud/pkg/instances/proto"
	stpb "github.com/slntopp/nocloud/pkg/states/proto"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/slntopp/nocloud-driver-ione/pkg/shared"
)

func Lazy[T any](f func() T) func() T {
	var o T
	var once sync.Once
	return func() T {
		once.Do(func() {
			o = f()
			f = nil
		})
		return o
	}
}

type LazyVM func() (*onevm.VM, error)

func GetVM(f func() (*onevm.VM, error)) LazyVM {
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

type LazyTimeline func() []one.Record

type RecordsPublisherFunc func([]*billingpb.Record)

func handleInstanceBilling(logger *zap.Logger, publish RecordsPublisherFunc, client *one.ONeClient, i *ipb.Instance) {
	log := logger.Named("InstanceBillingHandler").Named(i.GetUuid())
	log.Debug("Initializing")

	plan := i.BillingPlan
	if plan == nil {
		log.Warn("Instance has no Billing Plan", zap.Any("instance", i))
		return
	}

	vmid, err := one.GetVMIDFromData(client, i)
	if err != nil {
		log.Error("Failed to get VM ID", zap.Error(err))
		return
	}

	vm := GetVM(func() (*onevm.VM, error) { return client.GetVM(vmid) })
	var created int64
	if _, ok := i.Data[shared.VM_CREATED]; ok {
		created = int64(i.Data[shared.VM_CREATED].GetNumberValue())
	} else {
		obj, err := vm()
		if err != nil {
			log.Error("Error getting VM", zap.Error(err))
			return
		}
		created = int64(obj.STime)
	}

	timeline := Lazy(func() []one.Record {
		o, _ := vm()
		return one.MakeTimeline(o)
	})

	var records []*billingpb.Record

	for _, resource := range plan.Resources {
		var last int64
		if _, ok := i.Data[resource.Key+"_last_monitoring"]; ok {
			last = int64(i.Data[resource.Key+"_last_monitoring"].GetNumberValue())
		} else {
			last = created
		}

		handler, ok := handlers.Get(resource.Key)
		if !ok {
			log.Warn("Handler not found", zap.String("resource", resource.Key))
			continue
		}
		log.Debug("Handling", zap.String("resource", resource.Key), zap.Int64("last", last), zap.Int64("created", created), zap.Any("kind", resource.Kind))
		new, last := handler(log, timeline, i, vm, resource, client, last)

		if len(new) != 0 {
			records = append(records, new...)
			i.Data[resource.Key+"_last_monitoring"] = structpb.NewNumberValue(float64(last))
		}
	}

	if plan.Kind == billingpb.PlanKind_STATIC {
		var last int64
		if _, ok := i.Data["last_monitoring"]; ok {
			last = int64(i.Data["last_monitoring"].GetNumberValue())
		} else {
			last = created
		}

		new, last := handleStaticBilling(log, i, last)
		if len(new) != 0 {
			records = append(records, new...)
			i.Data["last_monitoring"] = structpb.NewNumberValue(float64(last))
		}
	}

	log.Debug("Putting new Records", zap.Any("records", records))
	go publish(records)
	go datas.Pub(&ipb.ObjectData{
		Uuid: i.Uuid,
		Data: i.Data,
	})
}

type BillingHandlerFunc func(
	*zap.Logger,
	LazyTimeline,
	*ipb.Instance,
	LazyVM,
	*billingpb.ResourceConf,
	*one.ONeClient,
	int64,
) ([]*billingpb.Record, int64)

var handlers = BillingMap{
	handlers: map[string]BillingHandlerFunc{
		"cpu": handleCPUBilling,
		"ram": handleRAMBilling,
		"ip":  handleIPBilling,
		// See BillingMap.Get for other handlers
		// e.g. drive_${driveKind}
	},
}

type BillingMap struct {
	handlers map[string]BillingHandlerFunc
}

func (m *BillingMap) Get(key string) (BillingHandlerFunc, bool) {
	if strings.Contains(key, "drive_") {
		return handleDriveBilling, true
	}
	handler, ok := m.handlers[key]
	return handler, ok
}

func resourceKeyToDriveKind(key string) (string, error) {
	r, err := regexp.Compile(`drive.*_([A-Za-z]+)`)
	if err != nil {
		return "", err
	}

	rs := r.FindStringSubmatch(key)
	if len(rs) == 0 {
		return "UNKNOWN", nil
	}

	return strings.ToUpper(string(rs[1])), nil
}

func handleDriveBilling(log *zap.Logger, ltl LazyTimeline, i *ipb.Instance, vm LazyVM, res *billingpb.ResourceConf, c *one.ONeClient, last int64) ([]*billingpb.Record, int64) {

	driveKind, _ := resourceKeyToDriveKind(res.Key)

	o, _ := vm()
	storage := Lazy(func() float64 {
		disks := o.Template.GetDisks()
		total := 0.0
		for _, disk := range disks {
			capacity, _ := disk.GetFloat("SIZE")
			driveType, _ := disk.GetStr("DRIVE_TYPE")

			if driveType == driveKind {
				total += capacity / 1000
			}
		}
		return total
	})

	return handleCapacityBilling(log.Named("DRIVE"), storage, ltl, i, vm, res, c, last)
}

func handleIPBilling(log *zap.Logger, ltl LazyTimeline, i *ipb.Instance, vm LazyVM, res *billingpb.ResourceConf, c *one.ONeClient, last int64) ([]*billingpb.Record, int64) {
	o, _ := vm()
	ip := Lazy(func() float64 {
		publicNetworks := 0.0
		nics := o.Template.GetNICs()
		for _, nic := range nics {
			id, err := nic.GetInt("NETWORK_ID")
			if err != nil {
				log.Warn("Can't get NETWORK_ID from VM template", zap.String("Instance id", i.GetUuid()), zap.Int("VM id", o.ID))
				continue
			}

			vnet, err := c.GetVNet(id)
			if err != nil {
				log.Warn("Can't get vnet by id", zap.String("Instance id", i.GetUuid()), zap.Int("vnet id", id))
				continue
			}

			vnetType, err := vnet.Template.GetStr("TYPE")
			if err != nil {
				log.Warn("Can't get vnet type from vnet attributes", zap.String("Instance id", i.GetUuid()), zap.Int("vnet id", id))
				continue
			}

			if vnetType == "PUBLIC" {
				publicNetworks += 1.0
			}
		}
		return publicNetworks
	})
	return handleCapacityBilling(log.Named("IP"), ip, ltl, i, vm, res, c, last)
}

func handleCPUBilling(log *zap.Logger, ltl LazyTimeline, i *ipb.Instance, vm LazyVM, res *billingpb.ResourceConf, c *one.ONeClient, last int64) ([]*billingpb.Record, int64) {
	o, _ := vm()
	cpu := Lazy(func() float64 {
		cpu, _ := o.Template.GetCPU()
		return cpu
	})
	return handleCapacityBilling(log.Named("CPU"), cpu, ltl, i, vm, res, c, last)
}

func handleRAMBilling(log *zap.Logger, ltl LazyTimeline, i *ipb.Instance, vm LazyVM, res *billingpb.ResourceConf, c *one.ONeClient, last int64) ([]*billingpb.Record, int64) {
	o, _ := vm()
	ram := Lazy(func() float64 {
		ram, _ := o.Template.GetMemory()
		return float64(ram) / 1024
	})
	return handleCapacityBilling(log.Named("RAM"), ram, ltl, i, vm, res, c, last)
}

func handleCapacityBilling(log *zap.Logger, amount func() float64, ltl LazyTimeline, i *ipb.Instance, vm LazyVM, res *billingpb.ResourceConf, c *one.ONeClient, last int64) ([]*billingpb.Record, int64) {
	now := int64(time.Now().Unix())
	timeline := one.FilterTimeline(ltl(), last, now)
	var records []*billingpb.Record

	log.Debug("Handling Capacity Billing", zap.Any("timeline", ltl()), zap.Any("filtered", timeline))

	if res.Kind == billingpb.Kind_POSTPAID {
		on := make(map[stpb.NoCloudState]bool)
		for _, s := range res.On {
			on[s] = true
		}

		for end := last + res.Period; end <= int64(time.Now().Unix()); end += res.Period {
			tl := one.FilterTimeline(timeline, last, end)
			for _, rec := range tl {
				if _, ok := on[rec.State]; ok != res.Except {
					records = append(records, &billingpb.Record{
						Resource: res.Key,
						Instance: i.GetUuid(),
						Start:    rec.Start, End: rec.End,
						Exec:  rec.End,
						Total: float64((rec.End-rec.Start)/res.Period) * res.Price * amount(),
					})
				}
			}
			last = end
		}
	} else {
		for end := last + res.Period; last <= int64(time.Now().Unix()); end += res.Period {
			md := map[string]*structpb.Value{
				"instance_title": structpb.NewStringValue(i.GetTitle()),
			}
			records = append(records, &billingpb.Record{
				Resource: res.Key,
				Instance: i.GetUuid(),
				Start:    last, End: end, Exec: last,
				Total: res.Price,
				Meta:  md,
			})
			last = end
		}
	}

	return records, last
}

func handleStaticBilling(log *zap.Logger, i *ipb.Instance, last int64) ([]*billingpb.Record, int64) {
	log.Debug("Handling Static Billing", zap.Int64("last", last))
	product, ok := i.BillingPlan.Products[*i.Product]
	if !ok {
		log.Warn("Product not found", zap.String("product", *i.Product))
		return nil, last
	}

	var records []*billingpb.Record
	if product.Kind == billingpb.Kind_POSTPAID {
		log.Debug("Handling Postpaid Billing", zap.Any("product", product))
		for end := last + product.Period; end <= time.Now().Unix(); end += product.Period {
			records = append(records, &billingpb.Record{
				Product:  *i.Product,
				Instance: i.GetUuid(),
				Start:    last, End: end, Exec: last,
				Total: product.Price,
			})
		}
	} else {
		end := last + product.Period
		log.Debug("Handling Prepaid Billing", zap.Any("product", product), zap.Int64("end", end), zap.Int64("now", time.Now().Unix()))
		for ; last <= time.Now().Unix(); end += product.Period {
			records = append(records, &billingpb.Record{
				Product:  *i.Product,
				Instance: i.GetUuid(),
				Start:    last, End: end, Exec: last,
				Total: product.Price,
			})
			last = end
		}
	}

	return records, last
}
