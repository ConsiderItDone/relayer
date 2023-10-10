package avalanche

import (
	"context"
	"errors"
	"math/big"
	"net/url"
	"time"

	"github.com/ava-labs/subnet-evm/precompile/contracts/ibc"
	"github.com/ava-labs/subnet-evm/rpc"
	sdk "github.com/cosmos/cosmos-sdk/types"
	proto "github.com/cosmos/gogoproto/proto"
	transfertypes "github.com/cosmos/ibc-go/v7/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v7/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	commitmenttypes "github.com/cosmos/ibc-go/v7/modules/core/23-commitment/types"
	ibcexported "github.com/cosmos/ibc-go/v7/modules/core/exported"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"go.uber.org/zap"

	"github.com/ava-labs/subnet-evm/accounts/abi/bind"
	evmtypes "github.com/ava-labs/subnet-evm/core/types"
	"github.com/ava-labs/subnet-evm/ethclient"
	"github.com/ava-labs/subnet-evm/ethclient/subnetevmclient"

	"github.com/cosmos/relayer/v2/relayer/provider"
)

var (
	_ provider.ChainProvider  = &AvalancheProvider{}
	_ provider.KeyProvider    = &AvalancheProvider{}
	_ provider.ProviderConfig = &AvalancheProviderConfig{}

	tempKey, _      = crypto.HexToECDSA("56289e99c94b6912bfc12adc093c9b51124f0dc54ac7a766b2bc5ccf558d8027")
	contractAddress = common.HexToAddress("0x0300000000000000000000000000000000000002")
)

type AvalancheProviderConfig struct {
	RPCAddr         string `json:"rpc-addr" yaml:"rpc-addr"`
	ChainID         string `json:"chain-id" yaml:"chain-id"`
	ChainName       string `json:"-" yaml:"-"`
	Timeout         string `json:"timeout" yaml:"timeout"`
	ContractAddress string `json:"contract-address" yaml:"contract-address"`
}

func (a AvalancheProviderConfig) NewProvider(log *zap.Logger, homepath string, debug bool, chainName string) (provider.ChainProvider, error) {
	a.ChainName = chainName

	return &AvalancheProvider{
		PCfg: a,
	}, nil
}

func (a AvalancheProviderConfig) Validate() error {
	_, err := url.Parse(a.RPCAddr)
	if err != nil {
		return err
	}

	return nil
}

func (a AvalancheProviderConfig) BroadcastMode() provider.BroadcastMode {
	return provider.BroadcastModeSingle
}

type AvalancheProvider struct {
	PCfg AvalancheProviderConfig

	ethClient    ethclient.Client
	subnetClient *subnetevmclient.Client
	txAuth       *bind.TransactOpts
}

func (a AvalancheProvider) BlockTime(ctx context.Context, height int64) (time.Time, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) QueryTx(ctx context.Context, hashHex string) (*provider.RelayerTxResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) QueryTxs(ctx context.Context, page, limit int, events []string) ([]*provider.RelayerTxResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) QueryLatestHeight(ctx context.Context) (int64, error) {
	blockNumber, err := a.ethClient.BlockNumber(context.Background())

	return int64(blockNumber), err
}

