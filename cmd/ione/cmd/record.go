/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"encoding/base64"
	"encoding/xml"

	"google.golang.org/protobuf/types/known/structpb"

	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/vm"
	"github.com/slntopp/nocloud-driver-ione/pkg/actions"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// recordCmd represents the record command
var recordCmd = &cobra.Command{
	Use:   "record",
	Short: "Sends VM State to NoCloud",
	RunE: func(cmd *cobra.Command, args []string) error {
		encoded := args[0]
		tmpl, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return err
		}
		vm := &vm.VM{}
		err = xml.Unmarshal(tmpl, vm)
		if err != nil {
			return err
		}
		
		st, lcm_st, err := vm.State()
		if err != nil {
			return err
		}

		state := int(st)
		state_str := st.String()
		lcm_state := int(lcm_st)
		lcm_state_str := lcm_st.String()

		log.Info("Storing Instance State", 
			zap.Int("vmid", vm.ID), zap.String("instance", vm.Name),
			zap.Int("state", state), zap.String("state_str", state_str),
			zap.Int("lcm_state", lcm_state), zap.String("lcm_state_str", lcm_state_str),
		)

		data, err := structpb.NewStruct(map[string]interface{}{
			"uuid":          vm.Name,
			"state":         state,
			"state_str":     state_str,
			"lcm_state":     lcm_state,
			"lcm_state_str": lcm_state_str,
		})
		if err != nil {
			return err
		}


		req := actions.MakePostStateRequest(vm.Name, data.GetFields())
		_, err = srvClient.PostState(ctx, req)
		return err
	},
}

func init() {
	rootCmd.AddCommand(recordCmd)
}
