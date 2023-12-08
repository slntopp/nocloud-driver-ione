package server

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strings"
	"sync"
	"time"

	epb "github.com/slntopp/nocloud-proto/events"

	oneshared "github.com/OpenNebula/one/src/oca/go/src/goca/schemas/shared"
	onevm "github.com/OpenNebula/one/src/oca/go/src/goca/schemas/vm"
	"github.com/slntopp/nocloud-driver-ione/pkg/datas"
	one "github.com/slntopp/nocloud-driver-ione/pkg/driver"
	"github.com/slntopp/nocloud-driver-ione/pkg/utils"
	billingpb "github.com/slntopp/nocloud-proto/billing"
	ipb "github.com/slntopp/nocloud-proto/instances"
	stpb "github.com/slntopp/nocloud-proto/states"
	statuspb "github.com/slntopp/nocloud-proto/statuses"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/slntopp/nocloud-driver-ione/pkg/shared"
)

var clock utils.IClock = &utils.Clock{}

type ExpiryDiff struct {
	Timestamp int64
	Days      int64
}

var notificationsPeriods = []ExpiryDiff{
	{0, 0},
	{86400, 1},
	{172800, 2},
	{259200, 3},
	{604800, 7},
	{1296000, 15},
	{2592000, 30},
}

var suspendNotificationsPeriods = []ExpiryDiff{
	{604800, 7},
	{259200, 3},
	{172800, 2},
	{86400, 1},
}

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

type RecordsPublisherFunc func(context.Context, []*billingpb.Record)

type EventsPublisherFunc func(context.Context, *epb.Event)

