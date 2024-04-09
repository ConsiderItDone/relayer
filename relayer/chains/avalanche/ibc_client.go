package avalanche

import (
	"context"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/subnet-evm/rpc"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

var _ IbcClient = (*ibcClient)(nil)

type IbcClient interface {
	GetPChainHeight(ctx context.Context, evmHeight uint64) (uint64, error)
	GetMessageAggregateSignature(ctx context.Context, messageID ids.ID, quorumNum uint64, subnetIDStr string) ([]byte, error)
}

// ibcClient implementation for interacting with EVM [chain]
type ibcClient struct {
	client *rpc.Client
}

func NewIbcClient(uri string) (IbcClient, error) {
	client, err := rpc.Dial(uri)
	if err != nil {
		return nil, fmt.Errorf("failed to dial client. err: %w", err)
	}
	return &ibcClient{
		client: client,
	}, nil
}

func (c *ibcClient) GetPChainHeight(ctx context.Context, evmHeight uint64) (uint64, error) {
	var res uint64
	err := c.client.CallContext(ctx, &res, "ibc_getPChainHeight", evmHeight)
	if err != nil {
		return 0, err
	}

	return res, err
}

func (c *ibcClient) GetMessageAggregateSignature(ctx context.Context, messageID ids.ID, quorumNum uint64, subnetIDStr string) ([]byte, error) {
	var res hexutil.Bytes
	if err := c.client.CallContext(ctx, &res, "warp_getMessageAggregateSignature", messageID, quorumNum, subnetIDStr); err != nil {
		return nil, fmt.Errorf("call to warp_getMessageAggregateSignature failed. err: %w", err)
	}
	return res, nil
}
