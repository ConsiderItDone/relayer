package avalanche

import (
	"context"
	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"

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
