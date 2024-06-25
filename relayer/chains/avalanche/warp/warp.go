package warp

import (
	"os"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/message"
	"github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

type Warp struct {
	network        *AppRequestNetwork
	responseChan   chan message.InboundMessage
	messageCreator message.Creator
}

func NewWarp(log *zap.Logger, networkID uint32, subnetID ids.ID, APINodeURL string) (*Warp, error) {
	logger := logging.NewLogger(
		"relayer",
		logging.NewWrappedCore(
			logging.Info,
			os.Stdout,
			logging.JSON.ConsoleEncoder(),
		),
	)
	// TODO: use some mock registerer?
	registerer := prometheus.NewRegistry()

	network, responseChan, err := NewNetwork(log, logger, registerer, networkID, []ids.ID{subnetID}, APINodeURL)
	if err != nil {
		log.Error(
			"Failed to create app request network",
			zap.Error(err),
		)
		return nil, err
	}

	// Initialize message creator passed down to relayers for creating app requests.
	messageCreator, err := message.NewCreator(logger, registerer, "message_creator", constants.DefaultNetworkCompressionType, constants.DefaultNetworkMaximumInboundTimeout)
	if err != nil {
		log.Error(
			"Failed to create message creator",
			zap.Error(err),
		)
		return nil, err
	}

	return &Warp{
		network:        network,
		responseChan:   responseChan,
		messageCreator: messageCreator,
	}, nil
}
