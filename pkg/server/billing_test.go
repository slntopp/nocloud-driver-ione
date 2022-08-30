package server

import "testing"

func TestResourceKeyToDriveKind(t *testing.T) {
	type test struct {
		input string
		want  string
		fails bool
	}
	tests := []test{
		{input: "drive_ssd", want: "SSD"},
		{input: "drive_hdd", want: "HDD"},
		{input: "drive_nvme", want: "NVME"},
		{input: "drive_type_foo_bar_nvme", want: "NVME"},
		{input: "foo_nvme", want: "UNKNOWN"},
		{input: "drive", want: "UNKNOWN"},
	}
	for _, test := range tests {
		res, err := resourceKeyToDriveKind(test.input)
		if err != nil && !test.fails {
			t.Errorf("input: %s, wanted: %s, got: %v", test.input, test.want, err)
		}
		if res != test.want {
			t.Errorf("input: %s, wanted: %s, got: %s", test.input, test.want, res)
		}
	}
}
