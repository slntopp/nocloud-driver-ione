package one

import (
	"errors"
	"strings"

	goca "github.com/OpenNebula/one/src/oca/go/src/goca"
	"go.uber.org/zap"
)

type ONeClient struct {
	*goca.Client
	ctrl *goca.Controller
	log *zap.Logger

	vars map[string]*sppb.Var
}

func NewClient(user, password, endpoint string, log *zap.Logger) *ONeClient {
	conf := goca.NewConfig(user, password, endpoint)
	c    := goca.NewClient(conf, nil)
	ctrl := goca.NewController(c)
	return &ONeClient{
		Client: c,
		ctrl: ctrl,
		log: log.Named("ONeClient"),
	}
}

func NewClientFromSP(sp *sppb.ServicesProvider, log *zap.Logger) (*ONeClient, error) {
	secrets := sp.GetSecrets()
	host  := secrets["host"].GetStringValue()
	cred  := secrets["cred"].GetStringValue()
	if host == "" || cred == "" {
		return nil, errors.New("Host or Credentials are empty")
	}
	var user, pass string
	{
		cred := strings.Split(cred, ":")
		user = cred[0]
		pass = cred[1]
	}
	return NewClient(host, user, pass, log), nil
}
