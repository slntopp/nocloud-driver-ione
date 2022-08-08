package main

import (
	"net/http"

	"github.com/slntopp/nocloud/pkg/nocloud"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var (
	log                                                          *zap.Logger
	vnc_tokens_dir, vmrc_tokens_dir, vmrc_endpoint, vnc_endpoint string
)

func init() {
	viper.AddConfigPath("/etc/one")
	viper.SetConfigType("yaml")
	viper.SetConfigName("ione")

	viper.AutomaticEnv()
	log = nocloud.NewLogger()

	viper.SetDefault("SUNSTONE_VNC_TOKENS_DIR", "/var/lib/one/sunstone_vnc_tokens/")
	viper.SetDefault("SUNSTONE_VMRC_TOKENS_DIR", "/var/lib/one/sunstone_vmrc_tokens/")
	viper.SetDefault("SOCKET_VMRC_ENDPOINT", "ws://localhost/fireedge/vmrc/")
	viper.SetDefault("SOCKET_VNC_ENDPOINT", "ws://localhost:29876")

	if err := viper.ReadInConfig(); err == nil {
		log.Info("Using config file", zap.String("file", viper.ConfigFileUsed()))
	} else {
		log.Fatal("Error reading Config", zap.Error(err))
	}

	vnc_tokens_dir = viper.GetString("SUNSTONE_VNC_TOKENS_DIR")
	vmrc_tokens_dir = viper.GetString("SUNSTONE_VMRC_TOKENS_DIR")
	vmrc_endpoint = viper.GetString("SOCKET_VMRC_ENDPOINT")
	vnc_endpoint = viper.GetString("SOCKET_VNC_ENDPOINT")
}

func main() {
	http.HandleFunc("/", HandleGenerateToken)
	http.ListenAndServe(":8010", nil)
}
