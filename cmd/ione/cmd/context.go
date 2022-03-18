/*
Copyright © 2022 Nikita Ivanovski info@slnt-opp.xyz

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
	"go.uber.org/zap"
)

var (
	VERSION string
)

// contextCmd represents the context command
var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Print hook context(host, binary version)",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("context called")
		log.Info("Context", zap.String("host", host), zap.Bool("insecure", insecure), zap.String("version", VERSION))
	},
}

func init() {
	rootCmd.AddCommand(contextCmd)
}
