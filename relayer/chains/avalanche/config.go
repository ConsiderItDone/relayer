package avalanche

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"time"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/types/module"
	"go.uber.org/zap"

	"github.com/cosmos/relayer/v2/relayer/codecs/ethermint"
	"github.com/cosmos/relayer/v2/relayer/provider"
)

var _ provider.ProviderConfig = &AvalancheProviderConfig{}

type AvalancheProviderConfig struct {
	RPCAddr         string                  `json:"rpc-addr" yaml:"rpc-addr"`
	BaseRPCAddr     string                  `json:"base-rpc-addr" yaml:"base-rpc-addr"`
	SubnetID        string                  `json:"subnet-id" yaml:"subnet-id"`
	ChainID         string                  `json:"chain-id" yaml:"chain-id"`
	ChainName       string                  `json:"-" yaml:"-"`
	Timeout         string                  `json:"timeout" yaml:"timeout"`
	ContractAddress string                  `json:"contract-address" yaml:"contract-address"`
	KeyDirectory    string                  `json:"key-directory" yaml:"key-directory"`
	Key             string                  `json:"key" yaml:"key"`
	KeyringBackend  string                  `json:"keyring-backend" yaml:"keyring-backend"`
	ExtraCodecs     []string                `json:"extra-codecs" yaml:"extra-codecs"`
	Modules         []module.AppModuleBasic `json:"-" yaml:"-"`
	MinLoopDuration time.Duration           `json:"min-loop-duration" yaml:"min-loop-duration"`
}

func (ac AvalancheProviderConfig) NewProvider(log *zap.Logger, homepath string, debug bool, chainName string) (provider.ChainProvider, error) {
	if err := ac.Validate(); err != nil {
		return nil, err
	}

	ac.KeyDirectory = keysDir(homepath, fmt.Sprintf("avalanche-%s", ac.ChainID))
	ac.ChainName = chainName

	ac.Modules = append([]module.AppModuleBasic{}, moduleBasics...)

	return &AvalancheProvider{
		log:            log,
		PCfg:           ac,
		KeyringOptions: []keyring.Option{ethermint.EthSecp256k1Option()},
		Input:          os.Stdin,
		Output:         os.Stdout,

		Codec: makeCodec(ac.Modules, ac.ExtraCodecs),
	}, nil
}

func (ac AvalancheProviderConfig) Validate() error {
	_, err := url.Parse(ac.RPCAddr)
	if err != nil {
		return err
	}

	_, err = url.Parse(ac.BaseRPCAddr)
	if err != nil {
		return err
	}

	return nil
}

func (ac AvalancheProviderConfig) BroadcastMode() provider.BroadcastMode {
	return provider.BroadcastModeSingle
}

// keysDir returns a string representing the path on the local filesystem where the keystore will be initialized.
func keysDir(home, chainID string) string {
	return path.Join(home, "keys", chainID)
}
