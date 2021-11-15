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

	pb "github.com/slntopp/nocloud/pkg/drivers/instance/vanilla"
	instpb "github.com/slntopp/nocloud/pkg/instances/proto"
	"github.com/spf13/viper"
	"go.uber.org/zap"
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

func (s *DriverServiceServer) Deploy(ctx context.Context, service *instpb.InstancesGroup) (*instpb.InstancesGroup, error) {
	s.log.Debug("Deploy request received", zap.Any("service", service))
	return service, nil
}