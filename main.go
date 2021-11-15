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
package main

import (
	"fmt"
	"net"

	"github.com/slntopp/nocloud/pkg/nocloud"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/slntopp/nocloud-driver-ione/pkg/server"
	pb "github.com/slntopp/nocloud/pkg/drivers/instance/vanilla"
)

var (
	port 	 string
	type_key string

	log  	 *zap.Logger
)

func init() {
	viper.AutomaticEnv()
	log = nocloud.NewLogger()

	viper.SetDefault("PORT", "8080")
	port = viper.GetString("PORT")

	viper.SetDefault("DRIVER_TYPE_KEY", "ione")
	type_key = viper.GetString("DRIVER_TYPE_KEY")
}

func main() {
	defer func() {
		_ = log.Sync()
	}()

	lis, err := net.Listen("tcp", fmt.Sprintf(":%v", port))
	if err != nil {
		log.Fatal("Failed to listen", zap.String("address", port), zap.Error(err))
	}

	s := grpc.NewServer()
	server.SetDriverType(type_key)
	srv := server.NewDriverServiceServer(log.Named("IONe Driver"))

	pb.RegisterDriverServiceServer(s, srv)

	log.Info(fmt.Sprintf("Serving gRPC on 0.0.0.0:%v", port), zap.Skip())
	log.Fatal("Failed to serve gRPC", zap.Error(s.Serve(lis)))
}