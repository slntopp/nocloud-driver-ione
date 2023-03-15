package server

import (
	"context"
	one "github.com/slntopp/nocloud-driver-ione/pkg/driver"
	epb "github.com/slntopp/nocloud-proto/events"
	"google.golang.org/protobuf/types/known/structpb"
)

func handleInstEvents(ctx context.Context, resp *one.CheckInstancesGroupResponse, events EventsPublisherFunc) {
	for _, inst := range resp.ToBeCreated {
		go events(ctx, &epb.Event{
			Uuid: inst.GetUuid(),
			Key:  "instance_created",
			Data: map[string]*structpb.Value{
				"instance": structpb.NewStringValue(inst.GetTitle()),
			},
		})
	}

	for _, inst := range resp.ToBeDeleted {
		go events(ctx, &epb.Event{
			Uuid: inst.GetUuid(),
			Key:  "instance_deleted",
			Data: map[string]*structpb.Value{
				"instance": structpb.NewStringValue(inst.GetTitle()),
			},
		})
	}
}
