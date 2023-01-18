package server

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"sync"
	"time"

	oneshared "github.com/OpenNebula/one/src/oca/go/src/goca/schemas/shared"
	onevm "github.com/OpenNebula/one/src/oca/go/src/goca/schemas/vm"
	"github.com/slntopp/nocloud-driver-ione/pkg/datas"
	one "github.com/slntopp/nocloud-driver-ione/pkg/driver"
	"github.com/slntopp/nocloud-driver-ione/pkg/utils"
	billingpb "github.com/slntopp/nocloud-proto/billing"
	ipb "github.com/slntopp/nocloud-proto/instances"
	stpb "github.com/slntopp/nocloud-proto/states"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/slntopp/nocloud-driver-ione/pkg/shared"
)

var clock utils.IClock = &utils.Clock{}

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

func handleInstanceBilling(logger *zap.Logger, publish RecordsPublisherFunc, client one.IClient, i *ipb.Instance, status ipb.InstanceStatus) {
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

	_, state, _, _, err := client.StateVM(vmid)
	if err != nil {
		log.Warn("Could not get state for VM ID", zap.Int("vmid", vmid))
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

	var productRecords, resourceRecords []*billingpb.Record

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
		new, last := handler(log, timeline, i, vm, resource, client, last, clock)

		if len(new) != 0 {
			if plan.Kind == billingpb.PlanKind_DYNAMIC {
				instState := stpb.NoCloudState_INIT
				if i.State != nil {
					instState = i.State.State
				}
				inStates := false

				for _, val := range resource.On {
					if val == instState {
						inStates = true
						break
					}
				}

				if inStates || (!inStates && resource.Except) {
					resourceRecords = append(resourceRecords, new...)
				}
			} else {
				resourceRecords = append(resourceRecords, new...)
			}
			i.Data[resource.Key+"_last_monitoring"] = structpb.NewNumberValue(float64(last))
		}
	}

	if plan.Kind == billingpb.PlanKind_STATIC {
		var last int64
		var priority billingpb.Priority
		if _, ok := i.Data["last_monitoring"]; ok {
			last = int64(i.Data["last_monitoring"].GetNumberValue())
			priority = billingpb.Priority_NORMAL
		} else {
			last = created
			priority = billingpb.Priority_URGENT
		}
		new, last := handleStaticBilling(log, i, last, priority)
		if len(new) != 0 {
			productRecords = append(productRecords, new...)
			i.Data["last_monitoring"] = structpb.NewNumberValue(float64(last))
		}
	}

	publisher := datas.DataPublisher(datas.POST_INST_DATA)

	log.Debug("Putting new Records", zap.Any("productRecords", productRecords), zap.Any("resourceRecords", resourceRecords))

	if status == ipb.InstanceStatus_SUS {
		_, isStatic := i.Data["last_monitoring"]
		if (len(productRecords) != 0 || (len(productRecords) == 0 && len(resourceRecords) != 0 && !isStatic)) && state != "SUSPENDED" {
			if err := client.SuspendVM(vmid); err != nil {
				log.Warn("Could not suspend VM with VMID", zap.Int("vmid", vmid))
			}
		}

		if state == "SUSPENDED" {
			if _, ok := i.Data["last_monitoring"]; ok {
				now := time.Now().Unix()
				nowPb := structpb.NewNumberValue(float64(now))
				i.Data["last_monitoring"] = nowPb
			}
		}
	} else {
		if state == "SUSPENDED" {
			if err := client.ResumeVM(vmid); err != nil {
				log.Warn("Could not resume VM with VMID", zap.Int("vmid", vmid))
			}
		}
	}

	go publish(productRecords)
	go publish(resourceRecords)
	go publisher(i.Uuid, i.Data)
}

type BillingHandlerFunc func(
	*zap.Logger,
	LazyTimeline,
	*ipb.Instance,
	LazyVM,
	*billingpb.ResourceConf,
	one.IClient,
	int64,
	utils.IClock,
) ([]*billingpb.Record, int64)

