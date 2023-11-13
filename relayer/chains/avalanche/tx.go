package avalanche

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ava-labs/subnet-evm/accounts/abi/bind"
	evmtypes "github.com/ava-labs/subnet-evm/core/types"
	"github.com/ava-labs/subnet-evm/interfaces"
	"github.com/ava-labs/subnet-evm/precompile/contracts/ibc"
	"github.com/avast/retry-go/v4"
	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v7/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	commitmenttypes "github.com/cosmos/ibc-go/v7/modules/core/23-commitment/types"
	ibcexported "github.com/cosmos/ibc-go/v7/modules/core/exported"
	avaclient "github.com/cosmos/ibc-go/v7/modules/light-clients/14-avalanche"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"go.uber.org/zap"

	"github.com/cosmos/relayer/v2/relayer/provider"
)

// Default IBC settings
var (
	defaultChainPrefix = commitmenttypes.NewMerklePrefix([]byte("ibc"))
	defaultDelayPeriod = uint32(0)
)

func (a AvalancheProvider) SendMessage(ctx context.Context, msg provider.RelayerMessage, memo string) (*provider.RelayerTxResponse, bool, error) {
	return a.SendMessages(ctx, []provider.RelayerMessage{msg}, memo)
}

func (a AvalancheProvider) broadcastTx(
	ctx context.Context, // context for tx broadcast
	signedTx *evmtypes.Transaction,
	asyncCtx context.Context, // context for async wait for block inclusion after successful tx broadcast
	asyncCallbacks []func(*provider.RelayerTxResponse, error), // callback for success/fail of the wait for block inclusion
) error {
	err := a.ethClient.SendTransaction(ctx, signedTx)
	if err != nil {
		return err
	}

	go a.waitForTx(asyncCtx, signedTx.Hash(), asyncCallbacks)

	return nil
}

func (a AvalancheProvider) waitForTx(
	ctx context.Context,
	txHash common.Hash,
	callbacks []func(*provider.RelayerTxResponse, error),
) {
	var receipt *evmtypes.Receipt
	err := retry.Do(
		func() error {
			rc, err := a.ethClient.TransactionReceipt(ctx, txHash)
			if err != nil {
				return err
			}
			receipt = rc
			return nil
		},
		retry.Delay(1*time.Second),
		retry.Attempts(10),
	)
	if err != nil {
		a.log.Error("Failed to wait for block inclusion", zap.Error(err))
		if len(callbacks) > 0 {
			for _, cb := range callbacks {
				//Call each callback in order since waitForTx is already invoked asyncronously
				cb(nil, err)
			}
		}
		return
	}
	// trying to parse events. The problem with that we need to know exactly event name to be able to parse it
	events, err := parseEventsFromTxReceipt(a.abi, receipt)
	if err != nil {
		a.log.Error("Failed to parse receipt events", zap.Error(err))
		if len(callbacks) > 0 {
			for _, cb := range callbacks {
				//Call each callback in order since waitForTx is already invoked asyncronously
				cb(nil, err)
			}
		}
		return
	}

	rlyResp := &provider.RelayerTxResponse{
		Height:    receipt.BlockNumber.Int64(),
		TxHash:    receipt.TxHash.String(),
		Codespace: "",
		Code:      0,
		Data:      "",
		Events:    events,
	}

	if len(callbacks) > 0 {
		for _, cb := range callbacks {
			//Call each callback in order since waitForTx is already invoked asyncronously
			cb(rlyResp, nil)
		}
	}
}

