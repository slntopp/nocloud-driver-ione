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
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"time"

	"github.com/gofrs/uuid"
	ione "github.com/slntopp/nocloud-driver-ione/pkg/driver"
	pb "github.com/slntopp/nocloud/pkg/drivers/instance/vanilla"
	instpb "github.com/slntopp/nocloud/pkg/instances/proto"
	"github.com/spf13/viper"
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

func MakeConfiguration(conf string) (res *viper.Viper, err error) {
	res = viper.New()
	res.SetConfigType("json")
	err = res.ReadConfig(bytes.NewBufferString(conf))
	if err != nil {
		return nil, err
	}

	// Default settings
	res.SetDefault("group_id", 1)

	return res, err
}

func (s *DriverServiceServer) GetType(ctx context.Context, request *pb.GetTypeRequest) (*pb.GetTypeResponse, error) {
	return &pb.GetTypeResponse{Type: DRIVER_TYPE}, nil
}

func (s *DriverServiceServer) ValidateConfigSyntax(ctx context.Context, request *instpb.ValidateInstancesGroupConfigRequest) (*instpb.ValidateInstancesGroupConfigResponse, error) {
	return &instpb.ValidateInstancesGroupConfigResponse{Result: true}, nil
}

func (s *DriverServiceServer) Deploy(ctx context.Context, input *pb.DeployRequest) (*pb.DeployResponse, error) {
	igroup := input.GetGroup()
	sp := input.GetServicesProvider()
	s.log.Debug("Deploy request received", zap.Any("instances_group", igroup))
	
	if igroup.GetType() != DRIVER_TYPE {
		return nil, status.Error(codes.InvalidArgument, "Wrong driver type")
	}

	id, err := uuid.NewV4()
	if err != nil {
		return nil, status.Error(codes.Internal, "Couldn't generate UUID")
	}

	DATA := make(map[string]*structpb.Value, 2)
	DATA["username"] = structpb.NewStringValue(id.String())

	hasher := sha256.New()
    hasher.Write([]byte(id.String() + time.Now().String()))
    userPass := base64.URLEncoding.EncodeToString(hasher.Sum(nil))
	
	secrets, err := MakeConfiguration(sp.GetSecrets())
	if err != nil {
		s.log.Error("Can't read config", zap.String("config", sp.GetSecrets()), zap.Error(err))
		return nil, status.Error(codes.InvalidArgument, "Can't read config")
	}
	host := secrets.GetString("host")
	cred := secrets.GetString("cred")
	group := secrets.GetInt64("group_id")

	client := ione.NewIONeClient(host, cred)
	oneID, err := client.UserCreate(id.String(), userPass, group)
	if err != nil {
		s.log.Debug("Couldn't create OpenNebula user",
			zap.Error(err), zap.String("login", id.String()),
			zap.String("pass", userPass), zap.Int64("group", group) )
		return nil, status.Error(codes.Internal, "Couldn't create OpenNebula user")
	}

	DATA["user_id"] = structpb.NewNumberValue(float64(oneID))

	return &pb.DeployResponse{
		Group: igroup, GroupData: DATA,
	}, nil
}