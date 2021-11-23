/*
Copyright © 2021 Nikita Ivanovski info@slnt-opp.xyz

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

	"github.com/gofrs/uuid"
	ione "github.com/slntopp/nocloud-driver-ione/pkg/driver"
	pb "github.com/slntopp/nocloud/pkg/drivers/instance/vanilla"
	instpb "github.com/slntopp/nocloud/pkg/instances/proto"
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

func (s *DriverServiceServer) TestServiceProviderConfig(ctx context.Context, sp *sppb.ServicesProvider) (res *sppb.TestResponse, err error) {
	s.log.Debug("TestServiceProviderConfig request received", zap.Any("sp", sp))
	secrets := sp.GetSecrets()
	host  := secrets["host"].GetStringValue()
	cred  := secrets["cred"].GetStringValue()
	group := secrets["group"].GetNumberValue()

	client := ione.NewIONeClient(host, cred, sp.GetVars(), s.log)
	pong, err := client.Ping()
	if err != nil {
		return &sppb.TestResponse{Result: false, Error: fmt.Sprintf("Ping didn't go through, error: %s", err.Error())}, nil
	}
	if !pong {
		return &sppb.TestResponse{Result: false, Error: "Ping didn't go through, check host, credentials and if IONe is running"}, nil
	}

	me, err := client.GetUser(-1)
	if err != nil {
		return &sppb.TestResponse{Result: false, Error: fmt.Sprintf("Can't get account: %s", err.Error())}, nil
	}
	isAdmin := false
	for _, g := range me.Groups.ID {
		isAdmin = isAdmin || g == 0
	}
	if !isAdmin {
		return &sppb.TestResponse{Result: false, Error: "User isn't admin(oneadmin group member)"}, nil
	}

	_, err = client.GetGroup(int64(group))
	if err != nil {
		return &sppb.TestResponse{Result: false, Error: fmt.Sprintf("Can't get group: %s", err.Error())}, nil
	}

	return &sppb.TestResponse{Result: true}, nil
}

func (s *DriverServiceServer) PrepareService(ctx context.Context, igroup *instpb.InstancesGroup, client *ione.IONe, group float64) (map[string]*structpb.Value, error) {
	id, err := uuid.NewV4()
	if err != nil {
		return nil, status.Error(codes.Internal, "Couldn't generate UUID")
	}

	data := igroup.GetData()
	if data["username"] == nil {
		data["username"] = structpb.NewStringValue(id.String())
	}
	username := data["username"].GetStringValue()

	hasher := sha256.New()
    hasher.Write([]byte(username + time.Now().String()))
    userPass := base64.URLEncoding.EncodeToString(hasher.Sum(nil))

	if data["user_id"] == nil {
		oneID, err := client.UserCreate(username, userPass, int64(group))
		if err != nil {
			s.log.Debug("Couldn't create OpenNebula user",
			zap.Error(err), zap.String("login", username),
			zap.String("pass", userPass), zap.Int64("group", int64(group)) )
			return nil, status.Error(codes.Internal, "Couldn't create OpenNebula user")
		}
		
		data["user_id"] = structpb.NewNumberValue(float64(oneID))
	}
	oneID := data["user_id"].GetNumberValue()

	resources := igroup.GetResources()
	var public_ips_amount float64 = 0
	if resources["ips_public"] != nil {
		public_ips_amount = resources["ips_public"].GetNumberValue()
	}

	var free float64 = 0
	if data["public_ips_free"] != nil {
		free = data["public_ips_free"].GetNumberValue()
	}
	if public_ips_amount > 0 && public_ips_amount > free {
		public_ips_amount -= free
		public_ips_pool_id, err := client.ReservePublicIP(oneID, public_ips_amount)
		if err != nil {
			s.log.Debug("Couldn't reserve Public IP addresses",
			zap.Error(err), zap.Float64("amount", public_ips_amount), zap.Float64("user", oneID))
			return nil, status.Error(codes.Internal, "Couldn't reserve Public IP addresses")
		}
		data["public_vn"] = structpb.NewNumberValue(public_ips_pool_id)
		total := float64(public_ips_amount)
		if data["public_ips_total"] != nil {
			total += data["public_ips_total"].GetNumberValue()
		}
		data["public_ips_total"] = structpb.NewNumberValue(total)

		data["public_ips_free"] = structpb.NewNumberValue(free + public_ips_amount)
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

	secrets := sp.GetSecrets()
	host := secrets["host"].GetStringValue()
	cred := secrets["cred"].GetStringValue()
	group := secrets["group"].GetNumberValue()

	client := ione.NewIONeClient(host, cred, sp.GetVars(), s.log)

	
	data := igroup.GetData()
	if data == nil {
		data = make(map[string]*structpb.Value)
		igroup.Data = data
	}

	data, err := s.PrepareService(ctx, igroup, client, group)
	if err != nil {
		return nil, err
	}

	for _, instance := range igroup.GetInstances() {
		client.TemplateInstantiate(instance, data)
	}

	igroup.Data = data
	return &pb.UpResponse{
		Group: igroup,
	}, nil
}