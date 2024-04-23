package avalanche

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	channeltypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"

	"github.com/ava-labs/subnet-evm/accounts/abi"
	evmtypes "github.com/ava-labs/subnet-evm/core/types"
	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	connectointypes "github.com/cosmos/ibc-go/v7/modules/core/03-connection/types"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/cosmos/relayer/v2/relayer/provider"
)

const (
	eventClientCreated         = "ClientCreated"
	eventClientUpdated         = "ClientUpdated"
	eventClientUpgraded        = "ClientUpgraded"
	eventConnectionOpenInit    = "ConnectionOpenInit"
	eventConnectionOpenTry     = "ConnectionOpenTry"
	eventConnectionOpenAck     = "ConnectionOpenAck"
	eventConnectionOpenConfirm = "ConnectionOpenConfirm"
	eventChannelOpenInit       = "ChannelOpenInit"
	eventChannelOpenTry        = "ChannelOpenTry"
	eventChannelOpenAck        = "ChannelOpenAck"
	eventChannelOpenConfirm    = "ChannelOpenConfirm"
	eventChannelCloseInit      = "ChannelCloseInit"
	eventChannelCloseConfirm   = "ChannelCloseConfirm"

	eventPacketSendPacket           = "SendPacket"
	eventPacketRecvPacket           = "RecvPacket"
	eventPacketWriteAck             = "WriteAck"
	eventPacketAcknowledgePacket    = "AcknowledgePacket"
	eventPacketTimeoutPacket        = "TimeoutPacket"
	eventPacketTimeoutPacketOnClose = "TimeoutPacketOnClose"
)

var (
	errNoEventSignature = errors.New("no event signature")

	eventNames = []string{
		eventClientCreated,
		eventClientUpdated,
		eventClientUpgraded,
		eventConnectionOpenInit,
		eventConnectionOpenTry,
		eventConnectionOpenAck,
		eventConnectionOpenConfirm,
		eventChannelOpenInit,
		eventChannelOpenTry,
		eventChannelOpenAck,
		eventChannelOpenConfirm,
		eventChannelCloseInit,
		eventChannelCloseConfirm,
		eventPacketSendPacket,
		eventPacketRecvPacket,
		eventPacketWriteAck,
		eventPacketAcknowledgePacket,
		eventPacketTimeoutPacket,
		eventPacketTimeoutPacketOnClose,
	}

	connEventsTransform = map[string]string{
		eventConnectionOpenInit:    connectointypes.EventTypeConnectionOpenInit,
		eventConnectionOpenTry:     connectointypes.EventTypeConnectionOpenTry,
		eventConnectionOpenAck:     connectointypes.EventTypeConnectionOpenAck,
		eventConnectionOpenConfirm: connectointypes.EventTypeConnectionOpenConfirm,
	}

	chanEventsTransform = map[string]string{
		eventChannelOpenInit:     channeltypes.EventTypeChannelOpenInit,
		eventChannelOpenTry:      channeltypes.EventTypeChannelOpenTry,
		eventChannelOpenAck:      channeltypes.EventTypeChannelOpenAck,
		eventChannelOpenConfirm:  channeltypes.EventTypeChannelOpenConfirm,
		eventChannelCloseInit:    channeltypes.EventTypeChannelCloseInit,
		eventChannelCloseConfirm: channeltypes.EventTypeChannelCloseConfirm,
	}

	packetEventsTransform = map[string]string{
		eventPacketSendPacket:           channeltypes.EventTypeSendPacket,
		eventPacketRecvPacket:           channeltypes.EventTypeRecvPacket,
		eventPacketWriteAck:             channeltypes.EventTypeWriteAck,
		eventPacketAcknowledgePacket:    channeltypes.EventTypeAcknowledgePacket,
		eventPacketTimeoutPacket:        channeltypes.EventTypeTimeoutPacket,
		eventPacketTimeoutPacketOnClose: channeltypes.EventTypeTimeoutPacketOnClose,
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
		case "clientId":
			ci.clientID = value
		case "consensusHeight":
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
				ci.Height = revisionHeight
			}
		case "clientMessage":
			ci.header = []byte(value)
		}
	}

}

type connectionInfo provider.ConnectionInfo

func (res *connectionInfo) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("connection_id", res.ConnID)
	enc.AddString("client_id", res.ClientID)
	enc.AddString("counterparty_connection_id", res.CounterpartyConnID)
	enc.AddString("counterparty_client_id", res.CounterpartyClientID)
	return nil
}

func (res *connectionInfo) parseAttrs(log *zap.Logger, attributes map[string]string) {
	for name, value := range attributes {
		res.parseConnectionAttribute(name, value)
	}
}

