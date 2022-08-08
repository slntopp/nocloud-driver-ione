package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"strconv"

	"github.com/OpenNebula/one/src/oca/go/src/goca"
	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/vm/keys"
	"go.uber.org/zap"
)

func GenerateToken() string {
	n, err := rand.Prime(rand.Reader, 105)
	if err != nil {
		log.Fatal("Error generating Prime number", zap.Error(err))
	}

	s := n.Text(36)
	log.Info("", zap.String("s", s), zap.Int("len", len(s)))
	return s
}

type TokenData struct {
	Token string `json:"token"`
	Info  string `json:"info"`
	Url   string `json:"url"`
}

func GenToken(c *goca.Controller, vmid, kind string) *TokenData {
	log.Info("Testing generating VNC token", zap.String("vmid", vmid))

	var vmc *goca.VMController
	if id, err := strconv.Atoi(vmid); err != nil {
		log.Error("Error converting vmid", zap.Error(err))
		return nil
	} else {
		vmc = c.VM(id)
	}

	vm, err := vmc.Info(false)
	if err != nil {
		log.Error("Error getting VM", zap.Error(err))
		return nil
	}

	gt, err := vm.Template.GetIOGraphic(keys.GraphicType)
	if err != nil {
		log.Error("Error getting GraphicType", zap.Error(err))
		return nil
	}

	if gt != "VNC" && gt != "SPICE" {
		log.Error("Graphics Type is not VNC nor Spice", zap.String("type", gt))
		return nil
	}

	host := vm.HistoryRecords[len(vm.HistoryRecords)-1].Hostname

	if h, _ := vm.UserTemplate.GetStr("HYPERVISOR"); h == "vcenter" {
		if esx, err := vm.MonitoringInfos.GetStr("VCENTER_ESX_HOST"); err != nil {
			log.Error("Error getting HYPERVISOR", zap.Error(err))
			return nil
		} else {
			host = esx
		}
	} else if kind == "vmrc" {
		log.Error("VMRC specified, but vm is not vcenter")
		return nil
	}

	log.Info("Last HistoryRecord Host", zap.String("host", host))
	port, err := vm.Template.GetIOGraphic(keys.Port)
	if err != nil {
		log.Error("Error getting PORT", zap.Error(err))
		return nil
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
	log.Info("info", zap.ByteString("json", infob))

	info64 := base64.StdEncoding.EncodeToString(infob)
	log.Info("Socket", zap.String("info64", info64))

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
		return nil
	}

	log.Info("Resulting vnc token", zap.String("token", token), zap.String("file", token_file))

	err = os.WriteFile(
		filename, []byte(token_file), 0644,
	)
	if err != nil {
		log.Error("error writing file", zap.Error(err))
		return nil
	}

	return &TokenData{
		Token: token,
		Info:  info64,
		Url:   url,
	}
}

func HandleGenerateToken(w http.ResponseWriter, req *http.Request) {

	user, pass, ok := req.BasicAuth()

	if !ok {
		w.WriteHeader(401)
		return
	}

	fmt.Println(user, pass)

	c := goca.NewController(
		goca.NewDefaultClient(goca.NewConfig(user, pass, "")),
	)

	query := req.URL.Query()
	if !query.Has("kind") || !query.Has("vmid") {
		w.WriteHeader(400)
		w.Write([]byte("Graphics Kind and/or VM ID aren't present"))
		return
	}

	kind, vmid := query.Get("kind"), query.Get("vmid")

	if kind != "vnc" && kind != "vmrc" {
		w.WriteHeader(400)
		w.Write([]byte(fmt.Sprintf("Kind '%s' is not supported", kind)))
		return
	}

	d := GenToken(c, vmid, kind)
	if d == nil {
		w.WriteHeader(400)
		w.Write([]byte("Something went wrong, check your inputs"))
		return
	}

	data, err := json.Marshal(d)
	if err != nil {
		log.Warn("Error Marshaling result", zap.Any("data", d), zap.Error(err))
		w.WriteHeader(500)
		w.Write([]byte("Error encoding result data"))
		return
	}

	w.Header().Add("Content-type", "application/json")
	w.Write(data)

	fmt.Println(kind, vmid)
}