func (a AvalancheProvider) SendMessages(ctx context.Context, msgs []provider.RelayerMessage, memo string) (*provider.RelayerTxResponse, bool, error) {
	var (
		rlyResp     *provider.RelayerTxResponse
		callbackErr error
		wg          sync.WaitGroup
	)

	callback := func(rtr *provider.RelayerTxResponse, err error) {
		rlyResp = rtr
		callbackErr = err
		wg.Done()
	}

	wg.Add(1)

	if err := retry.Do(func() error {
		return a.SendMessagesToMempool(ctx, msgs, memo, ctx, []func(*provider.RelayerTxResponse, error){callback})
	}, retry.Context(ctx), rtyAtt, rtyDel, rtyErr, retry.OnRetry(func(n uint, err error) {
		a.log.Info(
			"Error building or broadcasting transaction",
			zap.String("chain_id", a.PCfg.ChainID),
			zap.Uint("attempt", n+1),
			zap.Uint("max_attempts", rtyAttNum),
			zap.Error(err),
		)
	})); err != nil {
		return nil, false, err
	}

	wg.Wait()

	if callbackErr != nil {
		return rlyResp, false, callbackErr
	}

	if rlyResp.Code != 0 {
		return rlyResp, false, fmt.Errorf("transaction failed with code: %d", rlyResp.Code)
	}

	return rlyResp, true, callbackErr
}

func (a AvalancheProvider) SendMessagesToMempool(ctx context.Context, msgs []provider.RelayerMessage, memo string, asyncCtx context.Context, asyncCallbacks []func(*provider.RelayerTxResponse, error)) error {
	if len(msgs) == 0 {
		return errors.New("empty messages")
	}

	msg := msgs[0]

	input, err := msg.MsgBytes()
	if err != nil {
		return err
	}
	signedTx, err := a.signTx(input)
	if err != nil {
		return err
	}

	if err := a.broadcastTx(ctx, signedTx, asyncCtx, asyncCallbacks); err != nil {
		return err
	}

	return nil
}

func (a AvalancheProvider) createDynamicTx(opts *bind.TransactOpts, contract *common.Address, input []byte, head *evmtypes.Header) (*evmtypes.Transaction, error) {
	// Normalize value
	value := opts.Value
	if value == nil {
		value = new(big.Int)
	}
	// Estimate TipCap
	gasTipCap := opts.GasTipCap
	if gasTipCap == nil {
		tip, err := a.ethClient.SuggestGasTipCap(ensureContext(opts.Context))
		if err != nil {
			return nil, err
		}
		gasTipCap = tip
	}
	// Estimate FeeCap
	gasFeeCap := opts.GasFeeCap
	if gasFeeCap == nil {
		gasFeeCap = new(big.Int).Add(
			gasTipCap,
			new(big.Int).Mul(head.BaseFee, big.NewInt(2)),
		)
	}
	if gasFeeCap.Cmp(gasTipCap) < 0 {
		return nil, fmt.Errorf("maxFeePerGas (%v) < maxPriorityFeePerGas (%v)", gasFeeCap, gasTipCap)
	}
	// Estimate GasLimit
	gasLimit := opts.GasLimit
	if opts.GasLimit == 0 {
		var err error
		gasLimit, err = a.estimateGasLimit(opts, contract, input, nil, gasTipCap, gasFeeCap, value)
		if err != nil {
			return nil, err
		}
	}
	// create the transaction
	nonce, err := a.getNonce(opts)
	if err != nil {
		return nil, err
	}
	baseTx := &evmtypes.DynamicFeeTx{
		To:        contract,
		Nonce:     nonce,
		GasFeeCap: gasFeeCap,
		GasTipCap: gasTipCap,
		Gas:       gasLimit,
		Value:     value,
		Data:      input,
	}
	return evmtypes.NewTx(baseTx), nil
}

func (a AvalancheProvider) createLegacyTx(opts *bind.TransactOpts, contract *common.Address, input []byte) (*evmtypes.Transaction, error) {
	if opts.GasFeeCap != nil || opts.GasTipCap != nil {
		return nil, errors.New("maxFeePerGas or maxPriorityFeePerGas specified but london is not active yet")
	}
	// Normalize value
	value := opts.Value
	if value == nil {
		value = new(big.Int)
	}
	// Estimate GasPrice
	gasPrice := opts.GasPrice
	if gasPrice == nil {
		price, err := a.ethClient.SuggestGasPrice(ensureContext(opts.Context))
		if err != nil {
			return nil, err
		}
		gasPrice = price
	}
	// Estimate GasLimit
	gasLimit := opts.GasLimit
	if opts.GasLimit == 0 {
		var err error
		gasLimit, err = a.estimateGasLimit(opts, contract, input, gasPrice, nil, nil, value)
		if err != nil {
			return nil, err
		}
	}
	// create the transaction
	nonce, err := a.getNonce(opts)
	if err != nil {
		return nil, err
	}
	baseTx := &evmtypes.LegacyTx{
		To:       contract,
		Nonce:    nonce,
		GasPrice: gasPrice,
		Gas:      gasLimit,
		Value:    value,
		Data:     input,
	}
	return evmtypes.NewTx(baseTx), nil
}

