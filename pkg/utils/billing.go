package utils

import (
	"google.golang.org/protobuf/types/known/structpb"
)

func SendActualMonitoringData(copiedData map[string]*structpb.Value, actualData map[string]*structpb.Value, instance string, publisher func(string, map[string]*structpb.Value)) {
	if val, ok := actualData["last_monitoring"]; ok {
		copiedData["actual_last_monitoring"] = val
	}
	if val, ok := actualData["next_payment_date"]; ok {
		copiedData["actual_next_payment_date"] = val
	}
	publisher(instance, copiedData)
}
