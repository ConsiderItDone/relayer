package avalanche

import (
	"errors"
	"fmt"

	"github.com/ava-labs/subnet-evm/accounts/abi"
	evmtypes "github.com/ava-labs/subnet-evm/core/types"
	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"

	"github.com/cosmos/relayer/v2/relayer/provider"
)

const (
	eventClientCreated     = "ClientCreated"
	eventConnectionCreated = "ConnectionCreated"
)

var (
	errNoEventSignature = errors.New("no event signature")

	eventNames = []string{
		eventClientCreated,
		eventConnectionCreated,
	}
)

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
