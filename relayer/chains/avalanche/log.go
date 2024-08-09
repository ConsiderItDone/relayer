package avalanche

import (
	evmtypes "github.com/ava-labs/subnet-evm/core/types"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// LogSuccessTx take the transaction and the messages to create it and logs the appropriate data
func (a *AvalancheProvider) LogSuccessTx(receipt *evmtypes.Receipt) {
	// Include the chain_id
	fields := []zapcore.Field{
		zap.String("chain_id", a.ChainId()),
		zap.Uint64("gas_used", receipt.GasUsed),
		zap.Int64("height", receipt.BlockNumber.Int64()),
		zap.String("tx_hash", receipt.TxHash.Hex()),
	}

	// Log the succesful transaction with fields
	a.log.Info(
		"Successful transaction",
		fields...,
	)
}
