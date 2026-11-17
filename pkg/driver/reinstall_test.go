package one

import "testing"

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
