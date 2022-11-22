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
	pb "github.com/slntopp/nocloud-proto/edge"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// testCmd represents the test command
var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Tests Current configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := srvClient.Test(ctx, &pb.TestRequest{})
		if err != nil {
			return err
		}
		log.Info("Test Response", zap.Bool("result", r.GetResult()), zap.String("desc", r.GetError()))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(testCmd)
}