func (res *connectionInfo) parseConnectionAttribute(name, value string) {
	switch name {
	case "clientId":
		res.ClientID = value
	case "connectionId":
		res.ConnID = value
	case "counterpartyClientID":
		res.CounterpartyClientID = value
	case "counterpartyConnectionId":
		res.CounterpartyConnID = value
	}
}

type channelInfo provider.ChannelInfo

func (res *channelInfo) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("channel_id", res.ChannelID)
	enc.AddString("port_id", res.PortID)
	enc.AddString("counterparty_channel_id", res.CounterpartyChannelID)
	enc.AddString("counterparty_port_id", res.CounterpartyPortID)
	return nil
}

func (res *channelInfo) parseAttrs(log *zap.Logger, attributes map[string]string) {
	for name, value := range attributes {
		res.parseChannelAttribute(name, value)
	}
}

// parseChannelAttribute parses channel attributes from an event.
// If the attribute has already been parsed into the channelInfo,
// it will not overwrite, and return true to inform the caller that
// the attribute already exists.
func (res *channelInfo) parseChannelAttribute(name, value string) {
	switch name {
	case "connectionId":
		res.ConnID = value
	case "channelId":
		res.ChannelID = value
	case "portId":
		res.PortID = value
	case "counterpartyChannelId":
		res.CounterpartyChannelID = value
	case "counterpartyPortID":
		res.CounterpartyPortID = value
	case "version":
		res.Version = value
	}
}

type packetInfo provider.PacketInfo

func (res *packetInfo) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddUint64("sequence", res.Sequence)
	enc.AddString("src_channel", res.SourceChannel)
	enc.AddString("src_port", res.SourcePort)
	enc.AddString("dst_channel", res.DestChannel)
	enc.AddString("dst_port", res.DestPort)
	return nil
}

// parsePacketInfo is treated differently from the others since it can be constructed from the accumulation of multiple events
func (res *packetInfo) parseAttrs(log *zap.Logger, attributes map[string]string) {
	for name, value := range attributes {
		res.parseChannelAttribute(log, name, value)
	}
}

