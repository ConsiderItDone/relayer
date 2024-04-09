package avalanche

import (
	"context"
	"errors"
	"math/big"
	"time"

	"github.com/ava-labs/avalanchego/utils"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp/payload"
	"github.com/ava-labs/subnet-evm/accounts/abi/bind"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	transfertypes "github.com/cosmos/ibc-go/v7/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v7/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	ibcexported "github.com/cosmos/ibc-go/v7/modules/core/exported"
	tendermint "github.com/cosmos/ibc-go/v7/modules/light-clients/07-tendermint"
	avalanche "github.com/cosmos/ibc-go/v7/modules/light-clients/14-avalanche"
	"golang.org/x/exp/maps"

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
	// query EVM header by number
	ethHeader, err := a.ethClient.HeaderByNumber(ctx, big.NewInt(h))
	if err != nil {
		return nil, err
	}

	validatorSet, vdrs, err := a.avalancheValidatorSet(ctx, ethHeader.Number.Uint64())

	signedStorageRoot, _, err := a.avalancheBlsSignature(ctx, ethHeader.Root.Bytes())
	signedValidatorSet, signers, err := a.avalancheBlsSignature(ctx, validatorSet)

	return AvalancheIBCHeader{
		EthHeader:          ethHeader,
		SignedStorageRoot:  signedStorageRoot,
		SignedValidatorSet: signedValidatorSet,
		ValidatorSet:       validatorSet,
		Vdrs:               vdrs,
		SignersInput:       signers,
	}, nil

}

func (a AvalancheProvider) avalancheValidatorSet(ctx context.Context, evmHeight uint64) ([]byte, []*avalanche.Validator, error) {
	// query P-Chain block number by EVM height
	pChainHeight, err := a.ibcClient.GetPChainHeight(ctx, evmHeight)
	if err != nil {
		return nil, nil, err
	}

	// query P-Chain validators at specific height
	vdrSet, err := a.pClient.GetValidatorsAt(ctx, a.subnetID, pChainHeight)
	if err != nil {
		return nil, nil, err
	}

	vdrs := make(map[string]*Validator, len(vdrSet))
	for _, vdr := range vdrSet {
		if vdr.PublicKey == nil {
			continue
		}

		pkBytes := bls.SerializePublicKey(vdr.PublicKey)
		uniqueVdr, ok := vdrs[string(pkBytes)]
		if !ok {
			uniqueVdr = &Validator{
				PublicKey:      vdr.PublicKey,
				PublicKeyBytes: pkBytes,
			}
			vdrs[string(pkBytes)] = uniqueVdr
		}

		uniqueVdr.Weight += vdr.Weight // Impossible to overflow here
		uniqueVdr.NodeIDs = append(uniqueVdr.NodeIDs, vdr.NodeID)
	}
	// Sort validators by public key
	vdrList := maps.Values(vdrs)
	utils.Sort(vdrList)

	var avaVldrs []*avalanche.Validator
	for _, v := range vdrList {
		avaVldrs = append(avaVldrs, &avalanche.Validator{
			PublicKeyByte: v.PublicKeyBytes,
			Weight:        v.Weight,
			NodeIDs:       [][]byte{v.NodeIDs[0].Bytes()},
			EndTime:       time.Time{},
		})
	}

	// Avalanche validator set in binary format
	var avaVldrsBz []byte
	for _, vldr := range avaVldrs {
		data, err := vldr.Marshal()
		if err != nil {
			return nil, nil, err
		}
		avaVldrsBz = append(avaVldrsBz, data...)
	}

	return avaVldrsBz, avaVldrs, nil
}

func (a AvalancheProvider) avalancheBlsSignature(ctx context.Context, payloadData []byte) ([bls.SignatureLen]byte, []byte, error) {
	addressedPayload, err := payload.NewAddressedCall(
		[]byte{},
		payloadData,
	)
	if err != nil {
		return [96]byte{}, nil, err
	}
	unsignedMessage, err := warp.NewUnsignedMessage(a.PCfg.NetworkID, a.blockchainID, addressedPayload.Bytes())
	if err != nil {
		return [96]byte{}, nil, err
	}
	signedWarpMessageBytes, err := a.ibcClient.GetMessageAggregateSignature(ctx, unsignedMessage.ID(), 3, a.PCfg.SubnetID)
	if err != nil {
		return [96]byte{}, nil, err
	}

	warpMsg, err := warp.ParseMessage(signedWarpMessageBytes)
	if err != nil {
		return [96]byte{}, nil, err
	}

	bitsetSignature, ok := warpMsg.Signature.(*warp.BitSetSignature)
	if !ok {
		return [96]byte{}, nil, errors.New("unable to cast warp signature to BitSetSignature")
	}
	return bitsetSignature.Signature, bitsetSignature.Signers, nil
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
	return 0, nil
}

func (a AvalancheProvider) QueryClientState(ctx context.Context, height int64, clientid string) (ibcexported.ClientState, error) {
	clientStateBz, err := a.ibcContract.QueryClientState(&bind.CallOpts{BlockNumber: big.NewInt(height)}, clientid)
	if err != nil {
		return nil, err
	}

	// check if client exists
	if len(clientStateBz) == 0 {
		return nil, sdkerrors.Wrap(clienttypes.ErrClientNotFound, clientid)
	}

	tmClientState := tendermint.ClientState{}
	err = tmClientState.Unmarshal(clientStateBz)
	if err != nil {
		return nil, err
	}

	return &tmClientState, nil
}

func (a AvalancheProvider) QueryClientStateResponse(ctx context.Context, height int64, srcClientId string) (*clienttypes.QueryClientStateResponse, error) {
	clientStateBz, err := a.ibcContract.QueryClientState(&bind.CallOpts{BlockNumber: big.NewInt(height)}, srcClientId)
	if err != nil {
		return nil, err
	}

	// check if client exists
	if len(clientStateBz) == 0 {
		return nil, sdkerrors.Wrap(clienttypes.ErrClientNotFound, srcClientId)
	}

	tmClientState := tendermint.ClientState{}
	err = tmClientState.Unmarshal(clientStateBz)
	if err != nil {
		return nil, err
	}

	anyClientState, err := clienttypes.PackClientState(&tmClientState)
	if err != nil {
		return nil, err
	}

	return &clienttypes.QueryClientStateResponse{
		ClientState: anyClientState,
		Proof:       nil,
		ProofHeight: clienttypes.Height{},
	}, nil
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
	return conns, nil
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
	chans := []*chantypes.IdentifiedChannel{}

	return chans, nil
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
