package avalanche

import (
	"github.com/cosmos/relayer/v2/relayer/provider"
)

type EVMMessage struct {
	input []byte
}

func NewEVMMessage(input []byte) provider.RelayerMessage {
	return EVMMessage{
		input: input,
	}
}

func (em EVMMessage) Type() string {
	return ""
}

func (em EVMMessage) MsgBytes() ([]byte, error) {
	return em.input, nil
}
