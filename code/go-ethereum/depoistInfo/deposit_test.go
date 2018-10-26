package depoistInfo

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
	"math/big"
	"testing"
)

func TestBigInt(t *testing.T) {
	var tm *big.Int
	var h rpc.BlockNumber
	tm = big.NewInt(100)
	encode := hexutil.EncodeBig(tm)
	err := h.UnmarshalJSON([]byte(encode))
	fmt.Println("err", err)
	fmt.Printf("encode:%T   %v\n", encode, encode)
}
