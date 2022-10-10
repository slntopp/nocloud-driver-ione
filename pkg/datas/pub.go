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
	"go.uber.org/zap"

	amqp "github.com/rabbitmq/amqp091-go"
	i "github.com/slntopp/nocloud/pkg/instances"
)

var (
	log   *zap.Logger
	Pub   i.Pub
	IGPub i.Pub
)

func Configure(logger *zap.Logger, rbmq *amqp.Connection) {
	log = logger.Named("Datas")
	i := i.NewPubSub(log, nil, rbmq)
	ch := i.Channel()
	i.TopicExchange(ch, "datas")
	Pub = i.Publisher(ch, "datas", "instances")
	IGPub = i.Publisher(ch, "datas", "instances-groups")
}
