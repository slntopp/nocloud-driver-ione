package server

import (
	epb "github.com/slntopp/nocloud-proto/events"
	"google.golang.org/protobuf/types/known/structpb"
)

func shouldPublishOverdueTicket(now, due int64, data map[string]*structpb.Value) bool {
	if data == nil || due <= 0 {
		return false
	}
	if now <= due {
		delete(data, "overdue_ticket_created")
		return false
	}
	if v, ok := data["overdue_ticket_created"]; ok && v != nil && v.GetBoolValue() {
		return false
	}
	return true
}

func overdueTicketEvent(uuid string, due int64) *epb.Event {
	data := map[string]*structpb.Value{}
	if due > 0 {
		data["due"] = structpb.NewNumberValue(float64(due))
	}
	return &epb.Event{
		Uuid: uuid,
		Key:  "overdue_ticket",
		Data: data,
	}
}
