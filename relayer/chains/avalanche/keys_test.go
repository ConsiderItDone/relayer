package avalanche_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/cosmos/relayer/v2/relayer/chains/avalanche"
	"github.com/cosmos/relayer/v2/relayer/provider"
)

func testProviderWithKeystore(t *testing.T, extraCodecs []string) provider.ChainProvider {
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
		expectedAddress    = "0x6E7BE67F3619731AB38875999873E8FACE935735"
		coinType           = uint32(60)
	)

	p := testProviderWithKeystore(t, []string{"ethermint"})

	address, err := p.RestoreKey(keyName, mnemonic, coinType, signatureAlgorithm)
	require.NoError(t, err)
	require.Equal(t, expectedAddress, address)
}
