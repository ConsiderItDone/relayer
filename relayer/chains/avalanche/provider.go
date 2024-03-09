package avalanche

import (
	"context"
	"fmt"
	"io"
	"math/big"
	"strings"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/validators"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	"github.com/ava-labs/subnet-evm/accounts/abi"
	"github.com/ava-labs/subnet-evm/core/types"
	"github.com/ava-labs/subnet-evm/rpc"
	"github.com/ava-labs/subnet-evm/tests/precompile/contract"
	"github.com/avast/retry-go/v4"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/gogoproto/proto"
	ibcexported "github.com/cosmos/ibc-go/v7/modules/core/exported"
	avalanche "github.com/cosmos/ibc-go/v7/modules/light-clients/14-avalanche"
	"github.com/ethereum/go-ethereum/crypto"
	"go.uber.org/zap"

	"github.com/ava-labs/subnet-evm/accounts/abi/bind"
	"github.com/ava-labs/subnet-evm/ethclient"
	"github.com/ava-labs/subnet-evm/ethclient/subnetevmclient"

	"github.com/cosmos/relayer/v2/relayer/provider"
)

var (
	_ provider.ChainProvider = &AvalancheProvider{}
	_ provider.KeyProvider   = &AvalancheProvider{}

	tempKey, _ = crypto.HexToECDSA("56289e99c94b6912bfc12adc093c9b51124f0dc54ac7a766b2bc5ccf558d8027")
)

// Variables used for retries
var (
	rtyAttNum = uint(5)
	rtyAtt    = retry.Attempts(rtyAttNum)
	rtyDel    = retry.Delay(time.Second * 1)
	rtyErr    = retry.LastErrorOnly(true)
)

type AvalancheIBCHeader struct {
	EthHeader  *types.Header
	Validators map[ids.NodeID]*validators.GetValidatorOutput
	//SignedHeader *tmtypes.SignedHeader
	//ValidatorSet *tmtypes.ValidatorSet
}

func (h AvalancheIBCHeader) Height() uint64 {
	return h.EthHeader.Number.Uint64()
}

func (h AvalancheIBCHeader) ConsensusState() ibcexported.ConsensusState {
	return &avalanche.ConsensusState{
		Timestamp:          time.Unix(int64(h.EthHeader.Time), 0),
		StorageRoot:        h.EthHeader.Root.Bytes(),
		SignedStorageRoot:  nil,
		ValidatorSet:       nil,
		SignedValidatorSet: nil,
		Vdrs:               nil,
		SignersInput:       nil,
	}
}

func (h AvalancheIBCHeader) NextValidatorsHash() []byte {
	return nil
}

type AvalancheProvider struct {
	log *zap.Logger

	PCfg           AvalancheProviderConfig
	Keybase        keyring.Keyring
	KeyringOptions []keyring.Option
	Input          io.Reader
	Output         io.Writer
	Codec          Codec

	ethClient    ethclient.Client
	ibcClient    IbcClient
	subnetClient *subnetevmclient.Client
	pClient      platformvm.Client
	txAuth       *bind.TransactOpts
	abi          abi.ABI
	subnetID     ids.ID
}

func (a *AvalancheProvider) Init(ctx context.Context) error {
	rpcClient, err := rpc.DialContext(context.Background(), a.PCfg.RPCAddr)
	if err != nil {
		return err
	}
	a.ethClient = ethclient.NewClient(rpcClient)
	a.subnetClient = subnetevmclient.New(rpcClient)
	a.pClient = platformvm.NewClient(a.PCfg.BaseRPCAddr)
	ibcClient, err := NewIbcClient(a.PCfg.RPCAddr)
	a.ibcClient = ibcClient

	chainId, ok := new(big.Int).SetString(a.PCfg.ChainID, 10)
	if !ok {
		return fmt.Errorf("invalid chain id %s", a.PCfg.ChainID)
	}

	a.txAuth, err = bind.NewKeyedTransactorWithChainID(tempKey, chainId)
	if err != nil {
		return err
	}

	keybase, err := keyring.New(a.PCfg.ChainID, a.PCfg.KeyringBackend, a.PCfg.KeyDirectory, a.Input, a.Codec.Marshaler, a.KeyringOptions...)
	if err != nil {
		return err
	}

	abi, err := abi.JSON(strings.NewReader(contract.ContractMetaData.ABI))
	if err != nil {
		return err
	}

	subnetID, err := ids.FromString(a.PCfg.SubnetID)
	if err != nil {
		return err
	}

	a.Keybase = keybase
	a.abi = abi
	a.subnetID = subnetID

	return nil
}

func (a AvalancheProvider) ChainName() string {
	return a.PCfg.ChainName
}

func (a AvalancheProvider) ChainId() string {
	return a.PCfg.ChainID
}

func (a AvalancheProvider) Type() string {
	return "avalanche"
}

func (a AvalancheProvider) ProviderConfig() provider.ProviderConfig {
	return a.PCfg
}

func (a AvalancheProvider) Key() string {
	return a.PCfg.Key
}

func (a AvalancheProvider) Address() (string, error) {
	info, err := a.Keybase.Key(a.PCfg.Key)
	if err != nil {
		return "", err
	}

	acc, err := info.GetAddress()
	if err != nil {
		return "", err
	}
	out := a.EncodeAccAddr(acc)

	return out, nil
}

func (a AvalancheProvider) Timeout() string {
	return a.PCfg.Timeout
}

func (a AvalancheProvider) TrustingPeriod(ctx context.Context) (time.Duration, error) {
	// TODO
	return time.Hour * 2, nil
}

func (a AvalancheProvider) WaitForNBlocks(ctx context.Context, n int64) error {
	var initial uint64
	// if avalanche node is not synced then nil is returned
	err := a.ethClient.SyncProgress(ctx)
	// node is syncing
	if err != nil {
		return fmt.Errorf("chain catching up")
	}

	latestBlockNumber, err := a.ethClient.BlockNumber(ctx)
	if err != nil {
		return err
	}
	initial = latestBlockNumber

	for {
		latestBlockNumber, err = a.ethClient.BlockNumber(ctx)
		if err != nil {
			return err
		}
		if latestBlockNumber > initial+uint64(n) {
			return nil
		}
		select {
		case <-time.After(10 * time.Millisecond):
			// Nothing to do.
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (a AvalancheProvider) BlockTime(ctx context.Context, height int64) (time.Time, error) {
	block, err := a.ethClient.BlockByNumber(ctx, big.NewInt(height))
	if err != nil {
		return time.Time{}, err
	}

	return time.Unix(int64(block.Time()), 0), nil
}

func (a *AvalancheProvider) Sprint(toPrint proto.Message) (string, error) {
	out, err := a.Codec.Marshaler.MarshalJSON(toPrint)
	if err != nil {
		return "", err
	}
	return string(out), nil
}
