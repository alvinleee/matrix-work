package election

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/mc"
)

type CandidateInfo struct {
	Address common.Address
	TPS     uint64
	UpTime  uint64
	Deposit uint64
}

type ElectionResultInfo struct {
	Address common.Address
	Stake   uint64
}

type topoGen interface {
	MinerTopoGen()
	// Hashrate returns the current mining hashrate of a PoW consensus engine.
	ValidatorTopoGen()
}
type AllNative struct {

	Master []mc.TopologyNodeInfo //验证者主节点
	BackUp   []mc.TopologyNodeInfo //验证者备份
	Candidate []mc.TopologyNodeInfo //验证者候选

	MasterQ []common.Address//第一梯队候选
	BackUpQ []common.Address//第二梯队候选
	CandidateQ []common.Address//第三梯队候选


}