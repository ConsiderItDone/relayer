package avalanche

import (
	"context"
	"math/big"
	"net/url"
	"strings"
	"time"

	"github.com/ava-labs/subnet-evm/accounts/abi"
	"github.com/ava-labs/subnet-evm/rpc"
	"github.com/ava-labs/subnet-evm/tests/precompile/contract"
	"github.com/avast/retry-go/v4"
	"github.com/cosmos/gogoproto/proto"
	"github.com/ethereum/go-ethereum/crypto"
	"go.uber.org/zap"

	"github.com/ava-labs/subnet-evm/accounts/abi/bind"
	"github.com/ava-labs/subnet-evm/ethclient"
	"github.com/ava-labs/subnet-evm/ethclient/subnetevmclient"

	"github.com/cosmos/relayer/v2/relayer/provider"
)

var (
	_ provider.ChainProvider  = &AvalancheProvider{}
	_ provider.KeyProvider    = &AvalancheProvider{}
	_ provider.ProviderConfig = &AvalancheProviderConfig{}

	tempKey, _ = crypto.HexToECDSA("56289e99c94b6912bfc12adc093c9b51124f0dc54ac7a766b2bc5ccf558d8027")
)

// Variables used for retries
var (
	rtyAttNum = uint(5)
	rtyAtt    = retry.Attempts(rtyAttNum)
	rtyDel    = retry.Delay(time.Second * 1)
	rtyErr    = retry.LastErrorOnly(true)
)

type AvalancheProviderConfig struct {
	RPCAddr         string `json:"rpc-addr" yaml:"rpc-addr"`
	ChainID         string `json:"chain-id" yaml:"chain-id"`
	ChainName       string `json:"-" yaml:"-"`
	Timeout         string `json:"timeout" yaml:"timeout"`
	ContractAddress string `json:"contract-address" yaml:"contract-address"`
}

func (ac AvalancheProviderConfig) NewProvider(log *zap.Logger, homepath string, debug bool, chainName string) (provider.ChainProvider, error) {
	if err := ac.Validate(); err != nil {
		return nil, err
	}
	ac.ChainName = chainName

	return &AvalancheProvider{
		log:  log,
		PCfg: ac,
	}, nil
}

func (ac AvalancheProviderConfig) Validate() error {
	_, err := url.Parse(ac.RPCAddr)
	if err != nil {
		return err
	}

	return nil
}

func (ac AvalancheProviderConfig) BroadcastMode() provider.BroadcastMode {
	return provider.BroadcastModeSingle
}

type AvalancheProvider struct {
	log *zap.Logger

	PCfg AvalancheProviderConfig

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

	a.abi, err = abi.JSON(strings.NewReader(contract.ContractMetaData.ABI))
	if err != nil {
		return err
	}

	return nil
}

func (a AvalancheProvider) Sprint(toPrint proto.Message) (string, error) {
	return toPrint.String(), nil
}
