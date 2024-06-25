package avalanche

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"cosmossdk.io/math"
	"github.com/ava-labs/subnet-evm/interfaces"
	"github.com/ava-labs/subnet-evm/precompile/contracts/ibc"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/ethereum/go-ethereum/common"

	"github.com/ava-labs/avalanchego/utils"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp/payload"
	"github.com/ava-labs/subnet-evm/accounts/abi/bind"
	sdk "github.com/cosmos/cosmos-sdk/types"
	transfertypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v8/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v8/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v8/modules/core/04-channel/types"
	ibcexported "github.com/cosmos/ibc-go/v8/modules/core/exported"
	tendermint "github.com/cosmos/ibc-go/v8/modules/light-clients/07-tendermint"
	avalanche "github.com/cosmos/ibc-go/v8/modules/light-clients/14-avalanche"
	ibccontract "github.com/cosmos/relayer/v2/relayer/chains/avalanche/ibc"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"golang.org/x/exp/maps"
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

	validatorSet, vdrs, pChainHeight, err := a.avalancheValidatorSet(ctx, ethHeader.Number.Uint64())

	signedStorageRoot, _, err := a.avalancheBlsSignature(ctx, ethHeader.Root.Bytes())
	if err != nil {
		return nil, err
	}
	signedValidatorSet, signers, err := a.avalancheBlsSignature(ctx, validatorSet)
	if err != nil {
		return nil, err
	}

	return AvalancheIBCHeader{
		EthHeader:          ethHeader,
		SignedStorageRoot:  signedStorageRoot,
		SignedValidatorSet: signedValidatorSet,
		ValidatorSet:       validatorSet,
		Vdrs:               vdrs,
		SignersInput:       signers,
		PChainHeight:       pChainHeight,
	}, nil

}

func (a AvalancheProvider) avalancheValidatorSet(ctx context.Context, evmHeight uint64) ([]byte, []*avalanche.Validator, uint64, error) {
	// query P-Chain block number by EVM height
	pChainHeight, err := a.ibcClient.GetPChainHeight(ctx, evmHeight)
	if err != nil {
		return nil, nil, 0, err
	}

	// query P-Chain validators at specific height
	vdrSet, err := a.pClient.GetValidatorsAt(ctx, a.subnetID, pChainHeight)
	if err != nil {
		return nil, nil, 0, err
	}

	vdrs := make(map[string]*Validator, len(vdrSet))
	for _, vdr := range vdrSet {
		if vdr.PublicKey == nil {
			continue
		}

		pkBytes := bls.PublicKeyToBytes(vdr.PublicKey)
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
			return nil, nil, 0, err
		}
		avaVldrsBz = append(avaVldrsBz, data...)
	}

	return avaVldrsBz, avaVldrs, pChainHeight, nil
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
	abi, err := ibccontract.IBCMetaData.GetAbi()
	if err != nil {
		return provider.PacketInfo{}, err
	}

	EventPacketSent, exist := abi.Events["PacketSent"]
	if !exist {
		return provider.PacketInfo{}, fmt.Errorf("event PacketRecv not found in abi")
	}

	logs, err := a.ethClient.FilterLogs(ctx, interfaces.FilterQuery{
		Addresses: []common.Address{
			ibc.ContractAddress,
		},
		FromBlock: nil,
		ToBlock:   nil,
		Topics: [][]common.Hash{
			{EventPacketSent.ID},
		},
	})

	var event *ibccontract.IBCPacketSent = nil
	for i := range logs {
		e, err := a.ibcContract.ParsePacketSent(logs[i])
		if err == nil && e.Sequence.Uint64() == sequence {
			event = e
		}
	}

	if event == nil {
		return provider.PacketInfo{}, fmt.Errorf("event PacketRecv[seq=%d] not found", sequence)
	}

	ordering := ""
	switch event.ChannelOrdering {
	default:
		ordering = "ORDER_NONE_UNSPECIFIED"
	case 1:
		ordering = "ORDER_UNORDERED"
	case 2:
		ordering = "ORDER_ORDERED"
	}

	timeoutSplit := strings.Split(event.TimeoutHeight, "-")
	if len(timeoutSplit) != 2 {
		return provider.PacketInfo{}, fmt.Errorf("bad TimeoutHeight value: '%s'", event.TimeoutHeight)
	}
	revisionNumber, err := strconv.ParseUint(timeoutSplit[0], 10, 64)
	if err != nil {
		return provider.PacketInfo{}, fmt.Errorf("bad TimeoutHeight value: '%s'", event.TimeoutHeight)
	}
	revisionHeight, err := strconv.ParseUint(timeoutSplit[1], 10, 64)
	if err != nil {
		return provider.PacketInfo{}, fmt.Errorf("bad TimeoutHeight value: '%s'", event.TimeoutHeight)
	}
	return provider.PacketInfo{
		Sequence:      sequence,
		SourcePort:    event.SourcePort,
		SourceChannel: event.SourceChannel,
		DestPort:      event.DestPort,
		DestChannel:   event.DestChannel,
		ChannelOrder:  ordering,
		Data:          event.Data,
		TimeoutHeight: clienttypes.Height{
			RevisionHeight: revisionHeight,
			RevisionNumber: revisionNumber,
		},
		TimeoutTimestamp: event.TimeoutTimestamp.Uint64(),
	}, nil
}

