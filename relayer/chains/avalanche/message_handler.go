package avalanche

import (
	"context"

	"go.uber.org/zap"

	"github.com/cosmos/relayer/v2/relayer/processor"
)

func (acp *AvalancheChainProcessor) handleMessage(ctx context.Context, m ibcMessage, c processor.IBCMessagesCache) {
	switch t := m.info.(type) {
	case *clientInfo:
		acp.handleClientMessage(ctx, m.eventType, *t)
	}
}

func (acp *AvalancheChainProcessor) handleClientMessage(ctx context.Context, eventType string, ci clientInfo) {
	acp.latestClientState.update(ctx, ci, acp)
	acp.logObservedIBCMessage(eventType, zap.String("client_id", ci.clientID))
}

func (acp *AvalancheChainProcessor) logObservedIBCMessage(m string, fields ...zap.Field) {
	acp.log.With(zap.String("event_type", m)).Debug("Observed IBC message", fields...)
}
