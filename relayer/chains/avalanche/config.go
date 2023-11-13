package avalanche

import (
	"net/url"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"go.uber.org/zap"

	"github.com/cosmos/relayer/v2/relayer/codecs/ethermint"
	"github.com/cosmos/relayer/v2/relayer/provider"
)

var _ provider.ProviderConfig = &AvalancheProviderConfig{}

type AvalancheProviderConfig struct {
	RPCAddr         string `json:"rpc-addr" yaml:"rpc-addr"`
	ChainID         string `json:"chain-id" yaml:"chain-id"`
	ChainName       string `json:"-" yaml:"-"`
	Timeout         string `json:"timeout" yaml:"timeout"`
	ContractAddress string `json:"contract-address" yaml:"contract-address"`
	KeyDirectory    string `json:"key-directory" yaml:"key-directory"`
	Key             string `json:"key" yaml:"key"`
	KeyringBackend  string `json:"keyring-backend" yaml:"keyring-backend"`
	AccountPrefix   string `json:"account-prefix" yaml:"account-prefix"`
}

func (ac AvalancheProviderConfig) NewProvider(log *zap.Logger, homepath string, debug bool, chainName string) (provider.ChainProvider, error) {
	if err := ac.Validate(); err != nil {
		return nil, err
	}
	ac.ChainName = chainName

	return &AvalancheProvider{
		log:            log,
		PCfg:           ac,
		KeyringOptions: []keyring.Option{ethermint.EthSecp256k1Option()},
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
