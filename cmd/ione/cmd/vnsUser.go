/*
Copyright © 2021 Nikita Ivanovski info@slnt-opp.xyz

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
	"fmt"

	"github.com/spf13/cobra"
)

// vnsUserCmd represents the vnsUser command
var vnsUserCmd = &cobra.Command{
	Use:   "vns",
	Short: "List VNs",
	RunE: func(cmd *cobra.Command, args []string) error {
		res, err := client.GetUserNetworks()
		if err != nil {
			return err
		}

		fmt.Println(res)
		return nil
	},
}

func init() {
	userCmd.AddCommand(vnsUserCmd)
}
