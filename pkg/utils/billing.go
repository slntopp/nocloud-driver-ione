package utils

import (
	"google.golang.org/protobuf/types/known/structpb"
)

func SendActualMonitoringData(copiedData map[string]*structpb.Value, actualData map[string]*structpb.Value, instance string, publisher func(string, map[string]*structpb.Value)) {
	copiedData["actual_last_monitoring"] = actualData["last_monitoring"]
	copiedData["actual_next_payment_date"] = actualData["next_payment_date"]
	publisher(instance, copiedData)
}
