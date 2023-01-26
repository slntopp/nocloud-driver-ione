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
package datas

import (
	amqp "github.com/rabbitmq/amqp091-go"
	ipb "github.com/slntopp/nocloud-proto/instances"
	pdpb "github.com/slntopp/nocloud-proto/services_providers"
	stpb "github.com/slntopp/nocloud-proto/states"
	i "github.com/slntopp/nocloud/pkg/instances"
	pd "github.com/slntopp/nocloud/pkg/public_data"
	s "github.com/slntopp/nocloud/pkg/states"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/structpb"
)

var (
	log *zap.Logger

	DIPub  i.Pub
	DIGPub i.Pub

	StIPub  s.Pub
	StSpPub s.Pub

	PdSpPub pd.Pub
)

func Configure(logger *zap.Logger, rbmq *amqp.Connection) {
	log = logger.Named("Datas")
	i := i.NewPubSub(log, nil, rbmq)
	ich := i.Channel()
	i.TopicExchange(ich, "datas")
	DIPub = i.Publisher(ich, "datas", "instances")
	DIGPub = i.Publisher(ich, "datas", "instances-groups")

	log = logger.Named("PublicData")
	pd := pd.NewPublicDataPubSub(log, nil, rbmq)
	ch := pd.Channel()
	pd.TopicExchange(ch, "public_data")
	PdSpPub = pd.Publisher(ch, "public_data", "sp")

	log = logger.Named("States")
	s := s.NewStatesPubSub(log, nil, rbmq)
	sch := s.Channel()
	s.TopicExchange(sch, "states")
	StIPub = s.Publisher(sch, "states", "instances")
	StSpPub = s.Publisher(sch, "states", "sp")
}

func postInstData(uuid string, data map[string]*structpb.Value) {
	if DIPub != nil {
		msg := &ipb.ObjectData{Uuid: uuid, Data: data}
		if n, err := DIPub(msg); err != nil {
			log.Error("Failed to post instance Data", zap.Any("object_data", msg), zap.Int("len", n), zap.Error(err))
		}
	}
}

func postIGData(uuid string, data map[string]*structpb.Value) {
	if DIGPub != nil {
		msg := &ipb.ObjectData{Uuid: uuid, Data: data}
		if n, err := DIGPub(msg); err != nil {
			log.Error("Failed to post ig Data", zap.Any("object_data", msg), zap.Int("len", n), zap.Error(err))
		}
	}
}

func postInstState(uuid string, state *stpb.State) {
	if StIPub != nil {
		msg := &stpb.ObjectState{Uuid: uuid, State: state}
		if n, err := StIPub(msg); err != nil {
			log.Error("Failed to post instance state", zap.Any("object_state", msg), zap.Int("len", n), zap.Error(err))
		}
	}
}

func postSPState(uuid string, state *stpb.State) {
	if StSpPub != nil {
		msg := &stpb.ObjectState{Uuid: uuid, State: state}
		if n, err := StSpPub(&stpb.ObjectState{Uuid: uuid, State: state}); err != nil {
			log.Error("Failed to post sp state", zap.Any("object_state", msg), zap.Int("len", n), zap.Error(err))
		}
	}
}

func postSPPublicData(uuid string, data map[string]*structpb.Value) {
	if PdSpPub != nil {
		msg := &pdpb.ObjectPublicData{Uuid: uuid, Data: data}
		if n, err := PdSpPub(msg); err != nil {
			log.Error("Failed to post sp PublicData", zap.Any("object_public_data", msg), zap.Int("len", n), zap.Error(err))
		}
	}
}
