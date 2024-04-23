package avalanche

import (
	"context"
	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	"go.uber.org/zap/zapcore"

	conntypes "github.com/cosmos/ibc-go/v7/modules/core/03-connection/types"
	"go.uber.org/zap"

	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/cosmos/relayer/v2/relayer/provider"
)

func (acp *AvalancheChainProcessor) handleMessage(ctx context.Context, m ibcMessage, c processor.IBCMessagesCache) {
	switch t := m.info.(type) {
	case *clientInfo:
		acp.handleClientMessage(ctx, m.eventType, *t)
	case *connectionInfo:
		acp.handleConnectionMessage(m.eventType, provider.ConnectionInfo(*t), c)
	case *channelInfo:
		acp.handleChannelMessage(m.eventType, provider.ChannelInfo(*t), c)
	case *packetInfo:
		acp.handlePacketMessage(m.eventType, provider.PacketInfo(*t), c)
	}
}

func (acp *AvalancheChainProcessor) handleClientMessage(ctx context.Context, eventType string, ci clientInfo) {
	acp.latestClientState.update(ctx, ci, acp)
	acp.logObservedIBCMessage(eventType, zap.String("client_id", ci.clientID))
}

func (acp *AvalancheChainProcessor) handleConnectionMessage(eventType string, ci provider.ConnectionInfo, ibcMessagesCache processor.IBCMessagesCache) {
	acp.connectionClients[ci.ConnID] = ci.ClientID
	connectionKey := processor.ConnectionInfoConnectionKey(ci)
	if eventType == conntypes.EventTypeConnectionOpenInit {
		found := false
		for k := range acp.connectionStateCache {
			// Don't add a connectionKey to the connectionStateCache without counterparty connection ID
			// since we already have the connectionKey in the connectionStateCache which includes the
			// counterparty connection ID.
			if k.MsgInitKey() == connectionKey {
				found = true
				break
			}
		}
		if !found {
			acp.connectionStateCache[connectionKey] = false
		}
	} else {
		// Clear out MsgInitKeys once we have the counterparty connection ID
		delete(acp.connectionStateCache, connectionKey.MsgInitKey())
		open := (eventType == conntypes.EventTypeConnectionOpenAck || eventType == conntypes.EventTypeConnectionOpenConfirm)
		acp.connectionStateCache[connectionKey] = open
	}
	ibcMessagesCache.ConnectionHandshake.Retain(connectionKey, eventType, ci)

	acp.logConnectionMessage(eventType, ci)
}

func (acp *AvalancheChainProcessor) handleChannelMessage(eventType string, ci provider.ChannelInfo, ibcMessagesCache processor.IBCMessagesCache) {
	acp.channelConnections[ci.ChannelID] = ci.ConnID
	channelKey := processor.ChannelInfoChannelKey(ci)

	if eventType == chantypes.EventTypeChannelOpenInit {
		found := false
		for k := range acp.channelStateCache {
			// Don't add a channelKey to the channelStateCache without counterparty channel ID
			// since we already have the channelKey in the channelStateCache which includes the
			// counterparty channel ID.
			if k.MsgInitKey() == channelKey {
				found = true
				break
			}
		}
		if !found {
			acp.channelStateCache.SetOpen(channelKey, false, ci.Order)
		}
	} else {
		switch eventType {
		case chantypes.EventTypeChannelOpenTry:
			acp.channelStateCache.SetOpen(channelKey, false, ci.Order)
		case chantypes.EventTypeChannelOpenAck, chantypes.EventTypeChannelOpenConfirm:
			acp.channelStateCache.SetOpen(channelKey, true, ci.Order)
			acp.logChannelOpenMessage(eventType, ci)
		case chantypes.EventTypeChannelCloseConfirm:
			for k := range acp.channelStateCache {
				if k.PortID == ci.PortID && k.ChannelID == ci.ChannelID {
					acp.channelStateCache.SetOpen(channelKey, false, ci.Order)
					break
				}
			}
		}
		// Clear out MsgInitKeys once we have the counterparty channel ID
		delete(acp.channelStateCache, channelKey.MsgInitKey())
	}
}

func (acp *AvalancheChainProcessor) handlePacketMessage(eventType string, pi provider.PacketInfo, c processor.IBCMessagesCache) {
	k, err := processor.PacketInfoChannelKey(eventType, pi)
	if err != nil {
		acp.log.Error("Unexpected error handling packet message",
			zap.String("event_type", eventType),
			zap.Uint64("sequence", pi.Sequence),
			zap.Inline(k),
			zap.Error(err),
		)
		return
	}

	if eventType == chantypes.EventTypeTimeoutPacket && pi.ChannelOrder == chantypes.ORDERED.String() {
		acp.channelStateCache.SetOpen(k, false, chantypes.ORDERED)
	}

	if !c.PacketFlow.ShouldRetainSequence(acp.pathProcessors, k, acp.chainProvider.ChainId(), eventType, pi.Sequence) {
		acp.log.Debug("Not retaining packet message",
			zap.String("event_type", eventType),
			zap.Uint64("sequence", pi.Sequence),
			zap.Inline(k),
		)
		return
	}

	acp.log.Debug("Retaining packet message",
		zap.String("event_type", eventType),
		zap.Uint64("sequence", pi.Sequence),
		zap.Inline(k),
	)

	c.PacketFlow.Retain(k, eventType, pi)
	acp.logPacketMessage(eventType, pi)
}

func (acp *AvalancheChainProcessor) logPacketMessage(message string, pi provider.PacketInfo) {
	if !acp.log.Core().Enabled(zapcore.DebugLevel) {
		return
	}
	fields := []zap.Field{
		zap.Uint64("sequence", pi.Sequence),
		zap.String("src_channel", pi.SourceChannel),
		zap.String("src_port", pi.SourcePort),
		zap.String("dst_channel", pi.DestChannel),
		zap.String("dst_port", pi.DestPort),
	}
	if pi.TimeoutHeight.RevisionHeight > 0 {
		fields = append(fields, zap.Uint64("timeout_height", pi.TimeoutHeight.RevisionHeight))
	}
	if pi.TimeoutHeight.RevisionNumber > 0 {
		fields = append(fields, zap.Uint64("timeout_height_revision", pi.TimeoutHeight.RevisionNumber))
	}
	if pi.TimeoutTimestamp > 0 {
		fields = append(fields, zap.Uint64("timeout_timestamp", pi.TimeoutTimestamp))
	}
	acp.logObservedIBCMessage(message, fields...)
}

func (acp *AvalancheChainProcessor) logChannelOpenMessage(message string, ci provider.ChannelInfo) {
	fields := []zap.Field{
		zap.String("channel_id", ci.ChannelID),
		zap.String("connection_id", ci.ConnID),
		zap.String("port_id", ci.PortID),
	}
	acp.log.Info("Successfully created new channel", fields...)
}

func (acp *AvalancheChainProcessor) logConnectionMessage(message string, ci provider.ConnectionInfo) {
	acp.logObservedIBCMessage(message,
		zap.String("client_id", ci.ClientID),
		zap.String("connection_id", ci.ConnID),
		zap.String("counterparty_client_id", ci.CounterpartyClientID),
		zap.String("counterparty_connection_id", ci.CounterpartyConnID),
	)
}

func (acp *AvalancheChainProcessor) logObservedIBCMessage(m string, fields ...zap.Field) {
	acp.log.With(zap.String("event_type", m)).Debug("Observed IBC message", fields...)
}
