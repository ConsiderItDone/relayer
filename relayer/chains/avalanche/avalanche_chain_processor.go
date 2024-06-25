package avalanche

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ava-labs/subnet-evm/core/types"
	"github.com/avast/retry-go/v4"
	clienttypes "github.com/cosmos/ibc-go/v8/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v8/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v8/modules/core/04-channel/types"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/cosmos/relayer/v2/relayer/provider"
)

const (
	queryTimeout                = 5 * time.Second
	latestHeightQueryRetryDelay = 1 * time.Second
	latestHeightQueryRetries    = 5
	blockResultsQueryTimeout    = 2 * time.Minute

	defaultMinQueryLoopDuration = 1 * time.Second
	inSyncNumBlocksThreshold    = 2
	blockMaxRetries             = 5
)

// latestClientState is a map of clientID to the latest clientInfo for that client.
type latestClientState map[string]provider.ClientState

func (l latestClientState) update(ctx context.Context, clientInfo clientInfo, acp *AvalancheChainProcessor) {
	existingClientInfo, ok := l[clientInfo.clientID]
	if ok && clientInfo.consensusHeight.LT(existingClientInfo.ConsensusHeight) {
		// height is less than latest, so no-op
		return
	}

	// TODO: don't hardcode
	tp := time.Hour * 2
	clientState := clientInfo.ClientState(tp)

	// update latest if no existing state or provided consensus height is newer
	l[clientInfo.clientID] = clientState
}

type AvalancheChainProcessor struct {
	log *zap.Logger

	chainProvider *AvalancheProvider

	pathProcessors processor.PathProcessors

	// indicates whether queries are in sync with latest height of the chain
	inSync bool

	// highest block
	latestBlock provider.LatestBlock

	// holds highest consensus height and header for all clients
	latestClientState

	// holds open state for known connections
	connectionStateCache processor.ConnectionStateCache

	// holds open state for known channels
	channelStateCache processor.ChannelStateCache

	// map of connection ID to client ID
	connectionClients map[string]string

	// map of channel ID to connection ID
	channelConnections map[string]string
}

func NewAvalancheChainProcessor(log *zap.Logger, provider *AvalancheProvider) *AvalancheChainProcessor {
	return &AvalancheChainProcessor{
		log:                  log.With(zap.String("chain_name", provider.ChainName()), zap.String("chain_id", provider.ChainId())),
		chainProvider:        provider,
		latestClientState:    make(latestClientState),
		connectionStateCache: make(processor.ConnectionStateCache),
		channelStateCache:    make(processor.ChannelStateCache),
		connectionClients:    make(map[string]string),
		channelConnections:   make(map[string]string),
	}
}

// queryCyclePersistence hold the variables that should be retained across queryCycles.
type queryCyclePersistence struct {
	latestHeight                int64
	latestQueriedBlock          int64
	retriesAtLatestQueriedBlock int
	minQueryLoopDuration        time.Duration
	lastBalanceUpdate           time.Time
	balanceUpdateWaitDuration   time.Duration
}

