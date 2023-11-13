package avalanche

import (
	"errors"
	"os"

	ckeys "github.com/cosmos/cosmos-sdk/client/keys"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/go-bip39"

	"github.com/cosmos/relayer/v2/relayer/codecs/ethermint"
	"github.com/cosmos/relayer/v2/relayer/provider"
)

const ethereumCoinType = uint32(60)

// CreateKeystore initializes a new instance of a keyring at the specified path in the local filesystem.
func (a *AvalancheProvider) CreateKeystore(_ string) error {
	keybase, err := keyring.New(a.PCfg.ChainID, a.PCfg.KeyringBackend, a.PCfg.KeyDirectory, a.Input, a.Codec.Marshaler, KeyringAlgoOptions())
	if err != nil {
		return err
	}
	a.Keybase = keybase
	return nil
}

// KeystoreCreated returns true if there is an existing keystore instance at the specified path, it returns false otherwise.
func (a *AvalancheProvider) KeystoreCreated(_ string) bool {
	if _, err := os.Stat(a.PCfg.KeyDirectory); errors.Is(err, os.ErrNotExist) {
		return false
	} else if a.Keybase == nil {
		return false
	}
	return true
}

// AddKey generates a new mnemonic which is then converted to a private key and BIP-39 HD Path and persists it to the keystore.
// It fails if there is an existing key with the same address.
func (a *AvalancheProvider) AddKey(name string, coinType uint32, signingAlgorithm string) (output *provider.KeyOutput, err error) {
	ko, err := cc.KeyAddOrRestore(name, coinType)
	if err != nil {
		return nil, err
	}
	return ko, nil
}

// RestoreKey converts a mnemonic to a private key and BIP-39 HD Path and persists it to the keystore.
// It fails if there is an existing key with the same address.
func (a *AvalancheProvider) RestoreKey(name, mnemonic string, coinType uint32, signingAlgorithm string) (address string, err error) {
	ko, err := a.KeyAddOrRestore(name, coinType, mnemonic)
	if err != nil {
		return "", err
	}
	return ko.Address, nil
}

// KeyAddOrRestore either generates a new mnemonic or uses the specified mnemonic and converts it to a private key
// and BIP-39 HD Path which is then persisted to the keystore. It fails if there is an existing key with the same address.
func (a *AvalancheProvider) KeyAddOrRestore(keyName string, coinType uint32, mnemonic ...string) (*provider.KeyOutput, error) {
	var mnemonicStr string
	var err error
	algo := keyring.SignatureAlgo(hd.Secp256k1)

	if len(mnemonic) > 0 {
		mnemonicStr = mnemonic[0]
	} else {
		mnemonicStr, err = CreateMnemonic()
		if err != nil {
			return nil, err
		}
	}

	if coinType == ethereumCoinType {
		algo = keyring.SignatureAlgo(ethermint.EthSecp256k1)
	}

	info, err := a.Keybase.NewAccount(keyName, mnemonicStr, "", hd.CreateHDPath(coinType, 0, 0).String(), algo)
	if err != nil {
		return nil, err
	}

	acc, err := info.GetAddress()
	if err != nil {
		return nil, err
	}

	out, err := a.EncodeBech32AccAddr(acc)
	if err != nil {
		return nil, err
	}
	return &provider.KeyOutput{Mnemonic: mnemonicStr, Address: out}, nil
}

// ShowAddress retrieves a key by name from the keystore and returns the bech32 encoded string representation of that key.
func (a *AvalancheProvider) ShowAddress(name string) (address string, err error) {
	info, err := a.Keybase.Key(name)
	if err != nil {
		return "", err
	}
	acc, err := info.GetAddress()
	if err != nil {
		return "", nil
	}
	out, err := a.EncodeBech32AccAddr(acc)
	if err != nil {
		return "", err
	}
	return out, nil
}

// ListAddresses returns a map of bech32 encoded strings representing all keys currently in the keystore.
func (a *AvalancheProvider) ListAddresses() (map[string]string, error) {
	out := map[string]string{}
	info, err := a.Keybase.List()
	if err != nil {
		return nil, err
	}
	for _, k := range info {
		acc, err := k.GetAddress()
		if err != nil {
			return nil, err
		}
		addr, err := a.EncodeBech32AccAddr(acc)
		if err != nil {
			return nil, err
		}
		out[k.Name] = addr
	}
	return out, nil
}

// DeleteKey removes a key from the keystore for the specified name.
func (a *AvalancheProvider) DeleteKey(name string) error {
	if err := a.Keybase.Delete(name); err != nil {
		return err
	}
	return nil
}

// KeyExists returns true if a key with the specified name exists in the keystore, it returns false otherwise.
func (a *AvalancheProvider) KeyExists(name string) bool {
	k, err := a.Keybase.Key(name)
	if err != nil {
		return false
	}

	return k.Name == name
}

// ExportPrivKeyArmor returns a private key in ASCII armored format.
// It returns an error if the key does not exist or a wrong encryption passphrase is supplied.
func (a *AvalancheProvider) ExportPrivKeyArmor(keyName string) (armor string, err error) {
	return a.Keybase.ExportPrivKeyArmor(keyName, ckeys.DefaultKeyPass)
}

// CreateMnemonic generates a new mnemonic.
func CreateMnemonic() (string, error) {
	entropySeed, err := bip39.NewEntropy(256)
	if err != nil {
		return "", err
	}
	mnemonic, err := bip39.NewMnemonic(entropySeed)
	if err != nil {
		return "", err
	}
	return mnemonic, nil
}

// EncodeBech32AccAddr returns the string bech32 representation for the specified account address.
// It returns an empty sting if the byte slice is 0-length.
// It returns an error if the bech32 conversion fails or the prefix is empty.
func (a *AvalancheProvider) EncodeBech32AccAddr(addr sdk.AccAddress) (string, error) {
	return sdk.Bech32ifyAddressBytes(a.PCfg.AccountPrefix, addr)
}
