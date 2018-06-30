package core

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/sha3"
	"github.com/ethereum/go-ethereum/rlp"
)

type packageTx struct {
	txs []*types.Transaction
	mu  sync.RWMutex
}

const (
	followerNum int = 10
)

var invalidTxSet [followerNum][]bool

type VoteResultSigner interface {
	/* TODO: add methods */
	Sender(vr *voteResult) (common.Address, error)
	SignatureValues(vr *voteResult, sig []byte) (r, s, v *big.Int, err error)
	Hash(vr *voteResult) common.Hash
	Equal(VoteResultSigner) bool
}

type voteResult struct {
	invalidTx []bool

	fee int //押金

	//Signature values
	V *big.Int
	R *big.Int
	S *big.Int
}

var voteResultSet [followerNum]voteResult

func rlpHash(x interface{}) (h common.Hash) {
	hw := sha3.NewKeccak256()
	rlp.Encode(hw, x)
	hw.Sum(h[:0])
	return h
}

func recoverPlain(sighash common.Hash, R, S, Vb *big.Int, homestead bool) (common.Address, error) {
	if Vb.BitLen() > 8 {
		return common.Address{}, types.ErrInvalidSig
	}
	V := byte(Vb.Uint64() - 27)
	if !crypto.ValidateSignatureValues(V, R, S, homestead) {
		return common.Address{}, types.ErrInvalidSig
	}
	// encode the snature in uncompressed format
	r, s := R.Bytes(), S.Bytes()
	sig := make([]byte, 65)
	copy(sig[32-len(r):32], r)
	copy(sig[64-len(s):64], s)
	sig[64] = V
	// recover the public key from the snature
	pub, err := crypto.Ecrecover(sighash[:], sig)
	if err != nil {
		return common.Address{}, err
	}
	if len(pub) == 0 || pub[0] != 4 {
		return common.Address{}, errors.New("invalid public key")
	}
	var addr common.Address
	copy(addr[:], crypto.Keccak256(pub[1:])[12:])
	return addr, nil
}

type LeaderVoteResultSigner struct{}

func (l LeaderVoteResultSigner) Equal(s2 VoteResultSigner) bool {
	_, ok := s2.(LeaderVoteResultSigner)
	return ok
}

func (l LeaderVoteResultSigner) Hash(vr *voteResult) common.Hash {
	return rlpHash([]interface{}{
		vr.invalidTx,
	})
}

func (l LeaderVoteResultSigner) Sender(vr *voteResult) (common.Address, error) {
	return recoverPlain(l.Hash(vr), vr.R, vr.S, vr.V, false)

}

func (l LeaderVoteResultSigner) SignatureValues(vr *voteResult, sig []byte) (r, s, v *big.Int, err error) {
	if len(sig) != 65 {
		panic(fmt.Sprintf("Wrong size of signature: got %d, want 65", len(sig)))
	}
	r = new(big.Int).SetBytes(sig[:32])
	s = new(big.Int).SetBytes(sig[32:64])
	v = new(big.Int).SetBytes([]byte{sig[64] + 27})
	return r, s, v, nil
}

func (vr *voteResult) WithSignature(signer LeaderVoteResultSigner, sig []byte) (*voteResult, error) {
	r, s, v, err := signer.SignatureValues(vr, sig)
	if err != nil {
		return nil, err
	}
	vr.R, vr.S, vr.V = r, s, v
	return vr, nil
}

func SignVoteResult(v *voteResult, s LeaderVoteResultSigner, prv *ecdsa.PrivateKey) (*voteResult, error) {
	h := s.Hash(v)
	sig, err := crypto.Sign(h[:], prv)
	if err != nil {
		return nil, err
	}
	return v.WithSignature(s, sig)
}

func packageTxInPool(pool *TxPool) *packageTx {

	txPackage := &packageTx{
		txs: make([]*types.Transaction, 0),
	}

	localTxs := pool.local()

	for _, v := range localTxs {
		for _, tx := range v {
			txPackage.txs = append(txPackage.txs, tx)
		}
	}
	fmt.Printf("txs's len is %d\n", len(txPackage.txs))
	return txPackage
}

func deleteInvalidTx(invalidTx []bool, txs []*types.Transaction, validTx *[]types.Transaction) {
	//无效交易列表中，true表示当前交易为无效交易
	for k, v := range invalidTx {
		if v != true {
			*validTx = append(*validTx, *txs[k])
		}
	}
	return
}

