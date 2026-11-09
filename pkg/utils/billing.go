package utils

import (
	"strings"

	"google.golang.org/protobuf/types/known/structpb"
)

func SendActualMonitoringData(copiedData map[string]*structpb.Value, actualData map[string]*structpb.Value, instance string, publisher func(string, map[string]*structpb.Value)) {
	if copiedData == nil {
		return
	}
	if val, ok := actualData["last_monitoring"]; ok {
		copiedData["actual_last_monitoring"] = val
	}
	if val, ok := actualData["next_payment_date"]; ok {
		copiedData["actual_next_payment_date"] = val
	}
	patch := BillingDatePatch(copiedData)
	if patch == nil {
		return
	}
	publisher(instance, patch)
}

// BillingDatePatch keeps payment-date keys only and marks the payload as a merge.
func BillingDatePatch(data map[string]*structpb.Value) map[string]*structpb.Value {
	if data == nil {
		return nil
	}
	patch := make(map[string]*structpb.Value)
	for k, v := range data {
		if !isBillingDateKey(k) {
			continue
		}
		patch[k] = v
	}
	if len(patch) == 0 {
		return nil
	}
	patch["_patch"] = structpb.NewBoolValue(true)
	return patch
}

func isBillingDateKey(k string) bool {
	switch k {
	case "last_monitoring", "next_payment_date", "actual_last_monitoring", "actual_next_payment_date":
		return true
	default:
		return strings.HasSuffix(k, "_last_monitoring") || strings.HasSuffix(k, "_next_payment_date")
	}
}
