package blockgenor

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/signhelper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/hd"
	"github.com/ethereum/go-ethereum/reelection"
	"github.com/ethereum/go-ethereum/topnode"
)

var (
	TimeStampError          = errors.New("Timestamp Error")
	NodeIDError             = errors.New("Node Error")
	PosHeaderError          = errors.New("PosHeader Error")
	MinerResultError        = errors.New("MinerResult Error")
	MinerPosfail            = errors.New("MinerResult POS Fail")
	AccountError            = errors.New("Acccount Error")
	TxsError                = errors.New("txs Error")
	NoWallets               = errors.New("No Wallets ")
	NoAccount               = errors.New("No Account   ")
	ParaNull                = errors.New("Para is null  ")
	Noleader                = errors.New("not leader  ")
	SignaturesError         = errors.New("Signatures Error")
	FakeHeaderError         = errors.New("FakeHeader Error")
	VoteResultError         = errors.New("VoteResultError Error")
	HeightError             = errors.New("Height Error")
	HaveNotOKResultError    = errors.New("have no satisfy miner result")
	HaveNoGenBlockError     = errors.New("have no gen block data")
	HashNoSignNotMatchError = errors.New("hash without sign not match")
	maxUint256              = new(big.Int).Exp(big.NewInt(2), big.NewInt(256), big.NewInt(0))
)

const (
	MinerResultTimeout  = 20
	maxTimeFutureBlocks = 20
)

func GetNetTopology(height uint64) (common.NetTopology, []common.Elect) {
	return common.NetTopology{common.NetTopoTypeChange, nil}, make([]common.Elect, 0)
}

type Backend interface {
	AccountManager() *accounts.Manager
	BlockChain() *core.BlockChain
	TxPool() *core.TxPool
	ChainDb() ethdb.Database
	EventMux() *event.TypeMux
	SignHelper() *signhelper.SignHelper
	HD() *hd.HD
	ReElection() *reelection.ReElection
	FetcherNotify(hash common.Hash, number uint64)
	TopNode() *topnode.TopNodeService
}