func (a AvalancheProvider) getNonce(opts *bind.TransactOpts) (uint64, error) {
	if opts.Nonce == nil {
		return a.ethClient.AcceptedNonceAt(ensureContext(opts.Context), opts.From)
	} else {
		return opts.Nonce.Uint64(), nil
	}
}

func (a AvalancheProvider) estimateGasLimit(opts *bind.TransactOpts, contract *common.Address, input []byte, gasPrice, gasTipCap, gasFeeCap, value *big.Int) (uint64, error) {
	msg := interfaces.CallMsg{
		From:      opts.From,
		To:        contract,
		GasPrice:  gasPrice,
		GasTipCap: gasTipCap,
		GasFeeCap: gasFeeCap,
		Value:     value,
		Data:      input,
	}
	return a.ethClient.EstimateGas(ensureContext(opts.Context), msg)
}

func (a AvalancheProvider) signTx(input []byte) (*evmtypes.Transaction, error) {
	// Create the transaction
	var (
		rawTx *evmtypes.Transaction
		err   error
	)
	contractAddress := common.HexToAddress(a.PCfg.ContractAddress)
	// get last header
	if head, errHead := a.ethClient.HeaderByNumber(ensureContext(a.txAuth.Context), nil); errHead != nil {
		return nil, errHead
	} else if head.BaseFee != nil {
		rawTx, err = a.createDynamicTx(a.txAuth, &contractAddress, input, head)
	} else {
		// Chain is not London ready -> use legacy transaction
		rawTx, err = a.createLegacyTx(a.txAuth, &contractAddress, input)
	}
	if err != nil {
		return nil, err
	}
	// Sign the transaction and schedule it for execution
	if a.txAuth.Signer == nil {
		return nil, errors.New("no signer to authorize the transaction with")
	}
	signedTx, err := a.txAuth.Signer(a.txAuth.From, rawTx)

	return signedTx, err
}

// DefaultUpgradePath is the default IBC upgrade path set for an on-chain light client
var defaultUpgradePath = "upgrade"

func (a AvalancheProvider) NewClientState(
	dstChainID string,
	dstUpdateHeader provider.IBCHeader,
	dstTrustingPeriod,
	dstUbdPeriod time.Duration,
	allowUpdateAfterExpiry,
	allowUpdateAfterMisbehaviour bool,
) (ibcexported.ClientState, error) {
	revisionNumber := clienttypes.ParseChainID(dstChainID)

	trustLevel := avaclient.Fraction{
		Numerator:   1,
		Denominator: 3,
	}

	// Create the ClientState we want on 'c' tracking 'dst'
	return &avaclient.ClientState{
		ChainId:        dstChainID,
		TrustLevel:     trustLevel,
		TrustingPeriod: dstTrustingPeriod,
		MaxClockDrift:  time.Minute * 10,
		FrozenHeight:   clienttypes.ZeroHeight(),
		LatestHeight: clienttypes.Height{
			RevisionNumber: revisionNumber,
			RevisionHeight: dstUpdateHeader.Height(),
		},
		UpgradePath:                  defaultUpgradePath,
		AllowUpdateAfterExpiry:       allowUpdateAfterExpiry,
		AllowUpdateAfterMisbehaviour: allowUpdateAfterMisbehaviour,
	}, nil
}

