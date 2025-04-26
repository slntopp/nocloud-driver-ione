package server

import (
	"context"
	"github.com/slntopp/nocloud-driver-ione/pkg/datas"
	one "github.com/slntopp/nocloud-driver-ione/pkg/driver"
	epb "github.com/slntopp/nocloud-proto/events"
	"google.golang.org/protobuf/types/known/structpb"
)

func handleInstEvents(ctx context.Context, resp *one.CheckInstancesGroupResponse, events EventsPublisherFunc) {
	for _, inst := range resp.ToBeDeleted {
		if inst.Data == nil {
			inst.Data = make(map[string]*structpb.Value)
		}
		if inst.Data["deleted_notification"].GetBoolValue() {
			continue
		}
		go events(ctx, &epb.Event{
			Uuid: inst.GetUuid(),
			Key:  "instance_deleted",
			Data: map[string]*structpb.Value{},
		})
		inst.Data["deleted_notification"] = structpb.NewBoolValue(true)
		datas.DataPublisher(datas.POST_INST_DATA)(inst.GetUuid(), inst.Data)
	}
}