func handleNonRegularInstanceBilling(logger *zap.Logger, records RecordsPublisherFunc, events EventsPublisherFunc, client *one.ONeClient, i *ipb.Instance, status statuspb.NoCloudStatus) {
	log := logger.Named("NonRegularInstanceBillingHandler").Named(i.GetUuid())
	if i.GetStatus() == statuspb.NoCloudStatus_DEL {
		log.Debug("Instance was deleted. No billing")
		return
	}
	log.Debug("Initializing")

	data := i.GetData()
	if data == nil {
		log.Debug("Instance has no data")
		return
	}

	if lastMonitoring, ok := data["last_monitoring"]; ok {
		now := time.Now().Unix()
		lastMonitoringValue := int64(lastMonitoring.GetNumberValue())

		vmid, err := one.GetVMIDFromData(client, i)
		if err != nil {
			log.Error("Failed to get VM ID", zap.Error(err))
			return
		}

		_, state, _, _, err := client.StateVM(vmid)
		if err != nil {
			log.Warn("Could not get state for VM ID", zap.Int("vmid", vmid))
		}

		suspendedManually := data["suspended_manually"].GetBoolValue()

		if now > lastMonitoringValue && state != "SUSPENDED" {
			err := client.SuspendVM(vmid)
			if err != nil {
				log.Error("Failed to suspend vm", zap.Error(err))
				return
			}
		} else if now <= lastMonitoringValue && state == "SUSPENDED" && !suspendedManually {
			err := client.ResumeVM(vmid)
			if err != nil {
				log.Error("Failed to resume vm", zap.Error(err))
				return
			}
		}

	} else {
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

		var productRecords, resourceRecords []*billingpb.Record

		for _, resource := range plan.Resources {
			var last int64
			_, ok := i.Data[resource.Key+"_last_monitoring"]

			if ok {
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

			if resource.GetPeriod() == 0 {
				if !ok {
					resourceRecords = append(resourceRecords, new...)
				}
			} else {
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

				if resource.GetKind() == billingpb.Kind_POSTPAID {
					i.Data[resource.Key+"_next_payment_date"] = structpb.NewNumberValue(float64(last + resource.GetPeriod()))
				} else {
					i.Data[resource.Key+"_next_payment_date"] = structpb.NewNumberValue(float64(last))
				}
			}
		}

		if plan.Kind == billingpb.PlanKind_STATIC {
			var last int64
			var priority billingpb.Priority
			_, ok := i.Data["last_monitoring"]

			if ok {
				last = int64(i.Data["last_monitoring"].GetNumberValue())
				priority = billingpb.Priority_NORMAL
			} else {
				last = created
				priority = billingpb.Priority_URGENT
			}

			if i.BillingPlan.Products[*i.Product].GetPeriod() == 0 {
				if !ok {
					new, last := handleStaticZeroBilling(log, i, last, priority)
					productRecords = append(productRecords, new...)
					i.Data["last_monitoring"] = structpb.NewNumberValue(float64(last))
				}
			} else {
				new, last := handleStaticBilling(log, i, last, priority)

				if len(new) != 0 {
					productRecords = append(productRecords, new...)
					i.Data["last_monitoring"] = structpb.NewNumberValue(float64(last))
				}

				product := i.GetBillingPlan().GetProducts()[i.GetProduct()]
				if product.GetKind() == billingpb.Kind_POSTPAID {
					i.Data["next_payment_date"] = structpb.NewNumberValue(float64(last + product.GetPeriod()))
				} else {
					i.Data["next_payment_date"] = structpb.NewNumberValue(float64(last))
				}
			}

			go records(context.Background(), append(resourceRecords, productRecords...))
			go events(context.Background(), &epb.Event{
				Uuid: i.GetUuid(),
				Key:  "instance_renew",
				Data: map[string]*structpb.Value{},
			})
			go datas.DataPublisher(datas.POST_INST_DATA)(i.Uuid, i.Data)
		}
	}
}

func handleInstanceBilling(logger *zap.Logger, records RecordsPublisherFunc, events EventsPublisherFunc, client one.IClient, i *ipb.Instance, status statuspb.NoCloudStatus) {
	log := logger.Named("InstanceBillingHandler").Named(i.GetUuid())

	if i.GetStatus() == statuspb.NoCloudStatus_DEL {
		log.Debug("Instance was deleted. No billing")
		return
	}

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
		_, ok := i.Data[resource.Key+"_last_monitoring"]

		if ok {
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

		if resource.GetPeriod() == 0 {
			if !ok {
				resourceRecords = append(resourceRecords, new...)
			}
		} else {
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

			if resource.GetKind() == billingpb.Kind_POSTPAID {
				i.Data[resource.Key+"_next_payment_date"] = structpb.NewNumberValue(float64(last + resource.GetPeriod()))
			} else {
				i.Data[resource.Key+"_next_payment_date"] = structpb.NewNumberValue(float64(last))
			}
		}
	}

	nextPaymentDate := i.Data["next_payment_date"]
	isOnePayment := false

	log.Debug("Next payment", zap.Any("p", nextPaymentDate))

	if plan.Kind == billingpb.PlanKind_STATIC {
		var last int64
		var priority billingpb.Priority
		_, ok := i.Data["last_monitoring"]

		if ok {
			last = int64(i.Data["last_monitoring"].GetNumberValue())
			priority = billingpb.Priority_NORMAL
		} else {
			last = created
			priority = billingpb.Priority_URGENT
		}

		if i.BillingPlan.Products[*i.Product].GetPeriod() == 0 {
			isOnePayment = true
			if !ok {
				new, last := handleStaticZeroBilling(log, i, last, priority)
				productRecords = append(productRecords, new...)
				i.Data["last_monitoring"] = structpb.NewNumberValue(float64(last))
			}
		} else {
			new, last := handleStaticBilling(log, i, last, priority)

			if len(new) != 0 {
				productRecords = append(productRecords, new...)
				i.Data["last_monitoring"] = structpb.NewNumberValue(float64(last))
			}

			product := i.GetBillingPlan().GetProducts()[i.GetProduct()]
			if product.GetKind() == billingpb.Kind_POSTPAID {
				i.Data["next_payment_date"] = structpb.NewNumberValue(float64(last + product.GetPeriod()))
			} else {
				i.Data["next_payment_date"] = structpb.NewNumberValue(float64(last))
			}
		}
	}

	if !isOnePayment {

		log.Debug("Putting new Records", zap.Any("productRecords", productRecords), zap.Any("resourceRecords", resourceRecords))
		_, isStatic := i.Data["last_monitoring"]
		if status == statuspb.NoCloudStatus_SUS && i.GetStatus() != statuspb.NoCloudStatus_DEL {
			if (len(productRecords) != 0 || (len(productRecords) == 0 && len(resourceRecords) != 0 && !isStatic)) && state != "SUSPENDED" {
				if err := client.SuspendVM(vmid); err != nil {
					log.Warn("Could not suspend VM with VMID", zap.Int("vmid", vmid))
				}

				suspendTime := structpb.NewNumberValue(float64(time.Now().Unix()))
				i.Data["suspend_time"] = suspendTime

				go events(context.Background(), &epb.Event{
					Uuid: i.GetUuid(),
					Key:  "instance_suspended",
					Data: map[string]*structpb.Value{},
				})
			}

			if state == "SUSPENDED" {
				if _, ok := i.Data["last_monitoring"]; ok {
					now := time.Now().Unix()
					nowPb := structpb.NewNumberValue(float64(now))
					i.Data["last_monitoring"] = nowPb
					i.Data["next_payment_date"] = nextPaymentDate
				}
			}
		} else {
			if state == "SUSPENDED" && !i.GetData()["suspended_manually"].GetBoolValue() {
				if err := client.ResumeVM(vmid); err != nil {
					log.Warn("Could not resume VM with VMID", zap.Int("vmid", vmid))
				}

				delete(i.Data, "suspend_time")

				go events(context.Background(), &epb.Event{
					Uuid: i.GetUuid(),
					Key:  "instance_unsuspended",
					Data: map[string]*structpb.Value{},
				})
			}
		}

		if status == statuspb.NoCloudStatus_DETACHED {
			now := time.Now().Unix()
			nowPb := structpb.NewNumberValue(float64(now))
			i.Data["last_monitoring"] = nowPb
		}

		log.Debug("Next payment", zap.Any("p", i.Data["next_payment_date"]))

		if state == "SUSPENDED" && !i.GetData()["suspended_manually"].GetBoolValue() {
			handleSuspendEvent(i, events)
		} else {
			handleBillingEvent(i, events)
		}

		canceled_renew, ok := i.Data["canceled_renew"]

		firstCondition := ok && canceled_renew.GetBoolValue() && isStatic && len(productRecords) != 0
		secondCondition := ok && canceled_renew.GetBoolValue() && !isStatic
		thirdCondition := ok && status == statuspb.NoCloudStatus_SUS

		if firstCondition || secondCondition || thirdCondition {
			go datas.PostInstanceStatus(i.GetUuid(), &statuspb.Status{
				Status: statuspb.NoCloudStatus_DEL,
			})
		}
	}

	go records(context.Background(), append(resourceRecords, productRecords...))
	if len(productRecords) != 0 {
		go events(context.Background(), &epb.Event{
			Uuid: i.GetUuid(),
			Key:  "instance_renew",
			Data: map[string]*structpb.Value{},
		})
	}
	go datas.DataPublisher(datas.POST_INST_DATA)(i.Uuid, i.Data)
}

func handleSuspendEvent(i *ipb.Instance, events EventsPublisherFunc) {
	if i.GetStatus() == statuspb.NoCloudStatus_DEL {
		return
	}

	data := i.GetData()
	now := time.Now().Unix()

	suspend_time, ok := data["suspend_time"]
	if !ok {
		return
	}

	suspend_time_value := int64(suspend_time.GetNumberValue())

	diff := now - suspend_time_value

	for _, val := range suspendNotificationsPeriods {
		if diff >= val.Timestamp {
			suspend_notification_period, ok := data["suspend_notification_period"]

			if !ok {
				data["suspend_notification_period"] = structpb.NewNumberValue(float64(val.Days))
				go events(context.Background(), &epb.Event{
					Uuid: i.GetUuid(),
					Key:  "suspend_expiry_notification",
					Data: map[string]*structpb.Value{
						"period": structpb.NewNumberValue(float64(val.Days)),
					},
				})
			}

			if val.Days != int64(suspend_notification_period.GetNumberValue()) {
				data["suspend_notification_period"] = structpb.NewNumberValue(float64(val.Days))
				go events(context.Background(), &epb.Event{
					Uuid: i.GetUuid(),
					Key:  "suspend_expiry_notification",
					Data: map[string]*structpb.Value{
						"period": structpb.NewNumberValue(float64(val.Days)),
					},
				})
			}
			break
		}
	}

	/*if int64(data["suspend_notification_period"].GetNumberValue()) == 7 {
		go datas.PostInstanceStatus(i.GetUuid(), &statuspb.Status{
			Status: statuspb.NoCloudStatus_DEL,
		})

		go events(context.Background(), &epb.Event{
			Uuid: i.GetUuid(),
			Key:  "suspend_delete_instance",
			Data: map[string]*structpb.Value{},
		})
	}*/

	i.Data = data
}

func handleBillingEvent(i *ipb.Instance, events EventsPublisherFunc) {
	if i.GetStatus() == statuspb.NoCloudStatus_DEL {
		return
	}

	data := i.GetData()
	now := time.Now().Unix()

	last_monitoring, ok := data["last_monitoring"]
	if !ok {
		return
	}

	last_monitoring_value := int64(last_monitoring.GetNumberValue())

	productName := i.GetProduct()

	products := i.GetBillingPlan().GetProducts()
	product, ok := products[productName]

	if !ok {
		return
	}

	productKind := product.GetKind()
	period := product.GetPeriod()

	var diff int64
	var expirationDate int64

	if productKind == billingpb.Kind_PREPAID {
		diff = last_monitoring_value - now
		expirationDate = last_monitoring_value
	} else {
		diff = last_monitoring_value + period - now
		expirationDate = last_monitoring_value + period
	}

	unix := time.Unix(expirationDate, 0)
	year, month, day := unix.Date()
	for _, val := range notificationsPeriods {
		if diff <= val.Timestamp {

			if val.Timestamp == period {
				break
			}

			notification_period, ok := data["notification_period"]
			if !ok {
				data["notification_period"] = structpb.NewNumberValue(float64(val.Days))
				go events(context.Background(), &epb.Event{
					Uuid: i.GetUuid(),
					Key:  "expiry_notification",
					Data: map[string]*structpb.Value{
						"period":  structpb.NewNumberValue(float64(val.Days)),
						"product": structpb.NewStringValue(i.GetProduct()),
						"date":    structpb.NewStringValue(fmt.Sprintf("%d/%d/%d", day, month, year)),
					},
				})
				continue
			}

			if val.Days != int64(notification_period.GetNumberValue()) {
				data["notification_period"] = structpb.NewNumberValue(float64(val.Days))
				go events(context.Background(), &epb.Event{
					Uuid: i.GetUuid(),
					Key:  "expiry_notification",
					Data: map[string]*structpb.Value{
						"period":  structpb.NewNumberValue(float64(val.Days)),
						"product": structpb.NewStringValue(i.GetProduct()),
						"date":    structpb.NewStringValue(fmt.Sprintf("%d/%d/%d", day, month, year)),
					},
				})
			}
			break
		}
	}
	i.Data = data
}

func handleManualRenewBilling(logger *zap.Logger, records RecordsPublisherFunc, i *ipb.Instance) {
	log := logger.Named("InstanceRenewBillingHandler").Named(i.GetUuid())
	log.Debug("Initializing")
	var recs []*billingpb.Record

	product := i.GetProduct()
	plan := i.GetBillingPlan()
	p := plan.GetProducts()[product]
	period := p.GetPeriod()
	resources := i.GetResources()

	//math.Round(float64((rec.End-rec.Start)/res.Period)*res.Price*amount()*100) / 100.0

	if period != 0 {
		log.Debug("Product")
		lastMonitoring := int64(i.GetData()["last_monitoring"].GetNumberValue())

		start := lastMonitoring
		end := start + period

		recs = append(recs, &billingpb.Record{
			Start:    start,
			End:      end,
			Exec:     time.Now().Unix(),
			Priority: billingpb.Priority_URGENT,
			Instance: i.GetUuid(),
			Product:  product,
			Total:    p.GetPrice(),
		})

		lastMonitoring += period
		i.Data["last_monitoring"] = structpb.NewNumberValue(float64(lastMonitoring))
	}

	for _, resource := range plan.GetResources() {
		log.Debug("Res", zap.String("key", resource.GetKey()))
		if resource.GetPeriod() == 0 {
			continue
		}
		lm := int64(i.GetData()[resource.GetKey()+"last_monitoring"].GetNumberValue())
		if strings.Contains(resource.GetKey(), "drive") {
			driveType := resources["drive_type"].GetStringValue()

			if resource.GetKey() != "drive_"+strings.ToLower(driveType) {
				continue
			}

			value := resources[resource.GetKey()].GetNumberValue() / 1024

			log.Debug("Res", zap.String("key", resource.GetKey()))
			recs = append(recs, &billingpb.Record{
				Start:    lm,
				End:      lm + resource.GetPeriod(),
				Exec:     time.Now().Unix(),
				Priority: billingpb.Priority_URGENT,
				Instance: i.GetUuid(),
				Product:  resource.GetKey(),
				Total:    math.Round(resource.GetPrice()*value*100) / 100.0,
			})
		} else {
			value := resources[resource.GetKey()].GetNumberValue()

			if resource.GetKey() == "ram" {
				value /= 1024
			}

			log.Debug("Res", zap.String("key", resource.GetKey()))
			recs = append(recs, &billingpb.Record{
				Start:    lm,
				End:      lm + resource.GetPeriod(),
				Exec:     time.Now().Unix(),
				Priority: billingpb.Priority_URGENT,
				Instance: i.GetUuid(),
				Product:  resource.GetKey(),
				Total:    math.Round(resource.GetPrice()*value*100) / 100.0,
			})
		}
		i.Data[resource.Key+"_last_monitoring"] = structpb.NewNumberValue(float64(lm + resource.GetPeriod()))
	}

	log.Debug("Data", zap.Any("d", i.GetData()))

	go records(context.Background(), recs)
	datas.DataPublisher(datas.POST_INST_DATA)(i.GetUuid(), i.GetData())
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

	if res.GetPeriod() == 0 {
		return handleCapacityZeroBilling(log.Named("DRIVE"), storage, ltl, i, res, last, clock)
	}

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

	if res.GetPeriod() == 0 {
		return handleCapacityZeroBilling(log.Named("IP"), ip, ltl, i, res, last, clock)
	}

	return handleCapacityBilling(log.Named("IP"), ip, ltl, i, res, last, clock)
}

func handleCPUBilling(log *zap.Logger, ltl LazyTimeline, i *ipb.Instance, vm LazyVM, res *billingpb.ResourceConf, c one.IClient, last int64, clock utils.IClock) ([]*billingpb.Record, int64) {
	o, _ := vm()
	cpu := Lazy(func() float64 {
		cpu, _ := o.Template.GetVCPU()
		return float64(cpu)
	})

	if res.GetPeriod() == 0 {
		return handleCapacityZeroBilling(log.Named("CPU"), cpu, ltl, i, res, last, clock)
	}

	return handleCapacityBilling(log.Named("CPU"), cpu, ltl, i, res, last, clock)
}

func handleRAMBilling(log *zap.Logger, ltl LazyTimeline, i *ipb.Instance, vm LazyVM, res *billingpb.ResourceConf, c one.IClient, last int64, clock utils.IClock) ([]*billingpb.Record, int64) {
	o, _ := vm()
	ram := Lazy(func() float64 {
		ram, _ := o.Template.GetMemory()
		return float64(ram) / 1024
	})

	if res.GetPeriod() == 0 {
		return handleCapacityZeroBilling(log.Named("RAM"), ram, ltl, i, res, last, clock)
	}

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

func handleCapacityZeroBilling(log *zap.Logger, amount func() float64, ltl LazyTimeline, i *ipb.Instance, res *billingpb.ResourceConf, last int64, time utils.IClock) ([]*billingpb.Record, int64) {
	now := time.Now().Unix()

	var records []*billingpb.Record
	records = append(records, &billingpb.Record{
		Resource: res.Key,
		Instance: i.GetUuid(),
		Start:    now, End: now + 1,
		Exec:     now,
		Priority: billingpb.Priority_URGENT,
		Total:    math.Round(res.Price*amount()*100) / 100.0,
	})

	return records, last
}

func handleStaticZeroBilling(log *zap.Logger, i *ipb.Instance, last int64, priority billingpb.Priority) ([]*billingpb.Record, int64) {
	log.Debug("Handling Static Billing", zap.Int64("last", last))
	product, ok := i.BillingPlan.Products[*i.Product]
	if !ok {
		log.Warn("Product not found", zap.String("product", *i.Product))
		return nil, last
	}

	var records []*billingpb.Record
	records = append(records, &billingpb.Record{
		Product:  *i.Product,
		Instance: i.GetUuid(),
		Start:    last, End: last + 1, Exec: last,
		Priority: billingpb.Priority_URGENT,
		Total:    math.Round(product.Price*100) / 100.0,
	})

	return records, last
}

func handleUpgradeBilling(log *zap.Logger, instances []*ipb.Instance, c *one.ONeClient, publish RecordsPublisherFunc) {
	now := time.Now().Unix()

	var records []*billingpb.Record

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
					log.Info("Billing res", zap.String("res", diff.ResName), zap.Float64("diff", diff.OldResCount))
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

					if res.Kind == billingpb.Kind_PREPAID {
						timeDiff := int64(lastMonitoring.GetNumberValue()) - now

						total := res.Price * (float64(timeDiff) / float64(res.GetPeriod())) * (diff.NewResCount - diff.OldResCount)
						total = math.Round(total*100) / 100.0

						records = append(records, &billingpb.Record{
							Start: now, End: int64(lastMonitoring.GetNumberValue()), Exec: now,
							Priority: billingpb.Priority_ADDITIONAL,
							Instance: inst.GetUuid(),
							Resource: diff.ResName,
							Total:    total,
						})
					}
				}
			}
		}
	}

	publish(context.Background(), records)
}