func (a AvalancheProvider) MsgCreateClient(clientState ibcexported.ClientState, consensusState ibcexported.ConsensusState) (provider.RelayerMessage, error) {
	anyClientState, err := clienttypes.PackClientState(clientState)
	if err != nil {
		return nil, err
	}
	clientStateBz, err := anyClientState.Marshal()
	if err != nil {
		return nil, err
	}

	anyConsensusState, err := clienttypes.PackConsensusState(consensusState)
	if err != nil {
		return nil, err
	}
	consensusStateBz, err := anyConsensusState.Marshal()
	if err != nil {
		return nil, err
	}

	input := ibc.CreateClientInput{
		ClientType:     clientState.ClientType(),
		ClientState:    clientStateBz,
		ConsensusState: consensusStateBz,
	}

	msg, err := ibc.PackCreateClient(input)
	if err != nil {
		return nil, err
	}

	return NewEVMMessage(msg), nil
}

func (a AvalancheProvider) MsgUpgradeClient(srcClientId string, consRes *clienttypes.QueryConsensusStateResponse, clientRes *clienttypes.QueryClientStateResponse) (provider.RelayerMessage, error) {
	input := ibc.UpgradeClientInput{
		ClientID:           srcClientId,
		ProofUpgradeClient: consRes.GetProof(),
	}
	msg, err := ibc.PackUpgradeClient(input)
	if err != nil {
		return nil, err
	}

	return NewEVMMessage(msg), nil
}

func (a AvalancheProvider) MsgUpdateClient(clientID string, dstHeader ibcexported.ClientMessage) (provider.RelayerMessage, error) {
	clientMsg, err := clienttypes.PackClientMessage(dstHeader)
	if err != nil {
		return nil, err
	}
	clientMsgBz, err := clientMsg.Marshal()
	if err != nil {
		return nil, err
	}

	input := ibc.UpdateClientInput{
		ClientID:      clientID,
		ClientMessage: clientMsgBz,
	}

	msg, err := ibc.PackUpdateClient(input)
	if err != nil {
		return nil, err
	}

	return NewEVMMessage(msg), nil
}

func (a AvalancheProvider) MsgChannelOpenInit(info provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	channel := chantypes.Channel{
		State:    chantypes.INIT,
		Ordering: info.Order,
		Counterparty: chantypes.Counterparty{
			PortId:    info.CounterpartyPortID,
			ChannelId: "",
		},
		ConnectionHops: []string{info.ConnID},
		Version:        info.Version,
	}
	channelBz, err := channel.Marshal()
	if err != nil {
		return nil, err
	}

	input := ibc.ChanOpenInitInput{
		PortID:  info.PortID,
		Channel: channelBz,
	}

	msg, err := ibc.PackChanOpenInit(input)
	if err != nil {
		return nil, err
	}

	return NewEVMMessage(msg), nil
}

func (a AvalancheProvider) MsgChannelOpenTry(msgOpenInit provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	channel := chantypes.Channel{
		State:    chantypes.TRYOPEN,
		Ordering: proof.Ordering,
		Counterparty: chantypes.Counterparty{
			PortId:    msgOpenInit.PortID,
			ChannelId: msgOpenInit.ChannelID,
		},
		ConnectionHops: []string{msgOpenInit.CounterpartyConnID},
		Version:        proof.Version,
	}
	channelBz, err := channel.Marshal()
	if err != nil {
		return nil, err
	}

	heightBz, err := proof.ProofHeight.Marshal()
	if err != nil {
		return nil, err
	}

	input := ibc.ChanOpenTryInput{
		PortID:              msgOpenInit.CounterpartyPortID,
		Channel:             channelBz,
		CounterpartyVersion: proof.Version,
		ProofInit:           proof.Proof,
		ProofHeight:         heightBz,
	}

	msg, err := ibc.PackChanOpenTry(input)
	if err != nil {
		return nil, err
	}

	return NewEVMMessage(msg), nil
}

func (a AvalancheProvider) MsgChannelOpenAck(msgOpenTry provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	heightBz, err := proof.ProofHeight.Marshal()
	if err != nil {
		return nil, err
	}

	input := ibc.ChannelOpenAckInput{
		PortID:                msgOpenTry.CounterpartyPortID,
		ChannelID:             msgOpenTry.CounterpartyChannelID,
		CounterpartyChannelID: msgOpenTry.ChannelID,
		CounterpartyVersion:   proof.Version,
		ProofTry:              proof.Proof,
		ProofHeight:           heightBz,
	}

	msg, err := ibc.PackChannelOpenAck(input)
	if err != nil {
		return nil, err
	}

	return NewEVMMessage(msg), nil
}