func (a AvalancheProvider) QueryIBCHeader(ctx context.Context, h int64) (provider.IBCHeader, error) {

	//type ConsensusState struct {
	//	// timestamp that corresponds to the block height in which the ConsensusState
	//	// was stored.
	//	Timestamp          time.Time    `protobuf:"bytes,1,opt,name=timestamp,proto3,stdtime" json:"timestamp"`
	//	StorageRoot        []byte       `protobuf:"bytes,2,opt,name=storage_root,json=storageRoot,proto3" json:"storage_root,omitempty"`
	//	SignedStorageRoot  []byte       `protobuf:"bytes,3,opt,name=signed_storage_root,json=signedStorageRoot,proto3" json:"signed_storage_root,omitempty"`
	//	ValidatorSet       []byte       `protobuf:"bytes,4,opt,name=validator_set,json=validatorSet,proto3" json:"validator_set,omitempty"`
	//	SignedValidatorSet []byte       `protobuf:"bytes,5,opt,name=signed_validator_set,json=signedValidatorSet,proto3" json:"signed_validator_set,omitempty"`
	//	Vdrs               []*Validator `protobuf:"bytes,6,rep,name=vdrs,proto3" json:"vdrs,omitempty"`
	//	SignersInput       []byte       `protobuf:"bytes,8,opt,name=signers_input,json=signersInput,proto3" json:"signers_input,omitempty"`
	//}

	//b, _ := a.ethClient.BlockByNumber(context.Background(), big.NewInt(1))
	//timestamp := b.Timestamp()
	//storageRoot := b.Root()
	//validatorSet := a.pClient.getCurrentValidators(a.PCfg.ChainID ? or a.PCfg.SubnetId)
	//for i, validator := range validatorSet {
	//	blsClient := ibc2.NewIbcClient(validatorURL, a.PCfg.SubnetId)
	//	signedStorageRoot_i := blsClient.GetBlsProof(context.Background(), storageRoot)
	//	signedVS_i := blsClient.GetBlsProof(context.Background(), validatorSet)
	//}
	//
	//signedStorageRoot, err := bls.AggregateSignatures([]*bls.Signature{signedStorageRoot_0, signedStorageRoot_1, signedStorageRoot_2})
	//SignedValidatorSet := bls.AggregateSignatures([]*bls.Signature{signedVS_0, signedVS_1})
	//Vdrs := ??
	//SignersInput := ??

	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) QuerySendPacket(ctx context.Context, srcChanID, srcPortID string, sequence uint64) (provider.PacketInfo, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) QueryRecvPacket(ctx context.Context, dstChanID, dstPortID string, sequence uint64) (provider.PacketInfo, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) QueryBalance(ctx context.Context, keyName string) (sdk.Coins, error) {
	return sdk.Coins{sdk.NewCoin("avax", sdk.NewInt(4))}, nil
}

func (a AvalancheProvider) QueryBalanceWithAddress(ctx context.Context, addr string) (sdk.Coins, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) QueryUnbondingPeriod(ctx context.Context) (time.Duration, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) QueryClientState(ctx context.Context, height int64, clientid string) (ibcexported.ClientState, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) QueryClientStateResponse(ctx context.Context, height int64, srcClientId string) (*clienttypes.QueryClientStateResponse, error) {

	// query client state
	//clientState := ibcContract.GetClientState(clientId) // TODO @ramil
	//
	//// query client state proof
	//clientStateSlots := ibc.GetClientStateSlots(a.subnetClient, ibc.ContractAddress, srcClientId)
	//
	//keys := make([]string, 0)
	//for _, slot := range clientStateSlots {
	//	keys = append(keys, slot.Hex())
	//}
	//
	//clientStateProof, err := a.subnetClient.GetProof(context.Background(), ibc.ContractAddress, keys, nil)
	//
	//return &clienttypes.QueryClientStateResponse{
	//	ClientState: clientState,
	//	Proof:       clientStateProof.StorageProof,
	//	ProofHeight: clientStateProof.Height, // TODO @ramil
	//}
	panic("implement me")

}

