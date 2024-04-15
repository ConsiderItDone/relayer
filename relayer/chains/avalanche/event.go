package avalanche

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ava-labs/subnet-evm/accounts/abi"
	evmtypes "github.com/ava-labs/subnet-evm/core/types"
	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	connectointypes "github.com/cosmos/ibc-go/v7/modules/core/03-connection/types"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/cosmos/relayer/v2/relayer/provider"
)

const (
	eventClientCreated     = "ClientCreated"
	eventClientUpdated     = "ClientUpdated"
	eventClientUpgraded    = "ClientUpgraded"
	eventConnectionCreated = "ConnectionCreated"
)

var (
	errNoEventSignature = errors.New("no event signature")

	eventNames = []string{
		eventClientCreated,
		eventClientUpdated,
		eventClientUpgraded,
		eventConnectionCreated,
	}
)

// ibcMessage is the type used for parsing all possible properties of IBC messages
type ibcMessage struct {
	eventType string
	info      ibcMessageInfo
}

type ibcMessageInfo interface {
	parseAttrs(log *zap.Logger, attrs map[string]string)
	MarshalLogObject(enc zapcore.ObjectEncoder) error
}

// clientInfo contains the consensus height of the counterparty chain for a client.
type clientInfo struct {
	clientID        string
	consensusHeight clienttypes.Height
	Height          uint64
	header          []byte
}

func (ci *clientInfo) ClientState(trustingPeriod time.Duration) provider.ClientState {
	return provider.ClientState{
		ClientID:        ci.clientID,
		ConsensusHeight: ci.consensusHeight,
		TrustingPeriod:  trustingPeriod,
		Header:          ci.header,
	}
}

func (ci *clientInfo) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("client_id", ci.clientID)
	enc.AddUint64("consensus_height", ci.consensusHeight.RevisionHeight)
	enc.AddUint64("consensus_height_revision", ci.consensusHeight.RevisionNumber)
	return nil
}

func (ci *clientInfo) parseAttrs(log *zap.Logger, attributes map[string]string) {
	for key, value := range attributes {
		switch key {
		case clienttypes.AttributeKeyClientID:
			ci.clientID = value
		case clienttypes.AttributeKeyConsensusHeight:
			revisionSplit := strings.Split(value, "-")
			if len(revisionSplit) != 2 {
				log.Error("Error parsing client consensus height",
					zap.String("client_id", ci.clientID),
					zap.String("value", value),
				)
			} else {
				revisionNumberString := revisionSplit[0]
				revisionNumber, err := strconv.ParseUint(revisionNumberString, 10, 64)
				if err != nil {
					log.Error("Error parsing client consensus height revision number",
						zap.Error(err),
					)
				}
				revisionHeightString := revisionSplit[1]
				revisionHeight, err := strconv.ParseUint(revisionHeightString, 10, 64)
				if err != nil {
					log.Error("Error parsing client consensus height revision height",
						zap.Error(err),
					)
					return
				}
				ci.consensusHeight = clienttypes.Height{
					RevisionNumber: revisionNumber,
					RevisionHeight: revisionHeight,
				}
			}
		case clienttypes.AttributeKeyHeader:
			ci.header = []byte(value)
		}
	}

}

func transformEvents(origEvents []provider.RelayerEvent) []provider.RelayerEvent {
	var events []provider.RelayerEvent

	for _, event := range origEvents {
		switch event.EventType {
		case eventClientCreated:
			attributes := make(map[string]string)
			attributes[clienttypes.AttributeKeyClientID] = event.Attributes["clientId"]

			events = append(events, provider.RelayerEvent{
				EventType:  clienttypes.EventTypeCreateClient,
				Attributes: attributes,
			})
		case eventClientUpdated:
			attributes := make(map[string]string)
			attributes[clienttypes.AttributeKeyClientID] = event.Attributes["clientId"]
			attributes[clienttypes.AttributeKeyConsensusHeight] = event.Attributes["consensusHeight"]
			events = append(events, provider.RelayerEvent{
				EventType:  clienttypes.EventTypeUpdateClient,
				Attributes: attributes,
			})
		case eventClientUpgraded:
			attributes := make(map[string]string)
			attributes[clienttypes.AttributeKeyClientID] = event.Attributes["clientId"]
			attributes[clienttypes.AttributeKeyConsensusHeight] = event.Attributes["consensusHeight"]
			attributes[clienttypes.AttributeKeyHeader] = event.Attributes["clientMessage"]
			events = append(events, provider.RelayerEvent{
				EventType:  clienttypes.EventTypeUpgradeClient,
				Attributes: attributes,
			})
		case eventConnectionCreated:
			attributes := make(map[string]string)
			attributes[clienttypes.AttributeKeyClientID] = event.Attributes["clientId"]
			attributes[connectointypes.AttributeKeyConnectionID] = event.Attributes["connectionId"]
			events = append(events, provider.RelayerEvent{
				EventType:  connectointypes.EventTypeConnectionOpenInit,
				Attributes: attributes,
			})
		}
	}

	return events
}

func parseEventsFromTxReceipt(contractABI abi.ABI, receipt *evmtypes.Receipt) ([]provider.RelayerEvent, error) {
	var events []provider.RelayerEvent

	for _, log := range receipt.Logs {
		if len(log.Topics) == 0 {
			return events, errNoEventSignature
		}
		for _, eventName := range eventNames {
			abiEvent, ok := contractABI.Events[eventName]
			if !ok {
				return nil, fmt.Errorf("event %s doesn't exist in ABI", eventName)
			}
			if log.Topics[0] != abiEvent.ID {
				continue
			}
			// we found our event in logs

			// parse non-indexed data
			eventMap := map[string]interface{}{}
			if len(log.Data) > 0 {
				if err := contractABI.UnpackIntoMap(eventMap, eventName, log.Data); err != nil {
					return nil, err
				}
			}

			// parse indexed data
			var indexed abi.Arguments
			for _, arg := range abiEvent.Inputs {
				if arg.Indexed {
					indexed = append(indexed, arg)
				}
			}

			if err := abi.ParseTopicsIntoMap(eventMap, indexed, log.Topics[1:]); err != nil {
				return nil, err
			}

			// convert from map into Relayer event structure
			attributes := make(map[string]string)
			for key, value := range eventMap {
				attributes[key] = fmt.Sprintf("%v", value)
			}

			events = append(events, provider.RelayerEvent{
				EventType:  eventName,
				Attributes: attributes,
			})
		}
	}

	return events, nil
}

func ibcMessagesFromEvents(log *zap.Logger, events []provider.RelayerEvent, height uint64) []ibcMessage {
	var messages []ibcMessage
	for _, event := range events {
		m := parseIBCMessageFromEvent(log, event, height)
		if m == nil || m.info == nil {
			continue
		}
		messages = append(messages, *m)
	}

	return messages
}

func parseIBCMessageFromEvent(log *zap.Logger, event provider.RelayerEvent, height uint64) *ibcMessage {
	switch event.EventType {
	case eventClientCreated, eventClientUpdated, eventClientUpgraded:
		ci := new(clientInfo)
		ci.parseAttrs(log, event.Attributes)
		return &ibcMessage{
			eventType: event.EventType,
			info:      ci,
		}
	case eventConnectionCreated:
		panic(fmt.Sprintf("TODO implement me parseIBCMessageFromEvent<%s>", eventConnectionCreated))
	}

	return nil
}
