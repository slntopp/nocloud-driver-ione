package main

import (
	"encoding/base64"
	"net/http"
	"os"

	"github.com/OpenNebula/one/src/oca/go/src/goca"
	"github.com/slntopp/nocloud/pkg/nocloud"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var (
	log                                                          *zap.Logger
	vnc_tokens_dir, vmrc_tokens_dir, vmrc_endpoint, vnc_endpoint string
)

func init() {
	viper.AutomaticEnv()
	log = nocloud.NewLogger()

	viper.SetDefault("SUNSTONE_VNC_TOKENS_DIR", "/var/lib/one/sunstone_vnc_tokens/")
	viper.SetDefault("SUNSTONE_VMRC_TOKENS_DIR", "/var/lib/one/sunstone_vmrc_tokens/")
	viper.SetDefault("SOCKET_VMRC_ENDPOINT", "wss://sunstone.demo.support.pl/fireedge/vmrc/")
	viper.SetDefault("SOCKET_VNC_ENDPOINT", "wss://sunstone.demo.support.pl:29876")

	vnc_tokens_dir = viper.GetString("SUNSTONE_VNC_TOKENS_DIR")
	vmrc_tokens_dir = viper.GetString("SUNSTONE_VMRC_TOKENS_DIR")
	vmrc_endpoint = viper.GetString("SOCKET_VMRC_ENDPOINT")
	vnc_endpoint = viper.GetString("SOCKET_VNC_ENDPOINT")
}

func main() {
	if len(os.Args) >= 3 {
		if os.Args[1] != "test" {
			log.Fatal("Unrecognized argument", zap.String("arg", os.Args[1]))
		}
		kind := "vnc"
		if len(os.Args) == 4 {
			kind = os.Args[3]
		}
		if kind != "vnc" && kind != "vmrc" {
			log.Fatal("Unsupported kind", zap.String("kind", kind))
		}
		c := goca.NewController(goca.NewDefaultClient(goca.NewConfig("oneadmin", "72cf91142b4593541993b02b9bfc22a9d4666afd09f95b1f268f4204dee3b1c4", "https://sunstone.demo.support.pl/RPC2")))
		d := GenToken(c, os.Args[2], kind)
		ui := base64.StdEncoding.EncodeToString([]byte(d.Url))
		log.Info("Resulting URL", zap.String("url", d.Url), zap.String("ui", "https://sunstone.demo.support.pl/vnc?socket="+ui))
		return
	}

	http.HandleFunc("/", HandleGenerateToken)
	http.ListenAndServe(":8010", nil)
}
