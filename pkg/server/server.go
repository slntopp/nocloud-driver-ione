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

	"github.com/slntopp/nocloud-driver-ione/pkg/actions"
	one "github.com/slntopp/nocloud-driver-ione/pkg/driver"
	pb "github.com/slntopp/nocloud/pkg/drivers/instance/vanilla"
	instpb "github.com/slntopp/nocloud/pkg/instances/proto"
	srvpb "github.com/slntopp/nocloud/pkg/services/proto"
	sppb "github.com/slntopp/nocloud/pkg/services_providers/proto"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

var DRIVER_TYPE string

func SetDriverType(_type string) {
	DRIVER_TYPE = _type
}

type DriverServiceServer struct {
	pb.UnimplementedDriverServiceServer
	log *zap.Logger
}

func NewDriverServiceServer(log *zap.Logger) *DriverServiceServer {
	return &DriverServiceServer{log: log}
}

func (s *DriverServiceServer) GetType(ctx context.Context, request *pb.GetTypeRequest) (*pb.GetTypeResponse, error) {
	return &pb.GetTypeResponse{Type: DRIVER_TYPE}, nil
}

func (s *DriverServiceServer) TestInstancesGroupConfig(ctx context.Context, request *instpb.TestInstancesGroupConfigRequest) (*instpb.TestInstancesGroupConfigResponse, error) {
	s.log.Debug("TestInstancesGroupConfig request received", zap.Any("request", request))
	igroup := request.GetGroup()
	if igroup.GetType() != DRIVER_TYPE {
		Errors := []*instpb.TestInstancesGroupConfigError{
			{Error: fmt.Sprintf("Group type(%s) isn't matching Driver type(%s)", igroup.GetType(), DRIVER_TYPE)},
		}
		return &instpb.TestInstancesGroupConfigResponse{Result: false, Errors: Errors}, nil
	}

	return &instpb.TestInstancesGroupConfigResponse{Result: true}, nil
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

func (s *DriverServiceServer) PrepareService(ctx context.Context, igroup *instpb.InstancesGroup, client *one.ONeClient, group float64) (map[string]*structpb.Value, error) {
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
	}
	oneID := int(data["userid"].GetNumberValue())

	resources := igroup.GetResources()
	var public_ips_amount int = 0
	if resources["ips_public"] != nil {
		public_ips_amount = int(resources["ips_public"].GetNumberValue())
	}

	var free int = 0
	if data["public_ips_free"] != nil {
		free = int(data["public_ips_free"].GetNumberValue())
	}
	if public_ips_amount > 0 && public_ips_amount > free {
		public_ips_amount -= free
		public_ips_pool_id, err := client.ReservePublicIP(oneID, public_ips_amount)
		if err != nil {
			s.log.Debug("Couldn't reserve Public IP addresses",
				zap.Error(err), zap.Int("amount", public_ips_amount), zap.Int("user", oneID))
			return nil, status.Error(codes.Internal, "Couldn't reserve Public IP addresses")
		}
		data["public_vn"] = structpb.NewNumberValue(float64(public_ips_pool_id))
		total := float64(public_ips_amount)
		if data["public_ips_total"] != nil {
			total += data["public_ips_total"].GetNumberValue()
		}
		data["public_ips_total"] = structpb.NewNumberValue(total)

		data["public_ips_free"] = structpb.NewNumberValue(float64(free + public_ips_amount))
	}

	return data, nil
}

func (s *DriverServiceServer) Up(ctx context.Context, input *pb.UpRequest) (*pb.UpResponse, error) {
	igroup := input.GetGroup()
	sp := input.GetServicesProvider()
	s.log.Debug("Up request received", zap.Any("instances_group", igroup))

	if igroup.GetType() != DRIVER_TYPE {
		return nil, status.Error(codes.InvalidArgument, "Wrong driver type")
	}

	client, err := one.NewClientFromSP(sp, s.log)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Error making client: %v", err)
	}
	client.SetSecrets(sp.GetSecrets())
	client.SetVars(sp.GetVars())

	secrets := sp.GetSecrets()
	group := secrets["group"].GetNumberValue()

	data := igroup.GetData()
	if data == nil {
		data = make(map[string]*structpb.Value)
		igroup.Data = data
	}

	data, err = s.PrepareService(ctx, igroup, client, group)
	if err != nil {
		s.log.Error("Error Preparing Service", zap.Any("group", igroup), zap.Error(err))
		return nil, err
	}
	userid := int(data["userid"].GetNumberValue())
	for _, instance := range igroup.GetInstances() {
		vmid, err := client.InstantiateTemplateHelper(instance, data)
		if err != nil {
			s.log.Error("Error deploying VM", zap.String("instance", instance.GetUuid()), zap.Error(err))
			continue
		}
		client.Chown("vm", vmid, userid, int(group))
	}

	igroup.Data = data
	s.log.Debug("Up request completed", zap.Any("instances_group", igroup))
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
		instance.Data = make(map[string]*structpb.Value)
		igroup.Instances[i] = instance
	}

	data := igroup.GetData()
	if _, ok := data["userid"]; !ok {
		s.log.Error("InstanceGroup has no User ID in data", zap.Any("data", data), zap.String("group", igroup.GetUuid()))
		return &pb.DownResponse{Group: igroup}, nil
	}
	userid := int(data["userid"].GetNumberValue())
	err = client.DeleteUser(userid)
	if err != nil {
		s.log.Error("Error deleting OpenNebula User", zap.Error(err))
	}

	igroup.Data = make(map[string]*structpb.Value)

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
	client.SetSecrets(sp.GetSecrets())
	client.SetVars(sp.GetVars())

	for _, ig := range req.GetGroups() {
		for _, inst := range ig.GetInstances() {
			_, err := actions.StatusesClient(client, inst, inst.GetData(), &srvpb.PerformActionResponse{Result: true})
			if err != nil {
				log.Error("Error Monitoring Instance", zap.Any("instance", inst), zap.Error(err))
			}
			vmid, err := actions.GetVMIDFromData(client, inst)
			if err != nil {
				log.Error("Error getting VM ID from data", zap.Error(err))
				continue
			}
			res, err := client.VMToInstance(vmid)
			log.Debug("Got Instance config from template", zap.Any("inst", res), zap.Error(err))
		}
	}

	r, err := client.MonitorLocation(sp)
	if err != nil {
		log.Error("Error Monitoring Location(ServicesProvider)", zap.String("sp", sp.GetUuid()), zap.Error(err))
		return &pb.MonitoringResponse{}, nil
	}

	actions.PostServicesProviderState(r)

	log.Info("Monitoring Routine Done", zap.String("sp", sp.GetUuid()))
	return &pb.MonitoringResponse{}, nil
}