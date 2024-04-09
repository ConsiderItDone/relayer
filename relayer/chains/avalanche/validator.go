package avalanche

import (
	"bytes"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
)

var (
	_ utils.Sortable[*Validator] = (*Validator)(nil)
)

type Validator struct {
	PublicKey      *bls.PublicKey
	PublicKeyBytes []byte
	Weight         uint64
	NodeIDs        []ids.NodeID
}

func (v *Validator) Less(o *Validator) bool {
	return bytes.Compare(v.PublicKeyBytes, o.PublicKeyBytes) < 0
}
