/*
Copyright © 2021-2022 Nikita Ivanovski info@slnt-opp.xyz

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package server

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"

	redis "github.com/go-redis/redis/v8"
	"github.com/slntopp/nocloud-driver-ione/pkg/actions"
	"github.com/slntopp/nocloud-driver-ione/pkg/datas"
	one "github.com/slntopp/nocloud-driver-ione/pkg/driver"
	pb "github.com/slntopp/nocloud/pkg/drivers/instance/vanilla"
	"github.com/slntopp/nocloud/pkg/instances/proto"
	ipb "github.com/slntopp/nocloud/pkg/instances/proto"
	auth "github.com/slntopp/nocloud/pkg/nocloud/auth"
	sppb "github.com/slntopp/nocloud/pkg/services_providers/proto"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

var (
	DRIVER_TYPE      string
	MONITORING_REDIS = "MONITORING"
)

func SetDriverType(_type string) {
	DRIVER_TYPE = _type
}

type DriverServiceServer struct {
	pb.UnimplementedDriverServiceServer
	log                  *zap.Logger
	HandlePublishRecords RecordsPublisherFunc
	rdb                  *redis.Client
}

func NewDriverServiceServer(log *zap.Logger, key []byte, rdb *redis.Client) *DriverServiceServer {
	auth.SetContext(log, key)
	return &DriverServiceServer{log: log, rdb: rdb}
}

func (s *DriverServiceServer) GetType(ctx context.Context, request *pb.GetTypeRequest) (*pb.GetTypeResponse, error) {
	return &pb.GetTypeResponse{Type: DRIVER_TYPE}, nil
}

func (s *DriverServiceServer) TestInstancesGroupConfig(ctx context.Context, request *ipb.TestInstancesGroupConfigRequest) (*ipb.TestInstancesGroupConfigResponse, error) {
	s.log.Debug("TestInstancesGroupConfig request received", zap.Any("request", request))
	igroup := request.GetGroup()
	if igroup.GetType() != DRIVER_TYPE {
		Errors := []*ipb.TestInstancesGroupConfigError{
			{Error: fmt.Sprintf("Group type(%s) isn't matching Driver type(%s)", igroup.GetType(), DRIVER_TYPE)},
		}
		return &ipb.TestInstancesGroupConfigResponse{Result: false, Errors: Errors}, nil
	}

	return &ipb.TestInstancesGroupConfigResponse{Result: true}, nil
}

func (s *DriverServiceServer) TestServiceProviderConfig(ctx context.Context, req *pb.TestServiceProviderConfigRequest) (res *sppb.TestResponse, err error) {
	sp := req.GetServicesProvider()
	s.log.Debug("TestServiceProviderConfig request received", zap.Any("sp", sp), zap.Bool("syntax_only", req.GetSyntaxOnly()))

	client, err := one.NewClientFromSP(sp, s.log)

	if err != nil {
		return &sppb.TestResponse{Result: false, Error: err.Error()}, nil
	}

	vars := sp.GetVars()
	{
		_, sched_ok := vars[one.SCHED]
		if !sched_ok {
			return &sppb.TestResponse{Result: false, Error: "Scheduler requirements unset"}, nil
		}
		_, sched_ds_ok := vars[one.SCHED_DS]
		if !sched_ok {
			return &sppb.TestResponse{Result: false, Error: "DataStore Scheduler requirements unset"}, nil
		}
		_, vnet_ok := vars[one.PUBLIC_IP_POOL]
		if !(sched_ok && sched_ds_ok && vnet_ok) {
			return &sppb.TestResponse{Result: false, Error: "Public IPs Pool unset"}, nil
		}
	}

	if req.GetSyntaxOnly() {
		return &sppb.TestResponse{Result: true}, nil
	}

	me, err := client.GetUser(-1)
	if err != nil {
		return &sppb.TestResponse{Result: false, Error: fmt.Sprintf("Can't get account: %s", err.Error())}, nil
	}

	s.log.Debug("Got user", zap.Any("user", me))
	isAdmin := me.GID == 0
	for _, g := range me.Groups.ID {
		isAdmin = isAdmin || g == 0
	}
	if !isAdmin {
		return &sppb.TestResponse{Result: false, Error: "User isn't admin(oneadmin group member)"}, nil
	}

	secrets := sp.GetSecrets()
	group := secrets["group"].GetNumberValue()

	_, err = client.GetGroup(int(group))
	if err != nil {
		return &sppb.TestResponse{Result: false, Error: fmt.Sprintf("Can't get group: %s", err.Error())}, nil
	}

	return &sppb.TestResponse{Result: true}, nil
}

func (s *DriverServiceServer) PrepareService(ctx context.Context, sp *sppb.ServicesProvider, igroup *ipb.InstancesGroup, client one.VMClient, group float64) (map[string]*structpb.Value, error) {
	data := igroup.GetData()
	username := igroup.GetUuid()

	hasher := sha256.New()
	hasher.Write([]byte(username + time.Now().String()))
	userPass := base64.URLEncoding.EncodeToString(hasher.Sum(nil))

	if data["userid"] == nil {
		oneID, err := client.CreateUser(username, userPass, []int{int(group)})
		if err != nil {
			s.log.Debug("Couldn't create OpenNebula user",
				zap.Error(err), zap.String("login", username),
				zap.String("pass", userPass), zap.Int64("group", int64(group)))
			return nil, status.Error(codes.Internal, "Couldn't create OpenNebula user")
		}

		data["userid"] = structpb.NewNumberValue(float64(oneID))

		client.UserAddAttribute(oneID, map[string]interface{}{
			"NOCLOUD": "TRUE",
		})
	}
	oneID := int(data["userid"].GetNumberValue())

	resources := igroup.GetResources()
	var public_ips_amount int = 0
	if resources["ips_public"] != nil {
		public_ips_amount = int(resources["ips_public"].GetNumberValue())
	}

	var freePubIps int = 0
	if data["public_ips_free"] != nil {
		freePubIps = int(data["public_ips_free"].GetNumberValue())
	}
	if public_ips_amount > 0 && public_ips_amount > freePubIps {
		public_ips_amount -= freePubIps
		public_ips_pool_id, err := client.ReservePublicIP(oneID, public_ips_amount)
		if err != nil {
			s.log.Debug("Couldn't reserve Public IP addresses",
				zap.Error(err), zap.Int("amount", public_ips_amount), zap.Int("user", oneID))

			client.DeleteUserAndVNets(oneID)

			return nil, status.Error(codes.Internal, "Couldn't reserve Public IP addresses")
		}
		data["public_vn"] = structpb.NewNumberValue(float64(public_ips_pool_id))
		total := float64(public_ips_amount)
		if data["public_ips_total"] != nil {
			total += data["public_ips_total"].GetNumberValue()
		}
		data["public_ips_total"] = structpb.NewNumberValue(total)

		data["public_ips_free"] = structpb.NewNumberValue(float64(freePubIps + public_ips_amount))
	}

	var private_ips_amount int = 0
	if resources["ips_private"] != nil {
		private_ips_amount = int(resources["ips_private"].GetNumberValue())
	}

	/*var address_range_id int = -1
	if data["address_range_id"] != nil {
		address_range_id = int(data["address_range_id"].GetNumberValue())
	}*/

	_, err := client.GetUserPrivateVNet(oneID)

	if private_ips_amount > 0 && err.Error() == "resource not found" {
		vnMad, freeVlan, err := client.FindFreeVlan(sp)
		if err != nil {
			s.log.Debug("Couldn't reserve Private IP addresses",
				zap.Error(err), zap.Int("amount", private_ips_amount), zap.Int("user", oneID))

			client.DeleteUserAndVNets(oneID)

			return nil, status.Error(codes.Internal, "Couldn't reserve Private IP addresses")
		}

		private_ips_pool_id, err := client.ReservePrivateIP(oneID, vnMad, freeVlan)
		if err != nil {
			s.log.Debug("Couldn't reserve Private IP addresses",
				zap.Error(err), zap.Int("amount", private_ips_amount), zap.Int("user", oneID))

			client.DeleteUserAndVNets(oneID)

			return nil, status.Error(codes.Internal, "Couldn't reserve Private IP addresses")
		}
		data["private_vn"] = structpb.NewNumberValue(float64(private_ips_pool_id))
	}

	return data, nil
}

