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
package main

import (
	"context"
	"fmt"
	epb "github.com/slntopp/nocloud-proto/events"
	"net"

	"github.com/go-redis/redis/v8"
	"github.com/slntopp/nocloud-driver-ione/pkg/actions"
	"github.com/slntopp/nocloud/pkg/nocloud"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"

	"github.com/slntopp/nocloud-driver-ione/pkg/datas"
	"github.com/slntopp/nocloud-driver-ione/pkg/server"

	amqp "github.com/rabbitmq/amqp091-go"
	billingpb "github.com/slntopp/nocloud-proto/billing"
	pb "github.com/slntopp/nocloud-proto/drivers/instance/vanilla"
)

var (
	port     string
	type_key string

	log          *zap.Logger
	RabbitMQConn string
	SIGNING_KEY  []byte
	redisHost    string
)

func init() {
	viper.AutomaticEnv()
	log = nocloud.NewLogger()

	viper.SetDefault("PORT", "8080")
	port = viper.GetString("PORT")

	viper.SetDefault("DRIVER_TYPE_KEY", "ione")
	type_key = viper.GetString("DRIVER_TYPE_KEY")

	viper.SetDefault("RABBITMQ_CONN", "amqp://nocloud:secret@rabbitmq:5672/")
	RabbitMQConn = viper.GetString("RABBITMQ_CONN")

	viper.SetDefault("SIGNING_KEY", "seeeecreet")
	SIGNING_KEY = []byte(viper.GetString("SIGNING_KEY"))

	viper.SetDefault("REDIS_HOST", "redis:6379")
	redisHost = viper.GetString("REDIS_HOST")
}

func main() {
	defer func() {
		_ = log.Sync()
	}()

	lis, err := net.Listen("tcp", fmt.Sprintf(":%v", port))
	if err != nil {
		log.Fatal("Failed to listen", zap.String("address", port), zap.Error(err))
	}

	log.Info("Dialing RabbitMQ", zap.String("url", RabbitMQConn))
	amqp.DialConfig(RabbitMQConn, amqp.Config{
		Properties: amqp.Table{
			"connection_name": "driver." + type_key,
		},
	})

	rbmq, err := amqp.Dial(RabbitMQConn)
	if err != nil {
		log.Fatal("Failed to connect to RabbitMQ", zap.Error(err))
	}
	defer rbmq.Close()
	log.Info("RabbitMQ connection established")

	datas.Configure(log, rbmq)
	actions.ConfigureStatusesClient(log)

	s := grpc.NewServer()
	server.SetDriverType(type_key)

	log.Info("Connecting redis", zap.String("url", redisHost))
	rdb := redis.NewClient(&redis.Options{
		Addr: redisHost,
		DB:   0, // use default DB
	})
	log.Info("RedisDB connection established")

	srv := server.NewDriverServiceServer(log.Named("IONe Driver"), SIGNING_KEY, rdb)
	srv.HandlePublishRecords = SetupRecordsPublisher(rbmq)
	srv.HandlePublishEvents = SetupEventPublisher(rbmq)

	pb.RegisterDriverServiceServer(s, srv)

	log.Info(fmt.Sprintf("Serving gRPC on 0.0.0.0:%v", port))
	log.Fatal("Failed to serve gRPC", zap.Error(s.Serve(lis)))
}

func SetupRecordsPublisher(rbmq *amqp.Connection) server.RecordsPublisherFunc {
	return func(ctx context.Context, payload []*billingpb.Record) {
		ch, err := rbmq.Channel()
		if err != nil {
			log.Fatal("Failed to open a channel", zap.Error(err))
		}
		defer ch.Close()

		queue, _ := ch.QueueDeclare(
			"records",
			true, false, false, true, nil,
		)

		for _, record := range payload {
			body, err := proto.Marshal(record)
			if err != nil {
				log.Error("Error while marshalling record", zap.Error(err))
				continue
			}
			ch.PublishWithContext(ctx, "", queue.Name, false, false, amqp.Publishing{
				ContentType: "text/plain", Body: body,
			})
		}
	}
}

func SetupEventPublisher(rbmq *amqp.Connection) server.EventsPublisherFunc {
	return func(ctx context.Context, event *epb.Event) {
		ch, err := rbmq.Channel()
		if err != nil {
			log.Fatal("Failed to open a channel", zap.Error(err))
		}
		defer ch.Close()

		queue, _ := ch.QueueDeclare(
			"events",
			true, false, false, true, nil,
		)

		body, err := proto.Marshal(event)
		if err != nil {
			log.Error("Error while marshalling record", zap.Error(err))
			return
		}
		ch.PublishWithContext(ctx, "", queue.Name, false, false, amqp.Publishing{
			ContentType: "text/plain", Body: body,
		})
	}
}
