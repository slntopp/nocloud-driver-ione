package one

import (
	"fmt"

	"github.com/slntopp/nocloud/pkg/services_providers/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

var (
	// OpenNebula Scheduler Requirements (nodes provisioning rules for VMs deploy)
	SCHED = "sched"
	// OpenNebula DataStore Scheduler Requirements (datastores provisioning rules for VMs deploy)
	SCHED_DS = "sched_ds"
	// OpenNebula Super VNet public IP addresses to be reserved from
	PUBLIC_IP_POOL = "public_ip_pool"
)

func GetVarValue(in *proto.Var, key string) (r *structpb.Value, err error) {
	let := in.GetValue()
	r, ok := let[key]
	if ok {
		return r, nil
	}
	r, ok = let["default"]
	if ok {
		return r, nil
	}
	return nil, fmt.Errorf("Keys '%s' and 'default' are not set", key)
}