func (a AvalancheProvider) QueryClientConsensusState(ctx context.Context, chainHeight int64, clientid string, clientHeight ibcexported.Height) (*clienttypes.QueryConsensusStateResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) QueryUpgradedClient(ctx context.Context, height int64) (*clienttypes.QueryClientStateResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) QueryUpgradedConsState(ctx context.Context, height int64) (*clienttypes.QueryConsensusStateResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) QueryConsensusState(ctx context.Context, height int64) (ibcexported.ConsensusState, int64, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) QueryClients(ctx context.Context) (clienttypes.IdentifiedClientStates, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) QueryConnection(ctx context.Context, height int64, connectionid string) (*conntypes.QueryConnectionResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) QueryConnections(ctx context.Context) (conns []*conntypes.IdentifiedConnection, err error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) QueryConnectionsUsingClient(ctx context.Context, height int64, clientid string) (*conntypes.QueryConnectionsResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) GenerateConnHandshakeProof(ctx context.Context, height int64, clientId, connId string) (clientState ibcexported.ClientState, clientStateProof []byte, consensusProof []byte, connectionProof []byte, connectionProofHeight ibcexported.Height, err error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) QueryChannel(ctx context.Context, height int64, channelid, portid string) (chanRes *chantypes.QueryChannelResponse, err error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) QueryChannelClient(ctx context.Context, height int64, channelid, portid string) (*clienttypes.IdentifiedClientState, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) QueryConnectionChannels(ctx context.Context, height int64, connectionid string) ([]*chantypes.IdentifiedChannel, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) QueryChannels(ctx context.Context) ([]*chantypes.IdentifiedChannel, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) QueryPacketCommitments(ctx context.Context, height uint64, channelid, portid string) (commitments *chantypes.QueryPacketCommitmentsResponse, err error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) QueryPacketAcknowledgements(ctx context.Context, height uint64, channelid, portid string) (acknowledgements []*chantypes.PacketState, err error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) QueryUnreceivedPackets(ctx context.Context, height uint64, channelid, portid string, seqs []uint64) ([]uint64, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) QueryUnreceivedAcknowledgements(ctx context.Context, height uint64, channelid, portid string, seqs []uint64) ([]uint64, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) QueryNextSeqRecv(ctx context.Context, height int64, channelid, portid string) (recvRes *chantypes.QueryNextSequenceReceiveResponse, err error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) QueryNextSeqAck(ctx context.Context, height int64, channelid, portid string) (recvRes *chantypes.QueryNextSequenceReceiveResponse, err error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) QueryPacketCommitment(ctx context.Context, height int64, channelid, portid string, seq uint64) (comRes *chantypes.QueryPacketCommitmentResponse, err error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) QueryPacketAcknowledgement(ctx context.Context, height int64, channelid, portid string, seq uint64) (ackRes *chantypes.QueryPacketAcknowledgementResponse, err error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) QueryPacketReceipt(ctx context.Context, height int64, channelid, portid string, seq uint64) (recRes *chantypes.QueryPacketReceiptResponse, err error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) QueryDenomTrace(ctx context.Context, denom string) (*transfertypes.DenomTrace, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) QueryDenomTraces(ctx context.Context, offset, limit uint64, height int64) ([]transfertypes.DenomTrace, error) {
	//TODO implement me
	panic("implement me")
}

func (a *AvalancheProvider) Init(ctx context.Context) error {
	rpcClient, err := rpc.DialContext(context.Background(), a.PCfg.RPCAddr)
	if err != nil {
		return err
	}
	a.ethClient = ethclient.NewClient(rpcClient)
	a.subnetClient = subnetevmclient.New(rpcClient)

	chainId, _ := new(big.Int).SetString(a.PCfg.ChainID, 10)

	a.txAuth, err = bind.NewKeyedTransactorWithChainID(tempKey, chainId)

	return nil
}

func (a AvalancheProvider) NewClientState(dstChainID string, dstIBCHeader provider.IBCHeader, dstTrustingPeriod, dstUbdPeriod time.Duration, allowUpdateAfterExpiry, allowUpdateAfterMisbehaviour bool) (ibcexported.ClientState, error) {
	//TODO implement me
	panic("implement me")
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

	baseTx := &evmtypes.LegacyTx{
		To:       contract,
		Nonce:    0,
		GasPrice: new(big.Int),
		Gas:      0,
		Value:    new(big.Int),
		Data:     input,
	}
	return evmtypes.NewTx(baseTx), nil
}

