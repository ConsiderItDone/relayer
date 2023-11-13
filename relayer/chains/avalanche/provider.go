package avalanche

import (
	"context"
	"io"
	"math/big"
	"strings"
	"time"

	"github.com/ava-labs/subnet-evm/accounts/abi"
	"github.com/ava-labs/subnet-evm/rpc"
	"github.com/ava-labs/subnet-evm/tests/precompile/contract"
	"github.com/avast/retry-go/v4"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/gogoproto/proto"
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

type AvalancheProvider struct {
	log *zap.Logger

	PCfg           AvalancheProviderConfig
	Keybase        keyring.Keyring
	KeyringOptions []keyring.Option
	Input          io.Reader
	Output         io.Writer
	Codec          Codec

	ethClient    ethclient.Client
	subnetClient *subnetevmclient.Client
	txAuth       *bind.TransactOpts
	abi          abi.ABI
}

func (a *AvalancheProvider) Init(ctx context.Context) error {
	rpcClient, err := rpc.DialContext(context.Background(), a.PCfg.RPCAddr)
	if err != nil {
		return err
	}
	a.ethClient = ethclient.NewClient(rpcClient)
	a.subnetClient = subnetevmclient.New(rpcClient)

	chainId, _ := new(big.Int).SetString(a.PCfg.ChainID, 10)

	a.txAuth, err = bind.NewKeyedTransactorWithChainID(tempKey, chainId)

	keybase, err := keyring.New(a.PCfg.ChainID, a.PCfg.KeyringBackend, a.PCfg.KeyDirectory, a.Input, a.Codec.Marshaler, a.KeyringOptions...)

	abi, err := abi.JSON(strings.NewReader(contract.ContractMetaData.ABI))
	if err != nil {
		return err
	}

	a.Keybase = keybase
	a.abi = abi

	return nil
}

func (a *AvalancheProvider) Sprint(toPrint proto.Message) (string, error) {
	out, err := a.Codec.Marshaler.MarshalJSON(toPrint)
	if err != nil {
		return "", err
	}
	return string(out), nil
}