var totalFee int = 1000

func DposTx(result [followerNum]voteResult) int {
	fmt.Println("enter dposTx!")
	var fee, count int
	for _, k := range result {
		if k.R != nil && k.V != nil && k.S != nil && k.fee > 0 {
			fee = fee + k.fee
			count++
		}

	}
	fmt.Println("count=", count, "fee", fee)
	if (count >= 6) && (fee*10/totalFee > 7) {
		return 0
	}
	return 1
}

type ConsesusResult struct {
	txs []types.Transaction
	fee int
}

const (
	msgStart int = iota
	msgRequestTx
	msgRequestVote
)

var msgChan chan int

//var txChan chan types.Transaction

//var invalidTxChan chan invalidTx

var voteResultChan chan voteResult

func sendTxRequestToFollowers() {
	msgChan = make(chan int, 0)
	msgChan <- msgRequestTx
	return
}

func sendResultToMiner(txs []types.Transaction) {
	var msg ConsesusResult
	msg.txs = txs
	msg.fee = 1000
	//sendToMiner()
	return
}

func sendVoteRequestToFollower() {
	msgChan = make(chan int, 0)
	msgChan <- msgRequestVote
	return
}

func followerProcess() {
	fmt.Println("Enter to followerProcess!")
	for {
		select {
		case msg := <-msgChan:
			if msg == msgRequestTx {
				fmt.Println("Recv msgRequestTx!")
			} else if msg == msgRequestVote {
				fmt.Println("Recv msgRequestVote")
			} else {
				fmt.Println("Recv wrong msg ", msg)
			}

		}
	}
}

func leaderProcess(local *TxPool) {
	fmt.Println("Enter leaderProcess()!")
	//var followers = make([]follower, 10)
	//向follower发送 请求发送交易的消息
	//for _, follower := range followers {
	go sendTxRequestToFollowers()
	//}

	//将交易加入到交易池中
	//if err := local.AddLocal(tx); err != nil {
	//fmt.Println("Add tx failed! ", err)
	//} else {
	//fmt.Println("Add tx to local txpool, success!")
	//}

	//将交易池中的交易打包为交易块
	txs := packageTxInPool(local)
	fmt.Printf("len(txs.txs)=%d\n", len(txs.txs))

	//向 follower发送打包好的交易块
	//for _, follower := range followers {
	//go sendTxToFollower(&follower)
	//}

	//验证交易
	invalidTx := make([]bool, len(txs.txs))
	for k, tx := range txs.txs {
		// If the transaction fails basic validation, discard it
		if err := local.validateTx(tx, false); err != nil {
			invalidTx[k] = true
		}
	}

	fmt.Println("invalidTx: ", invalidTx)

	key, err := crypto.GenerateKey()
	if err != nil {
		fmt.Printf("get PrivateKey failed! err=%s\n", err)
	}

	vr := voteResult{
		invalidTx: invalidTx,
		fee:       80,
	}
	it, _ := SignVoteResult(&vr, LeaderVoteResultSigner{}, key)
	addr1, err := LeaderVoteResultSigner{}.Sender(it)
	addr2 := crypto.PubkeyToAddress(key.PublicKey)
	if addr1 == addr2 {
		fmt.Printf("addr1==addr2!!!\n")
	} else {
		fmt.Printf("addr1=%v, addr2=%v;\n", addr1, addr2)
	}

	//删除无效交易
	validTx := make([]types.Transaction, 0)
	deleteInvalidTx(invalidTx, txs.txs, &validTx)
	if len(validTx) == 0 {
		fmt.Println("No Transaction is valid!!!")
	}
	fmt.Printf("len of validTx is:%v\n", len(validTx))

	//向follower发起投票申请
	//for _, follower := range followers {
	go sendVoteRequestToFollower()
	//}

	for i := 0; i < 10; i++ {
		voteResultSet[i] = vr
	}
	//fmt.Println("voteResultSet= ", voteResultSet)

	//根据投票结果进行共识
	ret := DposTx(voteResultSet)
	if ret != 0 {
		fmt.Println("Dpos varify tx failed!")
	}
	//将共识结果发送给矿工
	sendResultToMiner(validTx)
	return
}