func (a AvalancheProvider) estimateGasLimit(opts *bind.TransactOpts, contract *common.Address, input []byte, gasPrice, gasTipCap, gasFeeCap, value *big.Int) (uint64, error) {
	if contract != nil {
		// Gas estimation cannot succeed without code for method invocations.
		if code, err := a.ethClient.AcceptedCodeAt(ensureContext(opts.Context), address); err != nil {
			return 0, err
		} else if len(code) == 0 {
			return 0, ErrNoCode
		}
	}
	msg := interfaces.CallMsg{
		From:      opts.From,
		To:        contract,
		GasPrice:  gasPrice,
		GasTipCap: gasTipCap,
		GasFeeCap: gasFeeCap,
		Value:     value,
		Data:      input,
	}
	return c.transactor.EstimateGas(ensureContext(opts.Context), msg)
}

func (a AvalancheProvider) signTx(input []byte) (*evmtypes.Transaction, error) {
	// Create the transaction
	var (
		rawTx *evmtypes.Transaction
		err   error
	)
	// get last header
	// TODO use context?
	if head, errHead := a.ethClient.BlockByNumber(context.Background(), nil); errHead != nil {
		return nil, errHead
	} else if head.BaseFee() != nil {
		rawTx, err = a.createDynamicTx(opts, contract, input, head)
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

	//signedTx, err := a.signTx(msg)
	//if err != nil {
	//	return nil, err
	//}

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

	//signedTx, err := a.signTx(msg)
	//if err != nil {
	//	return nil, err
	//}

	return NewEVMMessage(msg), nil
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

func (a AvalancheProvider) MsgConnectionOpenTry(msgOpenInit provider.ConnectionInfo, proof provider.ConnectionProof) (provider.RelayerMessage, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) MsgConnectionOpenAck(msgOpenTry provider.ConnectionInfo, proof provider.ConnectionProof) (provider.RelayerMessage, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) MsgConnectionOpenConfirm(msgOpenAck provider.ConnectionInfo, proof provider.ConnectionProof) (provider.RelayerMessage, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) ChannelProof(ctx context.Context, msg provider.ChannelInfo, height uint64) (provider.ChannelProof, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) MsgChannelOpenInit(info provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) MsgChannelOpenTry(msgOpenInit provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) MsgChannelOpenAck(msgOpenTry provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) MsgChannelOpenConfirm(msgOpenAck provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) MsgChannelCloseInit(info provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) MsgChannelCloseConfirm(msgCloseInit provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) MsgUpdateClientHeader(latestHeader provider.IBCHeader, trustedHeight clienttypes.Height, trustedHeader provider.IBCHeader) (ibcexported.ClientMessage, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) MsgUpdateClient(clientID string, counterpartyHeader ibcexported.ClientMessage) (provider.RelayerMessage, error) {
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

func (a AvalancheProvider) SendMessage(ctx context.Context, msg provider.RelayerMessage, memo string) (*provider.RelayerTxResponse, bool, error) {
	input := msg.MsgBytes()
	a.signTx(input)
}

func (a AvalancheProvider) SendMessages(ctx context.Context, msgs []provider.RelayerMessage, memo string) (*provider.RelayerTxResponse, bool, error) {
	//for i, msg := range msgs {
	//	// generate tx
	//	signedTx := abiContract.Transact(....)
	//	a.ethClient.SendTransaction(context.Background(), signedTx)
	//}
	panic("implement me")
}

func (a AvalancheProvider) SendMessagesToMempool(ctx context.Context, msgs []provider.RelayerMessage, memo string, asyncCtx context.Context, asyncCallbacks []func(*provider.RelayerTxResponse, error)) error {
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

func (a AvalancheProvider) Sprint(toPrint proto.Message) (string, error) {
	//TODO implement me
	panic("implement me")
}
