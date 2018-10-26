package topnode

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/mc"

	"sync"
	"github.com/ethereum/go-ethereum/log"
)

type messageCheck struct {
	mu       sync.RWMutex
	leader   common.Address
	height	uint64
	curRound uint64
}

func (chk *messageCheck) checkLeaderChangeNotify(msg *mc.LeaderChangeNotify) bool {
	if msg.ConsensusState {
		round := msg.Number*100 + uint64(msg.ReelectTurn)
		if chk.setRound(round) {
			chk.setLeader(msg.Leader)
			chk.setBlockHeight(msg.Number)
			log.Info("topnodeOnline", "设置leader",msg.Leader, "设置Number", msg.Number)
			return true
		}
	}
	return false
}
func (chk *messageCheck) checkOnlineConsensusReq(msg *mc.OnlineConsensusReq) bool {
	return chk.setRound(msg.Seq)
}
func (chk *messageCheck) checkOnlineConsensusVote(msg *mc.HD_ConsensusVote) bool {
	return chk.setRound(msg.Round)
}

func (chk *messageCheck)setBlockHeight(height uint64)  {
	chk.mu.Lock()
	defer chk.mu.Unlock()
	chk.height = height
}

func (chk *messageCheck)getBlockHeight() uint64 {
	chk.mu.Lock()
	defer chk.mu.Unlock()
	return chk.height
}

func (chk *messageCheck) setLeader(leader common.Address) {
	chk.mu.Lock()
	defer chk.mu.Unlock()
	chk.leader = leader
}
func (chk *messageCheck) getLeader() common.Address {
	chk.mu.RLock()
	defer chk.mu.RUnlock()
	return chk.leader
}
func (chk *messageCheck) getRound() uint64 {
	chk.mu.RLock()
	defer chk.mu.RUnlock()
	return chk.curRound
}
func (chk *messageCheck) checkRound(round uint64) bool {
	chk.mu.RLock()
	defer chk.mu.RUnlock()
	return round >= chk.curRound
}
func (chk *messageCheck) setRound(round uint64) bool {
	chk.mu.Lock()
	defer chk.mu.Unlock()
	if round < chk.curRound {
		return false
	}
	if round > chk.curRound {
		chk.curRound = round
	}
	return true
}