func (s *DriverServiceServer) Up(ctx context.Context, input *pb.UpRequest) (*pb.UpResponse, error) {
	igroup := input.GetGroup()
	sp := input.GetServicesProvider()
	log := s.log.Named("Up")
	log.Debug("Request received", zap.Any("instances_group", igroup))

	if igroup.GetType() != DRIVER_TYPE {
		return nil, status.Error(codes.InvalidArgument, "Wrong driver type")
	}

	client, err := one.NewClientFromSP(sp, log)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Error making client: %v", err)
	}
	client.SetVars(sp.GetVars())

	s.Monitoring(ctx, &pb.MonitoringRequest{Groups: []*ipb.InstancesGroup{igroup}, ServicesProvider: sp, Scheduled: false})

	log.Debug("Up request completed", zap.Any("instances_group", igroup))
	return &pb.UpResponse{
		Group: igroup,
	}, nil
}

func (s *DriverServiceServer) Down(ctx context.Context, input *pb.DownRequest) (*pb.DownResponse, error) {
	igroup := input.GetGroup()
	sp := input.GetServicesProvider()
	s.log.Debug("Down request received", zap.Any("instances_group", igroup))

	if igroup.GetType() != DRIVER_TYPE {
		return nil, status.Error(codes.InvalidArgument, "Wrong driver type")
	}

	client, err := one.NewClientFromSP(sp, s.log)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Error making client: %v", err)
	}

	for i, instance := range igroup.GetInstances() {
		data := instance.GetData()
		if _, ok := data["vmid"]; !ok {
			s.log.Error("Instance has no VM ID in data", zap.Any("data", data), zap.String("instance", instance.GetUuid()))
		}
		vmid := int(data["vmid"].GetNumberValue())
		client.TerminateVM(vmid, true)

		delete(instance.Data, "vmid")
		delete(instance.Data, "vm_name")

		igroup.Instances[i] = instance

		go datas.Pub(&ipb.ObjectData{
			Uuid: instance.Uuid,
			Data: instance.Data,
		})
	}

	data := igroup.GetData()
	if _, ok := data["userid"]; !ok {
		s.log.Error("InstanceGroup has no User ID in data", zap.Any("data", data), zap.String("group", igroup.GetUuid()))
		return &pb.DownResponse{Group: igroup}, nil
	}
	userid := int(data["userid"].GetNumberValue())
	err = client.DeleteUserAndVNets(userid)
	if err != nil {
		s.log.Error("Error deleting OpenNebula User", zap.Error(err))
	}

	igroup.Data = make(map[string]*structpb.Value)
	go datas.IGPub(&ipb.ObjectData{
		Uuid: igroup.Uuid,
		Data: igroup.Data,
	})

	s.log.Debug("Down request completed", zap.Any("instances_group", igroup))
	return &pb.DownResponse{Group: igroup}, nil
}

