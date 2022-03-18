/*
Copyright © 2021-2022 Nikita Ivanovski info@slnt-opp.xyz

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
	"crypto/tls"
	"fmt"
	"os"

	pb "github.com/slntopp/nocloud/pkg/edge/proto"
	"github.com/slntopp/nocloud/pkg/nocloud"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	ins "google.golang.org/grpc/credentials/insecure"

	"github.com/spf13/viper"
)

var (
	log *zap.Logger

	ctx context.Context
	srvClient pb.EdgeServiceClient

	host string
	insecure bool
)
	
// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "nocloud-ione",
	Short: "Configure IONe to NoCloud bind",
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute(version string) {
	VERSION = version
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	log = nocloud.NewLogger()
	cobra.OnInitialize(initConfig)
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	log.Debug("Reading Config")
	viper.AddConfigPath("/etc/one")
	viper.SetConfigType("yaml")
	viper.SetConfigName("ione")

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	} else {
		log.Fatal("Error reading Config", zap.Error(err))
	}

	host = viper.GetString("host")
	cred := credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})
	if insecure = viper.GetBool("insecure"); insecure {
		cred = ins.NewCredentials()
	}
	conn, err := grpc.Dial(host, grpc.WithTransportCredentials(cred))
	if err != nil {
		panic(err)
	}

	ctx = context.Background()
	srvClient = pb.NewEdgeServiceClient(conn)
}
