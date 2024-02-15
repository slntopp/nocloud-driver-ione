package ansible_config

import "github.com/slntopp/nocloud-proto/ansible"

type AnsibleConfig struct {
	Vars   map[string]string `yaml:"vars"`
	SshKey string            `yaml:"sshKey"`
	Hop    ansible.Instance  `yaml:"hop"`
}