func (a AvalancheProvider) QueryRecvPacket(ctx context.Context, dstChanID, dstPortID string, sequence uint64) (provider.PacketInfo, error) {
	abi, err := ibccontract.IBCMetaData.GetAbi()
	if err != nil {
		return provider.PacketInfo{}, err
	}

	EventPacketRecv, exist := abi.Events["PacketRecv"]
	if !exist {
		return provider.PacketInfo{}, fmt.Errorf("event PacketRecv not found in abi")
	}

	logs, err := a.ethClient.FilterLogs(ctx, interfaces.FilterQuery{
		Addresses: []common.Address{
			ibc.ContractAddress,
		},
		FromBlock: nil,
		ToBlock:   nil,
		Topics: [][]common.Hash{
			{EventPacketRecv.ID},
		},
	})

	var event *ibccontract.IBCPacketRecv = nil
	for i := range logs {
		e, err := a.ibcContract.ParsePacketRecv(logs[i])
		if err == nil && e.Sequence.Uint64() == sequence {
			event = e
		}
	}

	if event == nil {
		return provider.PacketInfo{}, fmt.Errorf("event PacketRecv[seq=%d] not found", sequence)
	}

	ordering := ""
	switch event.ChannelOrdering {
	default:
		ordering = "ORDER_NONE_UNSPECIFIED"
	case 1:
		ordering = "ORDER_UNORDERED"
	case 2:
		ordering = "ORDER_ORDERED"
	}

	timeoutSplit := strings.Split(event.TimeoutHeight, "-")
	if len(timeoutSplit) != 2 {
		return provider.PacketInfo{}, fmt.Errorf("bad TimeoutHeight value: '%s'", event.TimeoutHeight)
	}
	revisionNumber, err := strconv.ParseUint(timeoutSplit[0], 10, 64)
	if err != nil {
		return provider.PacketInfo{}, fmt.Errorf("bad TimeoutHeight value: '%s'", event.TimeoutHeight)
	}
	revisionHeight, err := strconv.ParseUint(timeoutSplit[1], 10, 64)
	if err != nil {
		return provider.PacketInfo{}, fmt.Errorf("bad TimeoutHeight value: '%s'", event.TimeoutHeight)
	}
	return provider.PacketInfo{
		Sequence:      sequence,
		SourcePort:    event.SourcePort,
		SourceChannel: event.SourceChannel,
		DestPort:      event.DestPort,
		DestChannel:   event.DestChannel,
		ChannelOrder:  ordering,
		Data:          event.Data,
		TimeoutHeight: clienttypes.Height{
			RevisionHeight: revisionHeight,
			RevisionNumber: revisionNumber,
		},
		TimeoutTimestamp: event.TimeoutTimestamp.Uint64(),
	}, nil
}