func (a AvalancheProvider) MsgChannelOpenConfirm(msgOpenAck provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	heightBz, err := proof.ProofHeight.Marshal()
	if err != nil {
		return nil, err
	}

	input := ibc.ChannelOpenConfirmInput{
		PortID:      msgOpenAck.CounterpartyPortID,
		ChannelID:   msgOpenAck.CounterpartyChannelID,
		ProofAck:    proof.Proof,
		ProofHeight: heightBz,
	}

	msg, err := ibc.PackChannelOpenConfirm(input)
	if err != nil {
		return nil, err
	}

	return NewEVMMessage(msg), nil
}

func (a AvalancheProvider) MsgChannelCloseInit(info provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	input := ibc.ChannelCloseInitInput{
		PortID:    info.PortID,
		ChannelID: info.ChannelID,
	}

	msg, err := ibc.PackChannelCloseInit(input)
	if err != nil {
		return nil, err
	}

	return NewEVMMessage(msg), nil
}

func (a AvalancheProvider) MsgChannelCloseConfirm(msgCloseInit provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	heightBz, err := proof.ProofHeight.Marshal()
	if err != nil {
		return nil, err
	}
	input := ibc.ChannelCloseConfirmInput{
		PortID:      msgCloseInit.CounterpartyPortID,
		ChannelID:   msgCloseInit.CounterpartyChannelID,
		ProofInit:   proof.Proof,
		ProofHeight: heightBz,
	}

	msg, err := ibc.PackChannelCloseConfirm(input)
	if err != nil {
		return nil, err
	}

	return NewEVMMessage(msg), nil
}

func (a AvalancheProvider) MsgConnectionOpenTry(msgOpenInit provider.ConnectionInfo, proof provider.ConnectionProof) (provider.RelayerMessage, error) {
	counterparty := conntypes.Counterparty{
		ClientId:     msgOpenInit.ClientID,
		ConnectionId: msgOpenInit.ConnID,
		Prefix:       defaultChainPrefix,
	}
	counterpartyBz, err := counterparty.Marshal()
	if err != nil {
		return nil, err
	}

	csAny, err := clienttypes.PackClientState(proof.ClientState)
	if err != nil {
		return nil, err
	}
	csBz, err := csAny.Marshal()
	if err != nil {
		return nil, err
	}

	consensusHeight := proof.ClientState.GetLatestHeight().(clienttypes.Height)
	consensusHeightBz, err := consensusHeight.Marshal()
	if err != nil {
		return nil, err
	}

	proofHeightBz, err := proof.ProofHeight.Marshal()
	if err != nil {
		return nil, err
	}

	input := ibc.ConnOpenTryInput{
		Counterparty:    counterpartyBz,
		DelayPeriod:     defaultDelayPeriod,
		ClientID:        msgOpenInit.CounterpartyClientID,
		ClientState:     csBz,
		ProofInit:       proof.ConnectionStateProof,
		ProofClient:     proof.ClientStateProof,
		ProofConsensus:  proof.ConsensusStateProof,
		ProofHeight:     proofHeightBz,
		ConsensusHeight: consensusHeightBz,
	}

	msg, err := ibc.PackConnOpenTry(input)

	if err != nil {
		return nil, err
	}

	return NewEVMMessage(msg), nil
}

func (a AvalancheProvider) BlockTime(ctx context.Context, height int64) (time.Time, error) {
	block, err := a.ethClient.BlockByNumber(ctx, big.NewInt(height))
	if err != nil {
		return time.Time{}, err
	}

	return time.Unix(int64(block.Time()), 0), nil
}

