package one

import (
	"strings"
	"testing"

	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/vm"
	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/vm/keys"
)

func TestReinstallContextKeepsNetwork(t *testing.T) {
	base := vm.NewTemplate()
	_ = base.AddCtx(keys.Context("ETH0_IP"), "185.66.71.171")
	_ = base.AddCtx(keys.Context("ETH0_GATEWAY"), "185.66.71.129")
	_ = base.AddCtx(keys.Context("ETH0_MASK"), "255.255.255.128")
	_ = base.AddCtx(keys.Username, "ubuntu")
	_ = base.AddCtx(keys.Context("PASSWORD"), "old-secret")
	_ = base.AddCtx(keys.Context("DISK_ID"), "1")
	_ = base.AddCtx(keys.TargetCtx, "hda")

	conf := reinstallContext(base, "root", "new-secret")
	if conf == nil {
		t.Fatal("context is empty")
	}
	got := conf.String()
	for _, want := range []string{`ETH0_IP="185.66.71.171"`, `ETH0_GATEWAY="185.66.71.129"`, `ETH0_MASK="255.255.255.128"`, `USERNAME="root"`, `PASSWORD="new-secret"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %s in %s", want, got)
		}
	}
	if strings.Contains(got, "DISK_ID") || strings.Contains(got, "TARGET") || strings.Contains(got, "old-secret") || strings.Contains(got, "ubuntu") {
		t.Fatalf("context kept device or stale login fields: %s", got)
	}
}

func TestReinstallContextRebuildsAddressFromNIC(t *testing.T) {
	base := vm.NewTemplate()
	_ = base.AddCtx(keys.Username, "root")
	nic := base.AddVector("NIC")
	_ = nic.AddPair("NIC_ID", "0")
	_ = nic.AddPair("IP", "185.66.71.171")
	_ = nic.AddPair("MAC", "02:00:b9:42:47:ab")
	_ = nic.AddPair("GATEWAY", "185.66.71.129")
	_ = nic.AddPair("NETWORK_MASK", "255.255.255.128")

	conf := reinstallContext(base, "root", "")
	if conf == nil {
		t.Fatal("context is empty")
	}
	got := conf.String()
	for _, want := range []string{`ETH0_IP="185.66.71.171"`, `ETH0_MAC="02:00:b9:42:47:ab"`, `ETH0_GATEWAY="185.66.71.129"`, `ETH0_MASK="255.255.255.128"`, `USERNAME="root"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %s in %s", want, got)
		}
	}
}

func TestDiskFits(t *testing.T) {
	if err := diskFits(2048, 10240); err != nil {
		t.Fatal(err)
	}
	if err := diskFits(20480, 10240); err == nil {
		t.Fatal("expected image larger than the disk to be refused")
	}
	if err := diskFits(0, 10240); err != nil {
		t.Fatal(err)
	}
}

func TestUsernameOK(t *testing.T) {
	if !usernameOK("root") || !usernameOK("Administrator") {
		t.Fatal("linux and windows logins are accepted")
	}
	if usernameOK("") || usernameOK("root;reboot") || usernameOK("a b") {
		t.Fatal("empty and unsafe names are refused")
	}
}