func (res *packetInfo) parseChannelAttribute(log *zap.Logger, name, value string) {
	var err error
	switch name {
	case "sequence":
		res.Sequence, err = strconv.ParseUint(value, 10, 64)
	case "timeoutTimestamp":
		res.TimeoutTimestamp, err = strconv.ParseUint(value, 10, 64)
	case "data":
		res.Data = []byte(value)
	case "ack":
		res.Ack = []byte(value)
	case "timeoutHeight":
		timeoutSplit := strings.Split(value, "-")
		if len(timeoutSplit) != 2 {
			log.Error("Error parsing packet height timeout",
				zap.Uint64("sequence", res.Sequence),
				zap.String("value", value),
			)
			return
		}
		revisionNumber, err := strconv.ParseUint(timeoutSplit[0], 10, 64)
		if err != nil {
			log.Error("Error parsing packet timeout height revision number",
				zap.Uint64("sequence", res.Sequence),
				zap.String("value", timeoutSplit[0]),
				zap.Error(err),
			)
			return
		}
		revisionHeight, err := strconv.ParseUint(timeoutSplit[1], 10, 64)
		if err != nil {
			log.Error("Error parsing packet timeout height revision height",
				zap.Uint64("sequence", res.Sequence),
				zap.String("value", timeoutSplit[1]),
				zap.Error(err),
			)
			return
		}
		res.TimeoutHeight = clienttypes.Height{
			RevisionNumber: revisionNumber,
			RevisionHeight: revisionHeight,
		}
	case "sourcePort":
		res.SourcePort = value
	case "sourceChannel":
		res.SourceChannel = value
	case "destPort":
		res.DestPort = value
	case "destChannel":
		res.DestChannel = value
	case "channelOrdering":
		res.ChannelOrder = value
	}
	if err != nil {
		log.Error("Error parsing packet info",
			zap.String("name", name),
			zap.String("value", value),
			zap.Error(err),
		)
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
		case eventConnectionOpenInit:
			attributes := make(map[string]string)
			attributes[connectointypes.AttributeKeyClientID] = event.Attributes["clientId"]
			attributes[connectointypes.AttributeKeyConnectionID] = event.Attributes["connectionId"]
			attributes[connectointypes.AttributeKeyCounterpartyClientID] = event.Attributes["counterpartyClientID"]
			events = append(events, provider.RelayerEvent{
				EventType:  connectointypes.EventTypeConnectionOpenInit,
				Attributes: attributes,
			})
		case eventConnectionOpenTry:
			attributes := make(map[string]string)
			attributes[connectointypes.AttributeKeyClientID] = event.Attributes["clientId"]
			attributes[connectointypes.AttributeKeyConnectionID] = event.Attributes["connectionId"]
			attributes[connectointypes.AttributeKeyCounterpartyClientID] = event.Attributes["counterpartyClientID"]
			attributes[connectointypes.AttributeKeyCounterpartyConnectionID] = event.Attributes["counterpartyConnectionId"]
			events = append(events, provider.RelayerEvent{
				EventType:  connectointypes.EventTypeConnectionOpenTry,
				Attributes: attributes,
			})
		case eventConnectionOpenAck:
			attributes := make(map[string]string)
			attributes[connectointypes.AttributeKeyClientID] = event.Attributes["clientId"]
			attributes[connectointypes.AttributeKeyConnectionID] = event.Attributes["connectionId"]
			attributes[connectointypes.AttributeKeyCounterpartyClientID] = event.Attributes["counterpartyClientID"]
			attributes[connectointypes.AttributeKeyCounterpartyConnectionID] = event.Attributes["counterpartyConnectionId"]
			events = append(events, provider.RelayerEvent{
				EventType:  connectointypes.EventTypeConnectionOpenAck,
				Attributes: attributes,
			})
		case eventConnectionOpenConfirm:
			attributes := make(map[string]string)
			attributes[connectointypes.AttributeKeyClientID] = event.Attributes["clientId"]
			attributes[connectointypes.AttributeKeyConnectionID] = event.Attributes["connectionId"]
			attributes[connectointypes.AttributeKeyCounterpartyClientID] = event.Attributes["counterpartyClientID"]
			attributes[connectointypes.AttributeKeyCounterpartyConnectionID] = event.Attributes["counterpartyConnectionId"]
			events = append(events, provider.RelayerEvent{
				EventType:  connectointypes.EventTypeConnectionOpenConfirm,
				Attributes: attributes,
			})
		case eventChannelOpenInit:
			attributes := make(map[string]string)
			attributes[channeltypes.AttributeKeyConnectionID] = event.Attributes["connectionId"]
			attributes[channeltypes.AttributeKeyChannelID] = event.Attributes["channelId"]
			attributes[channeltypes.AttributeKeyPortID] = event.Attributes["portId"]
			attributes[channeltypes.AttributeCounterpartyPortID] = event.Attributes["counterpartyPortID"]
			attributes[channeltypes.AttributeVersion] = event.Attributes["version"]
			events = append(events, provider.RelayerEvent{
				EventType:  channeltypes.EventTypeChannelOpenInit,
				Attributes: attributes,
			})
		case eventChannelOpenTry:
			attributes := make(map[string]string)
			attributes[channeltypes.AttributeKeyConnectionID] = event.Attributes["connectionId"]
			attributes[channeltypes.AttributeKeyChannelID] = event.Attributes["channelId"]
			attributes[channeltypes.AttributeKeyPortID] = event.Attributes["portId"]
			attributes[channeltypes.AttributeCounterpartyChannelID] = event.Attributes["counterpartyChannelId"]
			attributes[channeltypes.AttributeCounterpartyPortID] = event.Attributes["counterpartyPortID"]
			attributes[channeltypes.AttributeVersion] = event.Attributes["version"]
			events = append(events, provider.RelayerEvent{
				EventType:  channeltypes.EventTypeChannelOpenTry,
				Attributes: attributes,
			})
		case eventChannelOpenAck:
			attributes := make(map[string]string)
			attributes[channeltypes.AttributeKeyConnectionID] = event.Attributes["connectionId"]
			attributes[channeltypes.AttributeKeyChannelID] = event.Attributes["channelId"]
			attributes[channeltypes.AttributeKeyPortID] = event.Attributes["portId"]
			attributes[channeltypes.AttributeCounterpartyChannelID] = event.Attributes["counterpartyChannelId"]
			attributes[channeltypes.AttributeCounterpartyPortID] = event.Attributes["counterpartyPortID"]
			events = append(events, provider.RelayerEvent{
				EventType:  channeltypes.EventTypeChannelOpenAck,
				Attributes: attributes,
			})
		case eventChannelOpenConfirm:
			attributes := make(map[string]string)
			attributes[channeltypes.AttributeKeyConnectionID] = event.Attributes["connectionId"]
			attributes[channeltypes.AttributeKeyChannelID] = event.Attributes["channelId"]
			attributes[channeltypes.AttributeKeyPortID] = event.Attributes["portId"]
			attributes[channeltypes.AttributeCounterpartyChannelID] = event.Attributes["counterpartyChannelId"]
			attributes[channeltypes.AttributeCounterpartyPortID] = event.Attributes["counterpartyPortID"]
			events = append(events, provider.RelayerEvent{
				EventType:  channeltypes.EventTypeChannelOpenConfirm,
				Attributes: attributes,
			})
		case eventChannelCloseInit:
			attributes := make(map[string]string)
			attributes[channeltypes.AttributeKeyConnectionID] = event.Attributes["connectionId"]
			attributes[channeltypes.AttributeKeyChannelID] = event.Attributes["channelId"]
			attributes[channeltypes.AttributeKeyPortID] = event.Attributes["portId"]
			attributes[channeltypes.AttributeCounterpartyChannelID] = event.Attributes["counterpartyChannelId"]
			attributes[channeltypes.AttributeCounterpartyPortID] = event.Attributes["counterpartyPortID"]
			events = append(events, provider.RelayerEvent{
				EventType:  channeltypes.EventTypeChannelCloseInit,
				Attributes: attributes,
			})
		case eventChannelCloseConfirm:
			attributes := make(map[string]string)
			attributes[channeltypes.AttributeKeyConnectionID] = event.Attributes["connectionId"]
			attributes[channeltypes.AttributeKeyChannelID] = event.Attributes["channelId"]
			attributes[channeltypes.AttributeKeyPortID] = event.Attributes["portId"]
			attributes[channeltypes.AttributeCounterpartyChannelID] = event.Attributes["counterpartyChannelId"]
			attributes[channeltypes.AttributeCounterpartyPortID] = event.Attributes["counterpartyPortID"]
			events = append(events, provider.RelayerEvent{
				EventType:  channeltypes.EventTypeChannelCloseConfirm,
				Attributes: attributes,
			})
		case eventPacketSendPacket:
			attributes := make(map[string]string)
			attributes[channeltypes.AttributeKeyData] = event.Attributes["data"]
			// TODO add transformation
			events = append(events, provider.RelayerEvent{
				EventType:  channeltypes.EventTypeChannelCloseConfirm,
				Attributes: attributes,
			})
		case eventPacketRecvPacket:
			attributes := make(map[string]string)
			// TODO add transformation
			events = append(events, provider.RelayerEvent{
				EventType:  channeltypes.EventTypeChannelCloseConfirm,
				Attributes: attributes,
			})
		case eventPacketWriteAck:
			attributes := make(map[string]string)
			// TODO add transformation
			events = append(events, provider.RelayerEvent{
				EventType:  channeltypes.EventTypeChannelCloseConfirm,
				Attributes: attributes,
			})
		case eventPacketAcknowledgePacket:
			attributes := make(map[string]string)
			// TODO add transformation
			events = append(events, provider.RelayerEvent{
				EventType:  channeltypes.EventTypeChannelCloseConfirm,
				Attributes: attributes,
			})
		case eventPacketTimeoutPacket:
			attributes := make(map[string]string)
			// TODO add transformation
			events = append(events, provider.RelayerEvent{
				EventType:  channeltypes.EventTypeChannelCloseConfirm,
				Attributes: attributes,
			})
		case eventPacketTimeoutPacketOnClose:
			attributes := make(map[string]string)
			// TODO add transformation
			events = append(events, provider.RelayerEvent{
				EventType:  channeltypes.EventTypeChannelCloseConfirm,
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
	var ci ibcMessageInfo
	var eventType string = event.EventType
	switch event.EventType {
	case eventClientCreated, eventClientUpdated, eventClientUpgraded:
		ci = new(clientInfo)
		ci.parseAttrs(log, event.Attributes)
	case eventConnectionOpenInit, eventConnectionOpenTry, eventConnectionOpenAck, eventConnectionOpenConfirm:
		ci = &connectionInfo{Height: height}
		ci.parseAttrs(log, event.Attributes)
		eventType = connEventsTransform[eventType]
	case eventChannelOpenInit, eventChannelOpenTry, eventChannelOpenAck, eventChannelOpenConfirm, eventChannelCloseInit, eventChannelCloseConfirm:
		ci = &channelInfo{Height: height}
		ci.parseAttrs(log, event.Attributes)
		eventType = chanEventsTransform[eventType]
	case eventPacketSendPacket, eventPacketRecvPacket, eventPacketWriteAck, eventPacketAcknowledgePacket, eventPacketTimeoutPacket, eventPacketTimeoutPacketOnClose:
		ci = &packetInfo{Height: height}
		ci.parseAttrs(log, event.Attributes)
		eventType = packetEventsTransform[eventType]
	}

	log.Debug("Parse IBC message", zap.String("event", eventType), zap.Object("ci", ci))
	return &ibcMessage{
		eventType: eventType,
		info:      ci,
	}
}
