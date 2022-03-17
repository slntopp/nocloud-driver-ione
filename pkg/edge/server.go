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
package edge

import (
	"context"

	"github.com/slntopp/nocloud-driver-ione/pkg/actions"
	pb "github.com/slntopp/nocloud-driver-ione/pkg/edge/proto"
	stpb "github.com/slntopp/nocloud/pkg/states/proto"
	"go.uber.org/zap"
)

type EdgeServiceServer struct {
	pb.UnimplementedEdgeServiceServer

	log *zap.Logger
	st *stpb.StatesServiceClient
}

func NewEdgeServiceServer(log *zap.Logger, st *stpb.StatesServiceClient) *EdgeServiceServer {
	return &EdgeServiceServer{
		log: log.Named("EdgeService"),
		st: st,
	}
}

func (s *EdgeServiceServer) PostInstanceState(ctx context.Context, req *pb.InstanceState) (*pb.PostResponse, error) {
	actions.PostInstanceState(req.GetInstance(), req.GetData())
	return &pb.PostResponse{}, nil
}