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
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

// createUserCmd represents the createUser command
var createUserCmd = &cobra.Command{
	Use:   "create [login] [password] [group ID]",
	Short: "Create User",
	Args: cobra.MinimumNArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("createUser called")
		login  := args[0]
		passwd := args[1]
		group, err := strconv.Atoi(args[2])

		if err != nil {
			return err
		}

		user, err := client.CreateUser(login, passwd, []int{group})
		if err != nil {
			return err
		}

		fmt.Printf("User '%s' created. ID: %d\n", login, user)
		return nil
	},
}

func init() {
	userCmd.AddCommand(createUserCmd)
}
