package hd

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// AlgorithmMsg
type AlgorithmMsg struct {
	Account common.Address
	Data    NetData
}

//NetData
type NetData struct {
	SubCode uint32
	Msg     []byte
}

type BroadcastRspMsgForMarshal struct {
	Header *types.Header
	Txs    []*types.Transaction_Mx
}
