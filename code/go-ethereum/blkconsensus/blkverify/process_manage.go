package blkverify

import (
	"sync"

	"github.com/ethereum/go-ethereum/accounts/signhelper"
	"github.com/ethereum/go-ethereum/blkconsensus/votepool"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/hd"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/reelection"
	"github.com/ethereum/go-ethereum/topnode"
	"github.com/pkg/errors"
)

type ProcessManage struct {
	mu         sync.Mutex
	curNumber  uint64
	processMap map[uint64]*Process
	votePool   *votepool.VotePool
	hd         *hd.HD
	signHelper *signhelper.SignHelper
	bc         *core.BlockChain
	txPool     *core.TxPool
	reElection *reelection.ReElection
	topNode    *topnode.TopNodeService
	event      *event.TypeMux
}

func NewProcessManage(matrix Matrix) *ProcessManage {
	return &ProcessManage{
		curNumber:  0,
		processMap: make(map[uint64]*Process),
		votePool:   votepool.NewVotePool(common.RoleValidator, "区块验证服务票池"),
		hd:         matrix.HD(),
		signHelper: matrix.SignHelper(),
		bc:         matrix.BlockChain(),
		txPool:     matrix.TxPool(),
		reElection: matrix.ReElection(),
		topNode:    matrix.TopNode(),
		event:      matrix.EventMux(),
	}
}

func (pm *ProcessManage) SetCurNumber(number uint64) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.curNumber = number
	pm.fixProcessMap()
}

func (pm *ProcessManage) GetCurrentProcess() *Process {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	return pm.getProcess(pm.curNumber)
}

func (pm *ProcessManage) GetProcess(number uint64) (*Process, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if err := pm.isLegalNumber(number); err != nil {
		return nil, err
	}
	return pm.getProcess(number), nil
}

/*func (pm *ProcessManage) GetProcessByRole(number uint64, role common.RoleType) (*Process, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if err := pm.isLegalNumber(number); err != nil {
		return nil, err
	}
	process := pm.getProcess(number)
	pRole := process.GetRole()
	if pRole != role {
		return nil, errors.Errorf("process.role(%s) != params.role(%s)", pRole.String(), role.String())
	}
	return process, nil
}*/

func (pm *ProcessManage) fixProcessMap() {
	if len(pm.processMap) == 0 {
		return
	}

	log.INFO(pm.logExtraInfo(), "PM 开始修正map, process数量", len(pm.processMap), "修复高度", pm.curNumber)

	delKeys := make([]uint64, 0)
	for key, process := range pm.processMap {
		if key < pm.curNumber {
			process.Close()
			delKeys = append(delKeys, key)
		}
	}

	for _, delKey := range delKeys {
		delete(pm.processMap, delKey)
	}

	log.INFO(pm.logExtraInfo(), "PM 结束修正map, process数量", len(pm.processMap))
}

func (pm *ProcessManage) AddVoteToPool(signHash common.Hash, sign common.Signature, fromAccount common.Address, height uint64) error {
	return pm.votePool.AddVote(signHash, sign, fromAccount, height)
}

func (pm *ProcessManage) isLegalNumber(number uint64) error {
	if number < pm.curNumber {
		return errors.Errorf("number(%d) is less than current number(%d)", number, pm.curNumber)
	}

	if number > pm.curNumber+2 {
		return errors.Errorf("number(%d) is too big than current number(%d)", number, pm.curNumber)
	}

	return nil
}

func (pm *ProcessManage) getProcess(number uint64) *Process {
	process, OK := pm.processMap[number]
	if OK == false {
		log.INFO(pm.logExtraInfo(), "PM 创建process，高度", number)
		process = newProcess(number, pm)
		pm.processMap[number] = process
	}
	return process
}

func (pm *ProcessManage) logExtraInfo() string {
	return "区块验证服务"
}