// Run starts the query loop for the chain which will gather applicable ibc messages and push events out to the relevant PathProcessors.
// The initialBlockHistory parameter determines how many historical blocks should be fetched and processed before continuing with current blocks.
// ChainProcessors should obey the context and return upon context cancellation.
func (acp *AvalancheChainProcessor) Run(ctx context.Context, initialBlockHistory uint64, stuckPacket *processor.StuckPacket) error {
	minQueryLoopDuration := acp.chainProvider.PCfg.MinLoopDuration
	if minQueryLoopDuration == 0 {
		minQueryLoopDuration = defaultMinQueryLoopDuration
	}

	// this will be used for persistence across query cycle loop executions
	persistence := queryCyclePersistence{
		minQueryLoopDuration: minQueryLoopDuration,
	}

	// Infinite retry to get initial latest height
	for {
		latestHeight, err := acp.latestHeightWithRetry(ctx)
		if err != nil {
			acp.log.Error(
				"Failed to query latest height after max attempts",
				zap.Uint("attempts", latestHeightQueryRetries),
				zap.Error(err),
			)
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil
			}
			continue
		}
		persistence.latestHeight = latestHeight
		break
	}

	// this will make initial QueryLoop iteration look back initialBlockHistory blocks in history
	latestQueriedBlock := persistence.latestHeight - int64(initialBlockHistory)

	if latestQueriedBlock < 0 {
		latestQueriedBlock = 0
	}

	persistence.latestQueriedBlock = latestQueriedBlock

	var eg errgroup.Group
	eg.Go(func() error {
		return acp.initializeConnectionState(ctx)
	})
	eg.Go(func() error {
		return acp.initializeChannelState(ctx)
	})
	if err := eg.Wait(); err != nil {
		return err
	}

	acp.log.Debug("Entering main query loop")

	ticker := time.NewTicker(persistence.minQueryLoopDuration)

	for {
		if err := acp.queryCycle(ctx, &persistence); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			ticker.Reset(persistence.minQueryLoopDuration)
		}
	}
}

// Provider returns the ChainProvider, which provides the methods for querying, assembling IBC messages, and sending transactions.
func (acp *AvalancheChainProcessor) Provider() provider.ChainProvider {
	return acp.chainProvider
}

// SetPathProcessors sets path processor for ChainProcessor should publish relevant IBC events to.
// ChainProcessors need reference to their PathProcessors and vice-versa, handled by EventProcessorBuilder.Build().
func (acp *AvalancheChainProcessor) SetPathProcessors(pathProcessors processor.PathProcessors) {
	acp.pathProcessors = pathProcessors
}

// latestHeightWithRetry will query for the latest height, retrying in case of failure.
// It will delay by latestHeightQueryRetryDelay between attempts, up to latestHeightQueryRetries.
func (acp *AvalancheChainProcessor) latestHeightWithRetry(ctx context.Context) (latestHeight int64, err error) {
	return latestHeight, retry.Do(func() error {
		latestHeightQueryCtx, cancelLatestHeightQueryCtx := context.WithTimeout(ctx, queryTimeout)
		defer cancelLatestHeightQueryCtx()
		var err error
		latestHeight, err = acp.chainProvider.QueryLatestHeight(latestHeightQueryCtx)
		return err
	}, retry.Context(ctx), retry.Attempts(latestHeightQueryRetries), retry.Delay(latestHeightQueryRetryDelay), retry.LastErrorOnly(true), retry.OnRetry(func(n uint, err error) {
		acp.log.Info(
			"Failed to query latest height",
			zap.Uint("attempt", n+1),
			zap.Uint("max_attempts", latestHeightQueryRetries),
			zap.Error(err),
		)
	}))
}

// initializeConnectionState will bootstrap the connectionStateCache with the open connection state.
func (acp *AvalancheChainProcessor) initializeConnectionState(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	connections, err := acp.chainProvider.QueryConnections(ctx)
	if err != nil {
		return fmt.Errorf("error querying connections: %w", err)
	}
	for _, c := range connections {
		acp.connectionClients[c.Id] = c.ClientId
		acp.connectionStateCache[processor.ConnectionKey{
			ConnectionID:         c.Id,
			ClientID:             c.ClientId,
			CounterpartyConnID:   c.Counterparty.ConnectionId,
			CounterpartyClientID: c.Counterparty.ClientId,
		}] = c.State == conntypes.OPEN
	}
	return nil
}

