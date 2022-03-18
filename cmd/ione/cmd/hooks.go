/*
Copyright © 2022 Nikita Ivanovski info@slnt-opp.xyz

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/OpenNebula/one/src/oca/go/src/goca"
	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/hook"
	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/hook/keys"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var hooks = []map[string]string{
	{
    "NAME": "nocloud.pending",
    "ON": "CUSTOM",
    "STATE": "PENDING",
    "LCM_STATE": "LCM_INIT",
    "ARGUMENTS": "record $TEMPLATE",
    "TYPE": "state",
    "RESOURCE": "VM",
  },
  {
    "NAME": "nocloud.hold",
    "ON": "CUSTOM",
    "STATE": "HOLD",
    "LCM_STATE": "LCM_INIT",
    "ARGUMENTS": "record $TEMPLATE",
    "TYPE": "state",
    "RESOURCE": "VM",
  },
  {
    "NAME": "nocloud.active-boot",
    "ON": "CUSTOM",
    "STATE": "ACTIVE",
    "LCM_STATE": "BOOT",
    "ARGUMENTS": "record $TEMPLATE",
    "TYPE": "state",
    "RESOURCE": "VM",
  },
  {
    "NAME": "nocloud.active-running",
    "ON": "CUSTOM",
    "STATE": "ACTIVE",
    "LCM_STATE": "RUNNING",
    "ARGUMENTS": "record $TEMPLATE",
    "TYPE": "state",
    "RESOURCE": "VM",
  },
  {
    "NAME": "nocloud.inactive-stopped",
    "ON": "CUSTOM",
    "STATE": "STOPPED",
    "LCM_STATE": "LCM_INIT",
    "ARGUMENTS": "record $TEMPLATE",
    "TYPE": "state",
    "RESOURCE": "VM",
  },
  {
    "NAME": "nocloud.inactive-suspended",
    "ON": "CUSTOM",
    "STATE": "SUSPENDED",
    "LCM_STATE": "LCM_INIT",
    "ARGUMENTS": "record $TEMPLATE",
    "TYPE": "state",
    "RESOURCE": "VM",
  },
  {
    "NAME": "nocloud.inactive-done",
    "ON": "CUSTOM",
    "STATE": "DONE",
    "LCM_STATE": "LCM_INIT",
    "ARGUMENTS": "record $TEMPLATE",
    "TYPE": "state",
    "RESOURCE": "VM",
  },
  {
    "NAME": "nocloud.inactive-poweroff",
    "ON": "CUSTOM",
    "STATE": "POWEROFF",
    "LCM_STATE": "LCM_INIT",
    "ARGUMENTS": "record $TEMPLATE",
    "TYPE": "state",
    "RESOURCE": "VM",
  },
}

// hooksCmd represents the hooks command
var hooksCmd = &cobra.Command{
	Use:   "hooks",
	Short: "Set Up OpenNebula hooks",
	RunE: func(cmd *cobra.Command, args []string) error {
		cred, err := GetAuth()
		if err != nil {
			return err
		}
		rpc, err := GetEndpoint()
		if err != nil {
			return err
		}

		ex, err := os.Executable()
		if err != nil {
			panic(err)
		}
		exPath := filepath.Dir(ex)

		log.Debug("Command context", zap.String("token", cred), zap.String("endpoint", rpc), zap.String("binary_path", exPath))

		client := goca.NewClient(goca.OneConfig{Token: cred, Endpoint: rpc}, nil)
		ctrl := goca.NewController(client)

		hkc := ctrl.Hooks()
		
		pool, err := hkc.Info()
		if err != nil {
			return err
		}

		for _, h := range pool.Hooks {
			if strings.HasPrefix(h.Name, "nocloud.") {
				ctrl.Hook(h.ID).Delete()
			}
		}

		for _, conf := range hooks {
			h := hook.NewTemplate()
			for k, v := range conf {
				h.AddPair(k, v)
			}
			h.Add(keys.Command, exPath)
			id, err := hkc.Create(h.String())
			log.Info("Create Hook attempt", zap.Int("id", id), zap.Error(err))
		}

		return nil
	},
}

var hooksCleanupCmd = &cobra.Command{
	Use: "cleanup",
	Short: "Remove NoCloud Hooks",
	RunE: func(cmd *cobra.Command, args []string) error {
		cred, err := GetAuth()
		if err != nil {
			return err
		}
		rpc, err := GetEndpoint()
		if err != nil {
			return err
		}

		log.Debug("Command context", zap.String("token", cred), zap.String("endpoint", rpc))

		client := goca.NewClient(goca.OneConfig{Token: cred, Endpoint: rpc}, nil)
		ctrl := goca.NewController(client)

		hkc := ctrl.Hooks()
		
		pool, err := hkc.Info()
		if err != nil {
			return err
		}

		for _, h := range pool.Hooks {
			if strings.HasPrefix(h.Name, "nocloud.") {
				ctrl.Hook(h.ID).Delete()
			}
		}

		return nil
	},
}

func GetAuth() (string, error) {
	cred := viper.GetString("ONE_CREDENTIALS")
	if cred != "" {
		return cred, nil
	}

	path := viper.GetString("ONE_AUTH")
	if path != "" {
		bytes, err := os.ReadFile(path)
		return string(bytes), err
	}

	bytes, err := os.ReadFile("/var/lib/one/.one/one_auth")
	return string(bytes), err
}

func GetEndpoint() (string, error) {
	endpoint := viper.GetString("ONE_ENDPOINT")
	if endpoint != "" {
		return endpoint, nil
	}

	endpoint = viper.GetString("ONE_XMLRPC")
	if endpoint != "" {
		return endpoint, nil
	}

	bytes, err := os.ReadFile("/var/lib/one/.one/one_endpoint")
	if err == nil {
		return string(bytes), nil
	}

	return "http://localhost:2633/RPC2", nil
}

func init() {
	hooksCmd.AddCommand(hooksCleanupCmd)
	rootCmd.AddCommand(hooksCmd)
}
