package core

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/params"
)

var testLeaderConfig TxPoolConfig

func init() {
	testLeaderConfig = DefaultTxPoolConfig
	testLeaderConfig.Journal = ""
}

type testLeaderBlockChain struct {
	statedb       *state.StateDB
	gasLimit      uint64
	chainHeadFeed *event.Feed
}

func (bc *testLeaderBlockChain) CurrentBlock() *types.Block {
	return types.NewBlock(&types.Header{
		GasLimit: bc.gasLimit,
	}, nil, nil, nil)
}

func (bc *testLeaderBlockChain) GetBlock(hash common.Hash, number uint64) *types.Block {
	return bc.CurrentBlock()
}

func (bc *testLeaderBlockChain) StateAt(common.Hash) (*state.StateDB, error) {
	return bc.statedb, nil
}

func (bc *testLeaderBlockChain) SubscribeChainHeadEvent(ch chan<- ChainHeadEvent) event.Subscription {
	return bc.chainHeadFeed.Subscribe(ch)
}

func setupLeaderTxPool() (*TxPool, *ecdsa.PrivateKey) {
	fmt.Println("Enter setupTxPool()!")
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	blockChain := &testLeaderBlockChain{statedb, 100000, new(event.Feed)}

	key, _ := crypto.GenerateKey()
	pool := NewTxPool(testLeaderConfig, params.TestChainConfig, blockChain)

	return pool, key
}

func leaderCreateTransaction(nounce uint64, gaslimit uint64, key *ecdsa.PrivateKey) *types.Transaction {
	return leaderPricedTransaction(nounce, gaslimit, big.NewInt(100), key)
}

func leaderPricedTransaction(nounce uint64, gaslimit uint64, gasprice *big.Int, key *ecdsa.PrivateKey) *types.Transaction {
	tx, _ := types.SignTx(types.NewTransaction(nounce, common.Address{}, big.NewInt(100), gaslimit, gasprice, nil), types.HomesteadSigner{}, key)
	return tx
}

func leaderDeriveSender(tx *types.Transaction) (common.Address, error) {
	return types.Sender(types.HomesteadSigner{}, tx)
}

func TestLeader(t *testing.T) {
	fmt.Println("start Test Leader !!!")
	//创建交易池
	pool, key := setupLeaderTxPool()
	defer pool.Stop()

	//创建交易
	tx1 := leaderCreateTransaction(1, 100000, key)
	tx2 := leaderCreateTransaction(10, 100000, key)
	tx3 := leaderCreateTransaction(11, 100000, key)
	from, _ := leaderDeriveSender(tx1)
	pool.currentState.AddBalance(from, big.NewInt(1000*1000000))
	pool.lockedReset(nil, nil)
	pool.AddLocal(tx1)
	pool.AddLocal(tx2)
	pool.AddLocal(tx3)
	//pool.enqueueTx(tx1.Hash(), tx1)
	//pool.enqueueTx(tx2.Hash(), tx2)
	//pool.enqueueTx(tx3.Hash(), tx3)

	go followerProcess()
	go leaderProcess(pool)
}
