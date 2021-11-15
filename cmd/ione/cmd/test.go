/*
Copyright © 2021 NAME HERE <EMAIL ADDRESS>

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
	"errors"
	"fmt"

	ione "github.com/slntopp/nocloud-driver-ione/pkg/driver"
	"github.com/spf13/cobra"
)

// testCmd represents the test command
var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Test connection",
	RunE: func(cmd *cobra.Command, args []string) error {
		h, err := cmd.Flags().GetString("hostname")
		if err != nil {
			return err
		}

		u, err := cmd.Flags().GetString("username")
		if err != nil {
			return err
		}
		p, err := cmd.Flags().GetString("password")
		if err != nil {
			return err
		}

		if h == "" || u == "" || p == "" {
			return errors.New("Hostname, Username or Password not given")
		}

		c := ione.NewIONeClient(h, u + ":" + p)
		fmt.Printf("IONe resolved: %t\n", c.Ping())
		return nil
	},
}

func init() {
	rootCmd.AddCommand(testCmd)
}
