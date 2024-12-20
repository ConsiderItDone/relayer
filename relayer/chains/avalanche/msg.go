package avalanche

import (
	"github.com/cosmos/relayer/v2/relayer/provider"
)

type (
	EVMMessage struct {
		input   []byte
		msgType string
	}
)

const (
	Warp     = "Warp"
	Transfer = "Transfer"
)

func NewEVMMessage(input []byte, msgType string) provider.RelayerMessage {
	return EVMMessage{
		input:   input,
		msgType: msgType,
	}
}

func (em EVMMessage) Type() string {
	return em.msgType
}

func (em EVMMessage) MsgBytes() ([]byte, error) {
	return em.input, nil
}
