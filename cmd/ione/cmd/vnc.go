package cmd

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strconv"

	"github.com/OpenNebula/one/src/oca/go/src/goca"
	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/vm/keys"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func GenerateToken() string {
	n, err := rand.Prime(rand.Reader, 105)
	if err != nil {
		log.Fatal("Error generating Prime number", zap.Error(err))
	}

	s := n.Text(36)
	return s
}

var genGraphicsTokenCmd = &cobra.Command{
	Use:   "graphics [vnc|vmrc] [vmid]",
	Short: "Generate Graphics(VNC/VMRC) token",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		kind := args[0]
		vmid := args[1]
		log.Debug("Testing generating VNC token", zap.String("vmid", vmid))

		c := goca.NewController(goca.NewDefaultClient(goca.NewConfig("", "", "")))
		var vmc *goca.VMController
		if id, err := strconv.Atoi(vmid); err != nil {
			log.Fatal("Error converting vmid", zap.Error(err))
		} else {
			vmc = c.VM(id)
		}

		vm, err := vmc.Info(false)
		if err != nil {
			log.Fatal("Error getting VM", zap.Error(err))
		}

		gt, err := vm.Template.GetIOGraphic(keys.GraphicType)
		if err != nil {
			log.Fatal("Error getting GraphicType", zap.Error(err))
		}

		if gt != "VNC" && gt != "SPICE" {
			log.Fatal("Graphics Type is not VNC nor Spice", zap.String("type", gt))
		}

		host := vm.HistoryRecords[len(vm.HistoryRecords)-1].Hostname

		if h, _ := vm.UserTemplate.GetStr("HYPERVISOR"); h == "vcenter" {
			if esx, err := vm.MonitoringInfos.GetStr("VCENTER_ESX_HOST"); err != nil {
				log.Fatal("Error getting HYPERVISOR", zap.Error(err))
			} else {
				host = esx
			}
		} else if kind == "vmrc" {
			log.Fatal("VMRC specified, but vm is not vcenter")
		}

		log.Debug("Last HistoryRecord Host", zap.String("host", host))
		port, err := vm.Template.GetIOGraphic(keys.Port)
		if err != nil {
			log.Fatal("Error getting PORT", zap.Error(err))
		}

		info := map[string]interface{}{
			"id":         vm.ID,
			"name":       vm.Name,
			"start_time": strconv.Itoa(vm.STime),
		}

		_, state, _ := vm.StateString()
		info["state"] = state
		info["networks"] = []string{}

		infob, _ := json.Marshal(info)
		log.Debug("info", zap.ByteString("json", infob))

		info64 := base64.StdEncoding.EncodeToString(infob)
		log.Debug("Socket", zap.String("info64", info64))

		token := GenerateToken()

		var filename string
		var token_file string
		var url string
		if kind == "vnc" {
			filename = path.Join(vnc_tokens_dir, fmt.Sprintf("one-%d", vm.ID))
			token_file = fmt.Sprintf("%s: %s:%s", token, host, port)
			url = fmt.Sprintf("%s?encrypt=true&password=null&token=%s&info=%s", vnc_endpoint, token, info64)
		} else if kind == "vmrc" {
			filename = path.Join(vmrc_tokens_dir, token)
			token_file = fmt.Sprintf("%s:%s", host, port)
			url = fmt.Sprintf("%s%s?info=%s", vmrc_endpoint, token, info64)
		} else {
			return
		}

		log.Debug("Resulting vnc token", zap.String("token", token), zap.String("file", token_file))

		err = os.WriteFile(
			filename, []byte(token_file), 0644,
		)
		if err != nil {
			log.Fatal("error writing file", zap.Error(err))
		}

		log.Info("Resulting URL", zap.String("url", url))
	},
}

func init() {
	rootCmd.AddCommand(genGraphicsTokenCmd)
}