func (a AvalancheProvider) QueryBalance(ctx context.Context, keyName string) (sdk.Coins, error) {
	return sdk.Coins{sdk.NewCoin("avax", math.NewInt(4))}, nil
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
		return nil, clienttypes.ErrClientNotFound
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
		return nil, clienttypes.ErrClientNotFound
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

func (a AvalancheProvider) QueryConnections(ctx context.Context) ([]*conntypes.IdentifiedConnection, error) {
	rawConnections, err := a.ibcContract.QueryConnectionAll(nil)
	if err != nil {
		return nil, err
	}
	var rawdata [][]byte
	if err := json.Unmarshal(rawConnections, &rawdata); err != nil {
		return nil, err
	}
	conns := make([]*conntypes.IdentifiedConnection, len(rawdata))
	for i := range rawdata {
		var conn conntypes.ConnectionEnd
		if err := conn.Unmarshal(rawdata[i]); err != nil {
			return nil, err
		}
		conns[i] = &conntypes.IdentifiedConnection{
			Id:           conn.Counterparty.ConnectionId,
			ClientId:     conn.ClientId,
			Versions:     conn.Versions,
			State:        conn.State,
			Counterparty: conn.Counterparty,
			DelayPeriod:  conn.DelayPeriod,
		}
	}

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
	rawdata, err := a.ibcContract.QueryChannel(&bind.CallOpts{BlockNumber: big.NewInt(height)}, portid, channelid)
	if err != nil {
		return nil, err
	}

	var ch chantypes.Channel
	if err := ch.Unmarshal(rawdata); err != nil {
		return nil, err
	}

	proof, err := a.ChannelProof(ctx, provider.ChannelInfo{PortID: portid, ChannelID: channelid}, uint64(height))
	if err != nil {
		return nil, err
	}

	return &chantypes.QueryChannelResponse{
		Channel:     &ch,
		Proof:       proof.Proof,
		ProofHeight: proof.ProofHeight,
	}, nil
}

func (a AvalancheProvider) QueryChannelClient(ctx context.Context, height int64, channelid, portid string) (*clienttypes.IdentifiedClientState, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) QueryConnectionChannels(ctx context.Context, height int64, connectionid string) ([]*chantypes.IdentifiedChannel, error) {
	rawdata, err := a.ibcContract.QueryChannelAll(nil)
	if err != nil {
		if err.Error() == "empty precompile state" {
			return []*chantypes.IdentifiedChannel{}, nil
		}
		return nil, fmt.Errorf("here: %w", err)
	}

	var rawlist [][]byte
	if err := json.Unmarshal(rawdata, &rawlist); err != nil {
		return nil, err
	}

	chans := make([]*chantypes.IdentifiedChannel, 0)
	for i := range rawlist {
		var ch chantypes.Channel
		if err := ch.Unmarshal(rawlist[i]); err != nil {
			return nil, err
		}
		if ch.ConnectionHops[0] == connectionid {
			chans = append(chans, &chantypes.IdentifiedChannel{
				State:          ch.State,
				Ordering:       ch.Ordering,
				Counterparty:   ch.Counterparty,
				ConnectionHops: ch.ConnectionHops,
				Version:        ch.Version,
				PortId:         ch.Counterparty.PortId,
				ChannelId:      ch.Counterparty.ChannelId,
			})
		}
	}

	return chans, nil
}

func (a AvalancheProvider) QueryChannels(ctx context.Context) ([]*chantypes.IdentifiedChannel, error) {
	rawdata, err := a.ibcContract.QueryChannelAll(nil)
	if err != nil {
		if err.Error() == "empty precompile state" {
			return []*chantypes.IdentifiedChannel{}, nil
		}
		return nil, fmt.Errorf("here: %w", err)
	}

	var rawlist [][]byte
	if err := json.Unmarshal(rawdata, &rawlist); err != nil {
		return nil, err
	}

	chans := make([]*chantypes.IdentifiedChannel, len(rawlist))
	for i := range rawlist {
		var ch chantypes.Channel
		if err := ch.Unmarshal(rawlist[i]); err != nil {
			return nil, err
		}
		chans[i] = &chantypes.IdentifiedChannel{
			State:          ch.State,
			Ordering:       ch.Ordering,
			Counterparty:   ch.Counterparty,
			ConnectionHops: ch.ConnectionHops,
			Version:        ch.Version,
			PortId:         ch.Counterparty.PortId,
			ChannelId:      ch.Counterparty.ChannelId,
		}
	}

	return chans, nil
}

func (a AvalancheProvider) QueryPacketCommitments(ctx context.Context, height uint64, channelid, portid string) (commitments *chantypes.QueryPacketCommitmentsResponse, err error) {
	rawdata, err := a.ibcContract.QueryPacketCommitments(&bind.CallOpts{BlockNumber: big.NewInt(int64(height))}, portid, channelid)
	if err != nil {
		if err.Error() == "empty precompile state" {
			return &chantypes.QueryPacketCommitmentsResponse{
				Pagination: &query.PageResponse{
					Total: 0,
				},
				Height: clienttypes.Height{
					RevisionNumber: 0,
					RevisionHeight: height,
				},
				Commitments: make([]*chantypes.PacketState, 0),
			}, nil
		}
		return nil, err
	}

	var data []struct {
		Commitment []byte
		Sequence   uint64
	}
	if err := json.Unmarshal(rawdata, &data); err != nil {
		return nil, err
	}

	res := chantypes.QueryPacketCommitmentsResponse{
		Pagination: &query.PageResponse{
			Total: uint64(len(data)),
		},
		Height: clienttypes.Height{
			RevisionNumber: 0,
			RevisionHeight: height,
		},
		Commitments: make([]*chantypes.PacketState, len(data)),
	}

	for i := range data {
		res.Commitments[i] = &chantypes.PacketState{
			PortId:    portid,
			ChannelId: channelid,
			Sequence:  data[i].Sequence,
			Data:      data[i].Commitment,
		}
	}

	return &res, nil
}

func (a AvalancheProvider) QueryPacketAcknowledgements(ctx context.Context, height uint64, channelid, portid string) (acknowledgements []*chantypes.PacketState, err error) {
	rawdata, err := a.ibcContract.QueryPacketAcknowledgements(&bind.CallOpts{BlockNumber: big.NewInt(int64(height))}, portid, channelid)
	if err != nil {
		return nil, err
	}

	var data []struct {
		Acknowledgement []byte
		Sequence        uint64
	}
	if err := json.Unmarshal(rawdata, &data); err != nil {
		return nil, err
	}

	acks := make([]*chantypes.PacketState, len(data))
	for i := range data {
		acks[i] = &chantypes.PacketState{
			PortId:    portid,
			ChannelId: channelid,
			Sequence:  data[i].Sequence,
			Data:      data[i].Acknowledgement,
		}
	}

	return acks, nil
}

func (a AvalancheProvider) QueryUnreceivedPackets(ctx context.Context, height uint64, channelid, portid string, seqs []uint64) ([]uint64, error) {
	seqsdata := make([]*big.Int, len(seqs))
	for i := range seqs {
		seqsdata[i] = new(big.Int).SetUint64(seqs[i])
	}
	rawdata, err := a.ibcContract.QueryUnreceivedPackets(&bind.CallOpts{BlockNumber: big.NewInt(int64(height))}, portid, channelid, seqsdata)
	if err != nil {
		return nil, err
	}
	resp := make([]uint64, len(rawdata.Seqs))
	for i := range rawdata.Seqs {
		resp[i] = rawdata.Seqs[i].Uint64()
	}
	return resp, nil
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

func (a AvalancheProvider) QueryDenomHash(ctx context.Context, trace string) (string, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) QueryDenomTrace(ctx context.Context, denom string) (*transfertypes.DenomTrace, error) {
	//TODO implement me
	panic("implement me")
}

func (a AvalancheProvider) QueryDenomTraces(ctx context.Context, offset, limit uint64, height int64) ([]transfertypes.DenomTrace, error) {
	//TODO implement me
	return []transfertypes.DenomTrace{
		transfertypes.DenomTrace{
			"demo2",
			"ETH",
		},
	}, nil
}
