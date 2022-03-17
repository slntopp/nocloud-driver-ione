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
	"context"
	"fmt"
	"os"

	"github.com/slntopp/nocloud/pkg/nocloud"
	pb "github.com/slntopp/nocloud/pkg/services/proto"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/spf13/viper"
)

var (
	log *zap.Logger

	ctx context.Context
	srvClient pb.ServicesServiceClient
)
	
// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "ione",
	Short: "A brief description of your application",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	cobra.OnInitialize(initConfig)
	log = nocloud.NewLogger()
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	viper.AddConfigPath("/etc/one")
	viper.SetConfigType("yaml")
	viper.SetConfigName("ione")

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}

	conn, err := grpc.Dial("localhost:8080")
	if err != nil {
		panic(err)
	}

	ctx = context.Background()
	srvClient = pb.NewServicesServiceClient(conn)
}
