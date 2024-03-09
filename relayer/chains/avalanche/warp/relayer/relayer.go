// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package relayer

import (
	"fmt"
	"math/rand"
	"strconv"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/message"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	"github.com/ethereum/go-ethereum/common"
	"go.uber.org/zap"

	"github.com/cosmos/relayer/v2/relayer/chains/avalanche/warp"
)

// Relayer handles all messages sent from a given source chain
type Relayer struct {
	pChainClient             platformvm.Client
	canonicalValidatorClient *CanonicalValidatorClient
	currentRequestID         uint32
	network                  *warp.AppRequestNetwork
	sourceSubnetID           ids.ID
	sourceBlockchainID       ids.ID
	responseChan             chan message.InboundMessage
	contractMessage          vms.ContractMessage
	messageManagers          map[common.Hash]messages.MessageManager
	logger                   logging.Logger
	supportedDestinations    set.Set[ids.ID]
	apiNodeURI               string
}

func NewRelayer(
	logger logging.Logger,
	sourceSubnetInfo config.SourceSubnet,
	pChainClient platformvm.Client,
	network *warp.AppRequestNetwork,
	responseChan chan message.InboundMessage,
	destinationClients map[ids.ID]vms.DestinationClient,
	shouldProcessMissedBlocks bool,
) (*Relayer, error) {

	subnetID, err := ids.FromString(sourceSubnetInfo.SubnetID)
	if err != nil {
		logger.Error(
			"Invalid subnetID in configuration",
			zap.Error(err),
		)
		return nil, err
	}

	blockchainID, err := ids.FromString(sourceSubnetInfo.BlockchainID)
	if err != nil {
		logger.Error(
			"Failed to decode base-58 encoded source chain ID",
			zap.Error(err),
		)
		return nil, err
	}

	var filteredDestinationClients map[ids.ID]vms.DestinationClient
	supportedDestinationsBlockchainIDs := sourceSubnetInfo.GetSupportedDestinations()
	if len(supportedDestinationsBlockchainIDs) > 0 {
		filteredDestinationClients := make(map[ids.ID]vms.DestinationClient)
		for id := range supportedDestinationsBlockchainIDs {
			filteredDestinationClients[id] = destinationClients[id]
		}
	} else {
		filteredDestinationClients = destinationClients
	}

	// Create message managers for each supported message protocol
	messageManagers := make(map[common.Hash]messages.MessageManager)
	for address, config := range sourceSubnetInfo.MessageContracts {
		addressHash := common.HexToHash(address)
		messageManager, err := messages.NewMessageManager(logger, addressHash, config, filteredDestinationClients)
		if err != nil {
			logger.Error(
				"Failed to create message manager",
				zap.Error(err),
			)
			return nil, err
		}
		messageManagers[addressHash] = messageManager
	}

	logger.Info(
		"Creating relayer",
		zap.String("subnetID", subnetID.String()),
		zap.String("subnetIDHex", subnetID.Hex()),
		zap.String("blockchainID", blockchainID.String()),
		zap.String("blockchainIDHex", blockchainID.Hex()),
	)
	r := Relayer{
		pChainClient:             pChainClient,
		canonicalValidatorClient: NewCanonicalValidatorClient(logger, pChainClient),
		currentRequestID:         rand.Uint32(), // Initialize to a random value to mitigate requestID collision
		network:                  network,
		sourceSubnetID:           subnetID,
		sourceBlockchainID:       blockchainID,
		responseChan:             responseChan,
		contractMessage:          vms.NewContractMessage(logger, sourceSubnetInfo),
		messageManagers:          messageManagers,
		logger:                   logger,
		supportedDestinations:    supportedDestinationsBlockchainIDs,
	}

	return &r, nil
}

// RelayMessage relays a single warp message to the destination chain. Warp message relay requests from the same origin chain are processed serially
func (r *Relayer) RelayMessage(warpLogInfo *vmtypes.WarpLogInfo, metrics *MessageRelayerMetrics, messageCreator message.Creator) error {
	r.logger.Info(
		"Relaying message",
		zap.String("blockchainID", r.sourceBlockchainID.String()),
	)
	// Unpack the VM message bytes into a Warp message
	warpMessageInfo, err := r.contractMessage.UnpackWarpMessage(warpLogInfo.UnsignedMsgBytes)
	if err != nil {
		r.logger.Error(
			"Failed to unpack sender message",
			zap.Error(err),
		)
		return err
	}

	r.logger.Info(
		"Unpacked warp message",
		zap.String("blockchainID", r.sourceBlockchainID.String()),
		zap.String("warpMessageID", warpMessageInfo.WarpUnsignedMessage.ID().String()),
	)

	// Check that the warp message is from a support message protocol contract address.
	messageManager, supportedMessageProtocol := r.messageManagers[warpLogInfo.SourceAddress]
	if !supportedMessageProtocol {
		// Do not return an error here because it is expected for there to be messages from other contracts
		// than just the ones supported by a single relayer instance.
		r.logger.Debug(
			"Warp message from unsupported message protocol address. Not relaying.",
			zap.String("protocolAddress", warpLogInfo.SourceAddress.Hex()),
		)
		return nil
	}

	destinationBlockchainID, err := messageManager.GetDestinationBlockchainID(warpMessageInfo)
	if err != nil {
		r.logger.Error(
			"Failed to get destination chain ID",
			zap.Error(err),
		)
		return err
	}

	// Check that the destination chain ID is supported
	if !r.CheckSupportedDestination(destinationBlockchainID) {
		r.logger.Debug(
			"Message destination chain ID not supported. Not relaying.",
			zap.String("blockchainID", r.sourceBlockchainID.String()),
			zap.String("destinationBlockchainID", destinationBlockchainID.String()),
		)
		return nil
	}

	// Create and run the message relayer to attempt to deliver the message to the destination chain
	messageRelayer := newMessageRelayer(r.logger, metrics, r, warpMessageInfo.WarpUnsignedMessage, destinationBlockchainID, r.responseChan, messageCreator)
	if err != nil {
		r.logger.Error(
			"Failed to create message relayer",
			zap.Error(err),
		)
		return err
	}

	// Relay the message to the destination. Messages from a given source chain must be processed in serial in order to
	// guarantee that the previous block (n-1) is fully processed by the relayer when processing a given log from block n.
	// TODO: Add a config option to use the Warp API, instead of hardcoding to the app request network here
	err = messageRelayer.relayMessage(warpMessageInfo, r.currentRequestID, messageManager, true)
	if err != nil {
		r.logger.Error(
			"Failed to run message relayer",
			zap.String("blockchainID", r.sourceBlockchainID.String()),
			zap.String("warpMessageID", warpMessageInfo.WarpUnsignedMessage.ID().String()),
			zap.Error(err),
		)
		return err
	}

	// Increment the request ID for the next message relay request
	r.currentRequestID++

	// Update the database with the latest processed block height
	err = r.db.Put(r.sourceBlockchainID, []byte(database.LatestProcessedBlockKey), []byte(strconv.FormatUint(warpLogInfo.BlockNumber, 10)))
	if err != nil {
		r.logger.Error(
			fmt.Sprintf("failed to put %s into database", database.LatestProcessedBlockKey),
			zap.Error(err),
		)
	}

	return nil
}

// Returns whether destinationBlockchainID is a supported destination.
// If supportedDestinations is empty, then all destination chain IDs are supported.
func (r *Relayer) CheckSupportedDestination(destinationBlockchainID ids.ID) bool {
	return len(r.supportedDestinations) == 0 || r.supportedDestinations.Contains(destinationBlockchainID)
}
