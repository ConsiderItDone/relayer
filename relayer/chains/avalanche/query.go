package avalanche

import (
	"context"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	transfertypes "github.com/cosmos/ibc-go/v7/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v7/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	ibcexported "github.com/cosmos/ibc-go/v7/modules/core/exported"

	"github.com/cosmos/relayer/v2/relayer/provider"
)

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