var handlers = BillingMap{
	handlers: map[string]BillingHandlerFunc{
		"cpu":        handleCPUBilling,
		"ram":        handleRAMBilling,
		"ips_public": handleIPBilling,
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

func handleDriveBilling(log *zap.Logger, ltl LazyTimeline, i *ipb.Instance, vm LazyVM, res *billingpb.ResourceConf, c one.IClient, last int64, clock utils.IClock) ([]*billingpb.Record, int64) {

	driveKind, _ := resourceKeyToDriveKind(res.Key)

	o, _ := vm()
	storage := Lazy(func() float64 {
		disks := o.Template.GetDisks()
		total := 0.0
		for _, disk := range disks {
			capacity, _ := disk.GetFloat("SIZE")
			driveType, _ := disk.GetStr("DRIVE_TYPE")

			if driveType == driveKind {
				total += capacity / 1024
			}
		}
		return total
	})

	return handleCapacityBilling(log.Named("DRIVE"), storage, ltl, i, res, last, clock)
}

func handleIPBilling(log *zap.Logger, ltl LazyTimeline, i *ipb.Instance, vm LazyVM, res *billingpb.ResourceConf, c one.IClient, last int64, clock utils.IClock) ([]*billingpb.Record, int64) {
	o, _ := vm()
	ip := Lazy(func() float64 {
		publicNetworks := 0.0
		nics := o.Template.GetNICs()
		for _, nic := range nics {
			id, err := nic.GetInt(string(oneshared.NetworkID))
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
	return handleCapacityBilling(log.Named("IP"), ip, ltl, i, res, last, clock)
}

func handleCPUBilling(log *zap.Logger, ltl LazyTimeline, i *ipb.Instance, vm LazyVM, res *billingpb.ResourceConf, c one.IClient, last int64, clock utils.IClock) ([]*billingpb.Record, int64) {
	o, _ := vm()
	cpu := Lazy(func() float64 {
		cpu, _ := o.Template.GetVCPU()
		return float64(cpu)
	})
	return handleCapacityBilling(log.Named("CPU"), cpu, ltl, i, res, last, clock)
}

func handleRAMBilling(log *zap.Logger, ltl LazyTimeline, i *ipb.Instance, vm LazyVM, res *billingpb.ResourceConf, c one.IClient, last int64, clock utils.IClock) ([]*billingpb.Record, int64) {
	o, _ := vm()
	ram := Lazy(func() float64 {
		ram, _ := o.Template.GetMemory()
		return float64(ram) / 1024
	})
	return handleCapacityBilling(log.Named("RAM"), ram, ltl, i, res, last, clock)
}

func handleCapacityBilling(log *zap.Logger, amount func() float64, ltl LazyTimeline, i *ipb.Instance, res *billingpb.ResourceConf, last int64, time utils.IClock) ([]*billingpb.Record, int64) {
	now := time.Now().Unix()
	timeline := one.FilterTimeline(ltl(), last, now)
	var records []*billingpb.Record

	if res.Kind == billingpb.Kind_POSTPAID {
		on := make(map[stpb.NoCloudState]bool)
		for _, s := range res.On {
			on[s] = true
		}

		for end := last + res.Period; end <= time.Now().Unix(); end += res.Period {
			tl := one.FilterTimeline(timeline, last, end)
			for _, rec := range tl {
				if _, ok := on[rec.State]; ok != res.Except {
					records = append(records, &billingpb.Record{
						Resource: res.Key,
						Instance: i.GetUuid(),
						Start:    rec.Start, End: rec.End,
						Exec:  rec.End,
						Total: math.Round(float64((rec.End-rec.Start)/res.Period)*res.Price*amount()*100) / 100.0,
					})
				}
			}
			last = end
		}
	} else {
		for end := last + res.Period; last <= time.Now().Unix(); end += res.Period {
			md := map[string]*structpb.Value{
				"instance_title": structpb.NewStringValue(i.GetTitle()),
			}
			records = append(records, &billingpb.Record{
				Resource: res.Key,
				Instance: i.GetUuid(),
				Priority: billingpb.Priority_URGENT,
				Start:    last, End: end, Exec: last,
				Total: math.Round(res.Price*amount()*100) / 100.0,
				Meta:  md,
			})
			last = end
		}
	}

	return records, last
}

func handleStaticBilling(log *zap.Logger, i *ipb.Instance, last int64, priority billingpb.Priority) ([]*billingpb.Record, int64) {
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
				Priority: billingpb.Priority_NORMAL,
				Total:    math.Round(product.Price*100) / 100.0,
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
				Priority: priority,
				Total:    math.Round(product.Price*100) / 100.0,
			})
			last = end
		}
	}

	return records, last
}

func handleUpgradeBilling(log *zap.Logger, instances []*ipb.Instance, c *one.ONeClient, publish RecordsPublisherFunc) {
	now := time.Now().Unix()

	var records []*billingpb.Record

	publisher := datas.DataPublisher(datas.POST_INST_DATA)

	for _, inst := range instances {
		plan := inst.GetBillingPlan()
		if plan == nil {
			break
		}
		resources := plan.GetResources()

		diffSlice := c.GetVmResourcesDiff(inst)

		for _, diff := range diffSlice {
			for _, res := range resources {
				if diff.ResName == res.GetKey() {
					log.Info("Billing res", zap.String("res", diff.ResName), zap.Int("diff", diff.OldResCount))
					instData := inst.GetData()
					if instData == nil {
						log.Info("Data is empty", zap.String("uuid", inst.GetUuid()))
						continue
					}

					key := fmt.Sprintf("%s_last_monitoring", diff.ResName)

					lastMonitoring, ok := instData[key]
					if !ok {
						log.Info("No param ins data", zap.String("uuid", inst.GetUuid()), zap.String("param", key))
						continue
					}

					timeDiff := now - int64(lastMonitoring.GetNumberValue())

					if timeDiff < 0 {
						timeDiff += res.GetPeriod()
					}

					total := res.Price * (float64(timeDiff) / float64(res.GetPeriod())) * float64(diff.OldResCount)
					total = math.Round(total*100) / 100.0

					log.Debug("Check diff time", zap.Int64("diff", timeDiff), zap.Float64("total", total))

					records = append(records, &billingpb.Record{
						Start: int64(lastMonitoring.GetNumberValue()), End: int64(lastMonitoring.GetNumberValue()) + timeDiff, Exec: now,
						Priority: 1,
						Instance: inst.GetUuid(),
						Resource: diff.ResName,
						Total:    total,
					})

					updateKey := fmt.Sprintf("%s_update_time", diff.ResName)
					inst.Data[updateKey] = structpb.NewNumberValue(float64(now))
				}
			}
		}
		go publisher(inst.GetUuid(), inst.GetData())
	}

	go publish(records)
}