// initializeChannelState will bootstrap the channelStateCache with the open channel state.
func (acp *AvalancheChainProcessor) initializeChannelState(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	channels, err := acp.chainProvider.QueryChannels(ctx)
	if err != nil {
		return fmt.Errorf("error querying channels: %w", err)
	}
	for _, ch := range channels {
		if len(ch.ConnectionHops) != 1 {
			acp.log.Error("Found channel using multiple connection hops. Not currently supported, ignoring.",
				zap.String("channel_id", ch.ChannelId),
				zap.String("port_id", ch.PortId),
				zap.Any("connection_hops", ch.ConnectionHops),
			)
			continue
		}
		acp.channelConnections[ch.ChannelId] = ch.ConnectionHops[0]
		k := processor.ChannelKey{
			ChannelID:             ch.ChannelId,
			PortID:                ch.PortId,
			CounterpartyChannelID: ch.Counterparty.ChannelId,
			CounterpartyPortID:    ch.Counterparty.PortId,
		}
		acp.channelStateCache.SetOpen(k, ch.State == chantypes.OPEN, ch.Ordering)
	}
	return nil
}

func (acp *AvalancheChainProcessor) queryCycle(ctx context.Context, persistence *queryCyclePersistence) error {
	var err error
	latestHeight, err := acp.latestHeightWithRetry(ctx)
	if err != nil {
		acp.log.Error(
			"Failed to query latest height after max attempts",
			zap.Uint("attempts", latestHeightQueryRetries),
			zap.Error(err),
		)
		return nil
	}

	persistence.latestHeight = latestHeight

	acp.log.Debug("Queried latest height",
		zap.Int64("latest_queried_block", persistence.latestQueriedBlock),
		zap.Int64("latest_height", persistence.latestHeight),
	)

	// used at the end of the cycle to send signal to path processors to start processing if both chains are in sync and no new messages came in this cycle
	firstTimeInSync := false

	if !acp.inSync {
		if (persistence.latestHeight - persistence.latestQueriedBlock) < inSyncNumBlocksThreshold {
			acp.inSync = true
			firstTimeInSync = true
			acp.log.Info("Chain is in sync")
		} else {
			acp.log.Info("Chain is not yet in sync",
				zap.Int64("latest_queried_block", persistence.latestQueriedBlock),
				zap.Int64("latest_height", persistence.latestHeight),
			)
		}
	}

	ibcMessagesCache := processor.NewIBCMessagesCache()

	ibcHeaderCache := make(processor.IBCHeaderCache)

	ppChanged := false

	var latestHeader AvalancheIBCHeader

	newLatestQueriedBlock := persistence.latestQueriedBlock

	chainID := acp.chainProvider.ChainId()

	for i := persistence.latestQueriedBlock + 1; i <= persistence.latestHeight; i++ {
		var eg errgroup.Group
		var blockRes *types.Block
		var receipts []*types.Receipt
		var ibcHeader provider.IBCHeader
		i := i
		eg.Go(func() (err error) {
			queryCtx, cancelQueryCtx := context.WithTimeout(ctx, blockResultsQueryTimeout)
			defer cancelQueryCtx()
			blockRes, err = acp.chainProvider.ethClient.BlockByNumber(queryCtx, big.NewInt(i))
			if err != nil {
				return err
			}
			for _, tx := range blockRes.Transactions() {
				receipt, err := acp.chainProvider.ethClient.TransactionReceipt(queryCtx, tx.Hash())
				if err != nil {
					acp.log.Error("Error fetching tx receipt",
						zap.String("tx_hash", tx.Hash().String()),
						zap.Error(err),
					)
					return err
				}
				receipts = append(receipts, receipt)
			}
			return nil
		})
		eg.Go(func() (err error) {
			queryCtx, cancelQueryCtx := context.WithTimeout(ctx, queryTimeout)
			defer cancelQueryCtx()
			ibcHeader, err = acp.chainProvider.QueryIBCHeader(queryCtx, i)
			return err
		})

		if err := eg.Wait(); err != nil {
			acp.log.Warn("Error querying block data",
				zap.Int64("height", i),
				zap.Error(err),
			)

			persistence.retriesAtLatestQueriedBlock++
			if persistence.retriesAtLatestQueriedBlock >= blockMaxRetries {
				acp.log.Warn("Reached max retries querying for block, skipping", zap.Int64("height", i))
				// skip this block. now depends on flush to pickup anything missed in the block.
				persistence.latestQueriedBlock = i
				persistence.retriesAtLatestQueriedBlock = 0
				continue
			}
			break
		}

		acp.log.Debug(
			"Queried block",
			zap.Int64("height", i),
			zap.Int64("latest", persistence.latestHeight),
			zap.Int64("delta", persistence.latestHeight-i),
		)

		persistence.retriesAtLatestQueriedBlock = 0

		var ok bool
		latestHeader, ok = ibcHeader.(AvalancheIBCHeader)
		if !ok {
			return fmt.Errorf("bad header: %#v", ibcHeader)
		}

		heightUint64 := uint64(i)

		acp.latestBlock = provider.LatestBlock{
			Height: heightUint64,
			Time:   time.Unix(int64(blockRes.Time()), 0),
		}

		ibcHeaderCache[heightUint64] = latestHeader
		ppChanged = true

		//blockMsgs := acp.ibcMessagesFromBlockEvents(blockRes.Transactions())
		//for _, m := range blockMsgs {
		//	acp.handleMessage(m, ibcMessagesCache)
		//}

		for _, receipt := range receipts {
			if receipt.Status != types.ReceiptStatusSuccessful {
				// tx was not successful
				continue
			}
			events, err := parseEventsFromTxReceipt(acp.chainProvider.abi, receipt)
			if err != nil {
				acp.log.Warn("Error parsing events",
					zap.Int64("height", i),
					zap.String("tx_hash", receipt.TxHash.String()),
					zap.Error(err),
				)

				continue
			}
			messages := ibcMessagesFromEvents(acp.log, events, heightUint64)

			for _, m := range messages {
				acp.handleMessage(ctx, m, ibcMessagesCache)
			}
		}
		newLatestQueriedBlock = i
	}

	if newLatestQueriedBlock == persistence.latestQueriedBlock /*&& !firstTimeInSync */ {
		return nil
	}

	if !ppChanged {
		if firstTimeInSync {
			for _, pp := range acp.pathProcessors {
				pp.ProcessBacklogIfReady()
			}
		}

		return nil
	}

	for _, pp := range acp.pathProcessors {
		clientID := pp.RelevantClientID(chainID)
		clientState, err := acp.clientState(ctx, clientID)
		if err != nil {
			acp.log.Error("Error fetching client state",
				zap.String("client_id", clientID),
				zap.Error(err),
			)
			continue
		}

		pp.HandleNewData(chainID, processor.ChainProcessorCacheData{
			LatestBlock:          acp.latestBlock,
			LatestHeader:         latestHeader,
			IBCMessagesCache:     ibcMessagesCache.Clone(),
			InSync:               acp.inSync,
			ClientState:          clientState,
			ConnectionStateCache: acp.connectionStateCache.FilterForClient(clientID),
			ChannelStateCache:    acp.channelStateCache.FilterForClient(clientID, acp.channelConnections, acp.connectionClients),
			IBCHeaderCache:       ibcHeaderCache.Clone(),
		})
	}

	persistence.latestQueriedBlock = newLatestQueriedBlock

	return nil
}

// clientState will return the most recent client state if client messages
// have already been observed for the clientID, otherwise it will query for it.
func (acp *AvalancheChainProcessor) clientState(ctx context.Context, clientID string) (provider.ClientState, error) {
	if state, ok := acp.latestClientState[clientID]; ok {
		return state, nil
	}
	cs, err := acp.chainProvider.QueryClientState(ctx, int64(acp.latestBlock.Height), clientID)
	if err != nil {
		return provider.ClientState{}, err
	}
	clientState := provider.ClientState{
		ClientID:        clientID,
		ConsensusHeight: cs.GetLatestHeight().(clienttypes.Height),
	}
	acp.latestClientState[clientID] = clientState
	return clientState, nil
}
