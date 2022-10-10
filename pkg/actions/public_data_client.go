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
package actions

import (
	amqp "github.com/rabbitmq/amqp091-go"
	one "github.com/slntopp/nocloud-driver-ione/pkg/driver"
	pd "github.com/slntopp/nocloud/pkg/public_data"
	pdpb "github.com/slntopp/nocloud/pkg/services_providers/proto"

	"go.uber.org/zap"
)

var (
	SPPDPub pd.Pub
)

func ConfigurePublicDataClient(logger *zap.Logger, rbmq *amqp.Connection) {
	log = logger.Named("PublicData")
	pd := pd.NewPublicDataPubSub(log, nil, rbmq)
	ch := pd.Channel()
	pd.TopicExchange(ch, "public_data")
	SPPDPub = pd.Publisher(ch, "public_data", "sp")
}

func PostServicesProviderPublicData(publicData *one.LocationPublicData) {
	request := &pdpb.ObjectPublicData{
		Uuid: publicData.Uuid,
		Data: publicData.PublicData,
	}
	err := SPPDPub(request)
	if err != nil {
		log.Error("Failed to post Location(ServicesProvider) PublicData", zap.Error(err))
	}
}