func (s *DriverServiceServer) Monitoring(ctx context.Context, req *pb.MonitoringRequest) (*pb.MonitoringResponse, error) {

	log := s.log.Named("Monitoring")
	sp := req.GetServicesProvider()
	log.Info("Starting Monitoring Routine", zap.String("sp", sp.GetUuid()))

	client, err := one.NewClientFromSP(sp, log)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Error making client: %v", err)
	}

	secrets := sp.GetSecrets()
	vars := sp.GetVars()

	client.SetVars(vars)

	group := secrets["group"].GetNumberValue()

	redisKey := fmt.Sprintf("%s-SP-%s", MONITORING_REDIS, sp.Uuid)

	for _, ig := range req.GetGroups() {
		log.Debug("Monitoring group", zap.String("group", ig.GetUuid()), zap.String("title", ig.GetTitle()))
		l := log.Named(ig.Uuid)

		// checking for unscheduled monitoring
		if req.Scheduled {
			if monitoredRecently := s.rdb.HExists(ctx, redisKey, ig.Uuid).Val(); monitoredRecently {
				continue
			}
		} else {
			s.rdb.HSet(ctx, redisKey, ig.Uuid, "MONITORED")
		}

		err = client.CheckOrphanInstanceGroup(ig, group)
		if err != nil {
			log.Error("Error Checking Orphan User of Instance Group", zap.String("ig", ig.GetUuid()), zap.Error(err))
		}

		resp, err := client.CheckInstancesGroup(ig)
		if err != nil {
			log.Error("Error Checking Instances Group", zap.String("ig", ig.GetUuid()), zap.Error(err))
		} else {
			log.Debug("Check Instances Group Response", zap.Any("resp", resp))

			if len(resp.ToBeCreated) > 0 {
				group := secrets["group"].GetNumberValue()

				data := ig.GetData()
				if data == nil {
					data = make(map[string]*structpb.Value)
					ig.Data = data
				}

				data, err = s.PrepareService(ctx, sp, ig, client, group)
				if err != nil {
					log.Error("Error Preparing Service", zap.Any("group", ig), zap.Error(err))
					return nil, err
				}

				ig.Data = data
				datas.IGPub(&ipb.ObjectData{
					Uuid: ig.Uuid,
					Data: ig.Data,
				})
			}

			client.CheckInstancesGroupResponseProcess(resp, ig.GetData(), int(group))
		}

		log.Debug("Monitoring instances", zap.String("group", ig.GetUuid()), zap.Int("instances", len(ig.GetInstances())))
		for _, inst := range ig.GetInstances() {
			l.Debug("Monitoring instance", zap.String("instance", inst.GetUuid()), zap.String("title", inst.GetTitle()))
			_, err = actions.StatusesClient(client, inst, inst.GetData(), &ipb.InvokeResponse{Result: true})
			if err != nil {
				log.Error("Error Monitoring Instance", zap.Any("instance", inst), zap.Error(err))
			}

			go handleInstanceBilling(log, s.HandlePublishRecords, client, inst)
		}
	}

	// cleaning of unschedully monitored IGs
	if req.Scheduled {
		igKeys := s.rdb.HKeys(ctx, redisKey).Val()
		s.rdb.HDel(ctx, redisKey, igKeys...)
	}

	st, pd, err := client.MonitorLocation(sp)
	if err != nil {
		log.Error("Error Monitoring Location(ServicesProvider)", zap.String("sp", sp.GetUuid()), zap.Error(err))
		return &pb.MonitoringResponse{}, nil
	}

	log.Debug("Location Monitoring", zap.Any("state", st), zap.Any("public_data", pd))

	actions.PostServicesProviderState(st)
	actions.PostServicesProviderPublicData(pd)

	log.Info("Monitoring Routine Done", zap.String("sp", sp.GetUuid()))
	return &pb.MonitoringResponse{}, nil
}

