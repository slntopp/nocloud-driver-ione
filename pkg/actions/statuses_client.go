package actions

import (
	amqp "github.com/rabbitmq/amqp091-go"
	one "github.com/slntopp/nocloud-driver-ione/pkg/driver"
	ipb "github.com/slntopp/nocloud/pkg/instances/proto"
	s "github.com/slntopp/nocloud/pkg/states"
	stpb "github.com/slntopp/nocloud/pkg/states/proto"
	"strings"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

var (
	log   *zap.Logger
	Pub   s.Pub
	SPPub s.Pub
)

func ConfigureStatusesClient(logger *zap.Logger, rbmq *amqp.Connection) {
	log = logger.Named("States")
	s := s.NewStatesPubSub(log, nil, rbmq)
	ch := s.Channel()
	s.TopicExchange(ch, "states")
	Pub = s.Publisher(ch, "states", "instances")
	SPPub = s.Publisher(ch, "states", "sp")
}

var STATES_REF = map[int32]stpb.NoCloudState{
	0:  stpb.NoCloudState_INIT,      // INIT
	1:  stpb.NoCloudState_INIT,      // PENDING
	2:  stpb.NoCloudState_INIT,      // HOLD
	4:  stpb.NoCloudState_STOPPED,   // STOPPED
	5:  stpb.NoCloudState_SUSPENDED, // SUSPENDED
	6:  stpb.NoCloudState_DELETED,   // DONE
	8:  stpb.NoCloudState_STOPPED,   // POWEROFF
	9:  stpb.NoCloudState_INIT,      // UNDEPLOYED
	10: stpb.NoCloudState_OPERATION, // CLONING
	11: stpb.NoCloudState_FAILURE,   // CLONING_FAILURE
}

var LCM_STATE_REF = map[int32]stpb.NoCloudState{
	0: stpb.NoCloudState_INIT,    // INIT
	1: stpb.NoCloudState_INIT,    // PENDING
	2: stpb.NoCloudState_INIT,    // HOLD
	3: stpb.NoCloudState_RUNNING, // RUNNING
}

// Returns the VM state of the VirtualMachine to statuses server
func StatusesClient(
	client one.IClient,
	inst *ipb.Instance,
	data map[string]*structpb.Value,
	result *ipb.InvokeResponse,
) (*ipb.InvokeResponse, error) {
	log.Debug("StatusesClient request received")

	par, err := State(client, inst, data)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Can't get State VM, error: %v", err)
	}

	result.Meta = par.Meta

	request := MakePostStateRequest(inst.GetUuid(), par.Meta)
	err = Pub(request)
	if err != nil {
		log.Error("Failed to post State", zap.Error(err))
	}

	return &ipb.InvokeResponse{Result: result.Result, Meta: result.Meta}, nil
}

func MakePostStateRequest(uuid string, meta map[string]*structpb.Value) *stpb.ObjectState {
	request := &stpb.ObjectState{
		Uuid: uuid,
		State: &stpb.State{
			State: stpb.NoCloudState_UNKNOWN,
			Meta:  meta,
		},
	}

	oneState := int32(meta["state"].GetNumberValue())
	oneLcmState := int32(meta["lcm_state"].GetNumberValue())

	res, ok := STATES_REF[oneState]
	if !ok {
		r, ok := LCM_STATE_REF[oneLcmState]
		if ok {
			request.State.State = r
			return request
		}

		if strings.HasSuffix(meta["lcm_state_str"].GetStringValue(), "FAILURE") {
			res = stpb.NoCloudState_FAILURE
		} else if strings.HasSuffix(meta["lcm_state_str"].GetStringValue(), "UNKNOWN") {
			res = stpb.NoCloudState_UNKNOWN
		} else {
			res = stpb.NoCloudState_OPERATION
		}
	}
	request.State.State = res
	return request
}