func (a AvalancheProvider) MsgConnectionOpenAck(msgOpenTry provider.ConnectionInfo, proof provider.ConnectionProof) (provider.RelayerMessage, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) MsgConnectionOpenConfirm(msgOpenAck provider.ConnectionInfo, proof provider.ConnectionProof) (provider.RelayerMessage, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) MsgSubmitMisbehaviour(clientID string, misbehaviour ibcexported.ClientMessage) (provider.RelayerMessage, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) ValidatePacket(msgTransfer provider.PacketInfo, latestBlock provider.LatestBlock) error {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) PacketCommitment(ctx context.Context, msgTransfer provider.PacketInfo, height uint64) (provider.PacketProof, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) PacketAcknowledgement(ctx context.Context, msgRecvPacket provider.PacketInfo, height uint64) (provider.PacketProof, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) PacketReceipt(ctx context.Context, msgTransfer provider.PacketInfo, height uint64) (provider.PacketProof, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) NextSeqRecv(ctx context.Context, msgTransfer provider.PacketInfo, height uint64) (provider.PacketProof, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) MsgTransfer(dstAddr string, amount sdk.Coin, info provider.PacketInfo) (provider.RelayerMessage, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) MsgRecvPacket(msgTransfer provider.PacketInfo, proof provider.PacketProof) (provider.RelayerMessage, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) MsgAcknowledgement(msgRecvPacket provider.PacketInfo, proofAcked provider.PacketProof) (provider.RelayerMessage, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) MsgTimeout(msgTransfer provider.PacketInfo, proofUnreceived provider.PacketProof) (provider.RelayerMessage, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) MsgTimeoutOnClose(msgTransfer provider.PacketInfo, proofUnreceived provider.PacketProof) (provider.RelayerMessage, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) CommitmentPrefix() commitmenttypes.MerklePrefix {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) ConnectionHandshakeProof(ctx context.Context, msgOpenInit provider.ConnectionInfo, height uint64) (provider.ConnectionProof, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) ConnectionProof(ctx context.Context, msgOpenAck provider.ConnectionInfo, height uint64) (provider.ConnectionProof, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) MsgConnectionOpenInit(info provider.ConnectionInfo, proof provider.ConnectionProof) (provider.RelayerMessage, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) ChannelProof(ctx context.Context, msg provider.ChannelInfo, height uint64) (provider.ChannelProof, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) MsgUpdateClientHeader(latestHeader provider.IBCHeader, trustedHeight clienttypes.Height, trustedHeader provider.IBCHeader) (ibcexported.ClientMessage, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) QueryICQWithProof(ctx context.Context, msgType string, request []byte, height uint64) (provider.ICQProof, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) MsgSubmitQueryResponse(chainID string, queryID provider.ClientICQQueryID, proof provider.ICQProof) (provider.RelayerMessage, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) RelayPacketFromSequence(ctx context.Context, src provider.ChainProvider, srch, dsth, seq uint64, srcChanID, srcPortID string, order chantypes.Order) (provider.RelayerMessage, provider.RelayerMessage, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) AcknowledgementFromSequence(ctx context.Context, dst provider.ChainProvider, dsth, seq uint64, dstChanID, dstPortID, srcChanID, srcPortID string) (provider.RelayerMessage, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) MsgRegisterCounterpartyPayee(portID, channelID, relayerAddr, counterpartyPayeeAddr string) (provider.RelayerMessage, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) ChainName() string {
	return a.PCfg.ChainName
}

func (a AvalancheProvider) ChainId() string {
	return a.PCfg.ChainID
}

func (a AvalancheProvider) Type() string {
	return "avalanche"
}

func (a AvalancheProvider) ProviderConfig() provider.ProviderConfig {
	return a.PCfg
}

// TODO: use json-file as a key storage.
func (a AvalancheProvider) Key() string {
	return "someKey"
}

// TODO: use json-file as a key storage.
// See https://github.com/ethereum/go-ethereum/blob/bc6d184872889224480cf9df58b0539b210ffa9e/cmd/ethkey/inspect.go#L61
func (a AvalancheProvider) Address() (string, error) {
	keyAddr := crypto.PubkeyToAddress(tempKey.PublicKey)

	return keyAddr.Hex(), nil
}

func (a AvalancheProvider) Timeout() string {
	return a.PCfg.Timeout
}

func (a AvalancheProvider) TrustingPeriod(ctx context.Context) (time.Duration, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) WaitForNBlocks(ctx context.Context, n int64) error {
	//TODO implement me
	panic("implement me")
}