func (s *DriverServiceServer) SuspendMonitoring(ctx context.Context, req *pb.MonitoringRequest) (*pb.MonitoringResponse, error) {
	log := s.log.Named("Monitoring")
	sp := req.GetServicesProvider()
	log.Info("Starting Monitoring Routine", zap.String("sp", sp.GetUuid()))

	c, err := one.NewClientFromSP(sp, log)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Error making client: %v", err)
	}

	for _, group := range req.GetGroups() {
		for _, inst := range group.GetInstances() {

			vmid, err := one.GetVMIDFromData(c, inst)
			if err != nil {
				log.Warn("Failed to obtain VM ID", zap.String("instance", inst.Uuid), zap.Error(err))
				continue
			}

			_, state, _, _, err := c.StateVM(vmid)
			if err != nil {
				log.Warn("Could not get state for VM ID", zap.Int("vmid", vmid))
				continue
			}

			if group.Status != proto.InstanceStatus_SUS && state == "SUSPENDED" {
				if err := c.ResumeVM(vmid); err != nil {
					log.Warn("Could not resume VM with VMID", zap.Int("vmid", vmid))
					continue
				}
			}

			if group.Status == proto.InstanceStatus_SUS && state != "SUSPENDED" {
				if err := c.SuspendVM(vmid); err != nil {
					log.Warn("Could not suspend VM with VMID", zap.Int("vmid", vmid))
					continue
				}
			}
		}
	}
	return &pb.MonitoringResponse{}, nil
}
