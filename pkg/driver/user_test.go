package one

import (
	"testing"

	pb "github.com/slntopp/nocloud-proto/instances"
	"github.com/slntopp/nocloud-proto/services_providers"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/structpb"
)

var (
	log *zap.Logger
	sp  *services_providers.ServicesProvider
)

func init() {

	log = zap.NewExample()
	sp = &services_providers.ServicesProvider{
		Secrets: map[string]*structpb.Value{
			"host": structpb.NewStringValue("host"),
			"user": structpb.NewStringValue("user"),
			"pass": structpb.NewStringValue("pass"),
		},
		Vars: map[string]*services_providers.Var{
			"public_ip_pool": {
				Value: map[string]*structpb.Value{
					"default": structpb.NewNumberValue(406),
				},
			},
			"sched_ds": {
				Value: map[string]*structpb.Value{
					"HDD": structpb.NewStringValue("ID=121"),
					"SSD": structpb.NewStringValue("ID=119"),
				},
			},
		},
	}
}

func TestGenerateQuotaFromIGroup(t *testing.T) {

	og_quotas := []string{
		`<VM_QUOTA><VM><CPU>2</CPU><MEMORY>4096</MEMORY></VM></VM_QUOTA>`,
		`<NETWORK_QUOTA><NETWORK><ID>406</ID><LEASES>2</LEASES></NETWORK></NETWORK_QUOTA>`,
		`<DATASTORE_QUOTA><DATASTORE><ID>121</ID><SIZE>64000</SIZE></DATASTORE><DATASTORE><ID>119</ID><SIZE>32000</SIZE></DATASTORE></DATASTORE_QUOTA>`,
	}

	igroup := &pb.InstancesGroup{
		Resources: map[string]*structpb.Value{
			"cpu":        structpb.NewNumberValue(2),
			"ram":        structpb.NewNumberValue(4096),
			"drive_hdd":  structpb.NewNumberValue(64000),
			"drive_ssd":  structpb.NewNumberValue(32000),
			"ips_public": structpb.NewNumberValue(2),
		},
	}

	tmpl := GenerateQuotaFromIGroup(log, igroup, sp)
	if len(tmpl) != len(og_quotas) {
		t.Fatalf("GenerateQuotaFromIGroup() => %v", tmpl)
	}

	for i, v := range tmpl {
		if v != og_quotas[i] {
			t.Fatalf("GenerateQuotaFromIGroup() => %s, want %s", v, og_quotas[i])
		}
	}

}

func TestSetQuotaFromConfig(t *testing.T) {

	c, err := NewClientFromSP(sp, zap.NewExample())
	if err != nil {
		t.Fatalf("NewClientFromSP() => %v", err)
	}

	igroup := &pb.InstancesGroup{
		Resources: map[string]*structpb.Value{
			"cpu":        structpb.NewNumberValue(2),
			"ram":        structpb.NewNumberValue(4096),
			"drive_hdd":  structpb.NewNumberValue(64000),
			"drive_ssd":  structpb.NewNumberValue(32000),
			"ips_public": structpb.NewNumberValue(2),
		},
	}
	one_id := 1106

	err = c.SetQuotaFromConfig(one_id, igroup, sp)
	if err != nil {
		t.Fatalf("SetQuotaFromConfig() => %v", err)
	}
}
