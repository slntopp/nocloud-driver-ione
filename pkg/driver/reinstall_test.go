package one

import "testing"

func TestDiskFits(t *testing.T) {
	if err := diskFits(10240, 40960); err != nil {
		t.Fatal(err)
	}
	if err := diskFits(0, 40960); err != nil {
		t.Fatal(err)
	}
	if err := diskFits(51200, 40960); err == nil {
		t.Fatal("expected image larger than the instance disk to be rejected")
	}
}
