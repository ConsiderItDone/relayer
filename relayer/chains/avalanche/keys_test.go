package avalanche_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/cosmos/relayer/v2/relayer/chains/avalanche"
	"github.com/cosmos/relayer/v2/relayer/provider"
)

func testProviderWithKeystore(t *testing.T) provider.ChainProvider {
	homePath := t.TempDir()
	cfg := avalanche.AvalancheProviderConfig{
		ChainID:        "test",
		KeyDirectory:   filepath.Join(homePath, "keys"),
		KeyringBackend: "test",
		Timeout:        "10s",
	}
	p, err := cfg.NewProvider(zap.NewNop(), homePath, true, "test_chain")
	if err != nil {
		t.Fatalf("Error creating provider: %v", err)
	}
	err = p.CreateKeystore(homePath)
	if err != nil {
		t.Fatalf("Error creating keystore: %v", err)
	}
	return p
}

// TestKeyRestore restores a test mnemonic
func TestKeyRestore(t *testing.T) {
	const (
		keyName            = "test_key"
		signatureAlgorithm = "secp256k1"
		mnemonic           = "three elevator silk family street child flip also leaf inmate call frame shock little legal october vivid enable fetch siege sell burger dolphin green"
		expectedAddress    = "0x836E7e82deDE708Ba83ADe38216F5e30AC0fFB03"
		coinType           = uint32(118)
	)

	p := testProviderWithKeystore(t)

	address, err := p.RestoreKey(keyName, mnemonic, coinType, signatureAlgorithm)
	require.NoError(t, err)
	require.Equal(t, expectedAddress, address)
}

// TestKeyRestore restores a test mnemonic
func TestKeyRestorePrivateKey(t *testing.T) {
	const (
		keyName            = "test_key"
		signatureAlgorithm = "secp256k1"
		mnemonic           = "56289e99c94b6912bfc12adc093c9b51124f0dc54ac7a766b2bc5ccf558d8027"
		expectedAddress    = "0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC"
		coinType           = uint32(118)
	)

	p := testProviderWithKeystore(t)

	address, err := p.RestoreKey(keyName, mnemonic, coinType, signatureAlgorithm)
	require.NoError(t, err)
	require.Equal(t, expectedAddress, address)
}
