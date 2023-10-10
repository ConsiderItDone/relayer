package avalanche

import (
	"github.com/cosmos/relayer/v2/relayer/provider"
)

func (a AvalancheProvider) CreateKeystore(path string) error {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) KeystoreCreated(path string) bool {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) AddKey(name string, coinType uint32, signingAlgorithm string) (output *provider.KeyOutput, err error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) RestoreKey(name, mnemonic string, coinType uint32, signingAlgorithm string) (address string, err error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) ShowAddress(name string) (address string, err error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) ListAddresses() (map[string]string, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) DeleteKey(name string) error {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) KeyExists(name string) bool {
	// TODO
	return true
}

func (a AvalancheProvider) ExportPrivKeyArmor(keyName string) (armor string, err error) {
	//TODO implement me
	panic("implement me")
}
