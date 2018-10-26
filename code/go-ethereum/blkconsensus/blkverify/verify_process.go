package blkverify

import (
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/signhelper"
	"github.com/ethereum/go-ethereum/blkconsensus/votepool"
	"github.com/ethereum/go-ethereum/ca"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/matrixwork"
	"github.com/ethereum/go-ethereum/mc"
	"github.com/ethereum/go-ethereum/reelection"
	"github.com/ethereum/go-ethereum/topnode"
)

type State uint16

const (
	StateIdle State = iota
	StateStart
	StateReqVerify
	StateTxsVerify
	StateDPOSVerify
	StateEnd
)

func (s State) String() string {
	switch s {
	case StateIdle:
		return "未运行状态"
	case StateStart:
		return "开始状态"
	case StateReqVerify:
		return "请求验证阶段"
	case StateTxsVerify:
		return "交易验证阶段"
	case StateDPOSVerify:
		return "DPOS共识阶段"
	case StateEnd:
		return "完成状态"
	default:
		return "未知状态"
	}
}

const (
	localVerifyResultProcessing uint8 = iota
	localVerifyResultSuccess
	localVerifyResultFailedButCanRecover
	localVerifyResultStateFailed
)

type Process struct {
	mu            sync.Mutex
	leader        common.Address
	number        uint64
	role          common.RoleType
	state         State
	curProcessReq *reqData
	reqCache      *reqCache
	pm            *ProcessManage
	txsAcquireSeq int
}

func newProcess(number uint64, pm *ProcessManage) *Process {
	p := &Process{
		leader:        common.Address{},
		number:        number,
		role:          common.RoleNil,
		state:         StateIdle,
		curProcessReq: nil,
		reqCache:      newReqCache(),
		pm:            pm,
		txsAcquireSeq: 0,
	}

	return p
}

func (p *Process) StartRunning(role common.RoleType) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.role = role
	p.changeState(StateStart)

	if p.role == common.RoleBroadcast {
		p.startReqVerifyBC()
	} else if p.role == common.RoleValidator {
		p.startReqVerifyCommon()
	}
}

func (p *Process) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.state = StateIdle
	p.curProcessReq = nil
}

func (p *Process) ReInit() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.checkState(StateIdle) {
		return
	}

	if p.role == common.RoleValidator {
		p.state = StateStart
		p.leader = common.Address{}
		p.curProcessReq = nil
	}
}

func (p *Process) SetLeader(leader common.Address) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.leader == leader {
		return
	}
	p.leader.Set(leader)

	if p.checkState(StateIdle) {
		return
	}

	if p.role == common.RoleValidator {
		p.state = StateStart
		p.curProcessReq = nil
		p.startReqVerifyCommon()
	} else if p.role == common.RoleBroadcast {
		log.WARN(p.logExtraInfo(), "广播身份下收到leader变更消息", "不处理")
	}
}

func (p *Process) AddReq(reqMsg *mc.HD_BlkConsensusReqMsg) {
	p.mu.Lock()
	defer p.mu.Unlock()

	err := p.reqCache.AddReq(reqMsg)
	if err != nil {
		log.ERROR(p.logExtraInfo(), "请求添加缓存失败", err, "from", reqMsg.From, "高度", p.number)
		return
	}
	log.INFO(p.logExtraInfo(), "请求添加缓存成功", err, "from", reqMsg.From, "高度", p.number)

	if p.role == common.RoleBroadcast {
		p.startReqVerifyBC()
	} else if p.role == common.RoleValidator {
		if p.leader == reqMsg.Header.Leader {
			p.startReqVerifyCommon()
		}
	}
}

func (p *Process) AddLocalReq(localReq *mc.LocalBlockVerifyConsensusReq) {
	p.mu.Lock()
	defer p.mu.Unlock()

	leader := localReq.BlkVerifyConsensusReq.Header.Leader
	err := p.reqCache.AddLocalReq(localReq)
	if err != nil {
		log.ERROR(p.logExtraInfo(), "本地请求添加缓存失败", err, "高度", p.number, "leader", leader.Hex())
		return
	}
	log.INFO(p.logExtraInfo(), "本地请求添加成功, 高度", p.number, "leader", leader.Hex())

	if p.role == common.RoleBroadcast {
		p.startReqVerifyBC()
	} else if p.role == common.RoleValidator {
		if p.leader == leader {
			p.startReqVerifyCommon()
		}
	}
}

func (p *Process) ProcessDPOSOnce() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.processDPOSOnce()
}

func (p *Process) startReqVerifyCommon() {
	if p.checkState(StateStart) == false && p.checkState(StateEnd) == false {
		log.WARN(p.logExtraInfo(), "准备开始请求验证阶段，状态错误", p.state.String(), "高度", p.number)
		return
	}

	if (p.leader == common.Address{}) {
		log.WARN(p.logExtraInfo(), "请求验证阶段", "当前leader为空，等待leader消息", "高度", p.number)
		return
	}

	req, err := p.reqCache.GetLeaderReq(p.leader)
	if err != nil {
		log.WARN(p.logExtraInfo(), "请求验证阶段,寻找leader的请求错误,继续等待请求", err, "Leader", p.leader.Hex(), "高度", p.number)
		return
	}

	p.curProcessReq = req
	p.curProcessReq.hash = p.curProcessReq.req.Header.HashNoSignsAndNonce()
	log.INFO(p.logExtraInfo(), "请求验证阶段", "开始", "高度", p.number, "HeaderHash", p.curProcessReq.hash.TerminalString(), "parent hash", p.curProcessReq.req.Header.ParentHash.TerminalString(), "之前状态", p.state.String())
	p.state = StateReqVerify
	p.processReqOnce()
}

func (p *Process) processReqOnce() {
	if p.checkState(StateReqVerify) == false {
		return
	}

	// notify leader server, begin verify now
	notify := mc.BlockVerifyStateNotify{Leader: p.leader, Number: p.number, State: true}
	mc.PublishEvent(mc.BlkVerify_VerifyStateNotify, &notify)

	// if is local req, skip local verify step
	if p.curProcessReq.localReq {
		log.INFO(p.logExtraInfo(), "请求为本地请求", "跳过验证阶段", "高度", p.number)
		p.startDPOSVerify(localVerifyResultSuccess)
		return
	}

	// verify header
	if err := p.blockChain().VerifyHeader(p.curProcessReq.req.Header); err != nil {
		log.ERROR(p.logExtraInfo(), "预验证头信息失败", err, "高度", p.number)
		p.startDPOSVerify(localVerifyResultStateFailed)
		return
	}

	// verify election info
	if err := p.verifyElection(p.curProcessReq.req.Header); err != nil {
		log.ERROR(p.logExtraInfo(), "验证选举信息失败", err, "高度", p.number)
		p.startDPOSVerify(localVerifyResultStateFailed)
		return
	}

	// verify net topology info
	if err := p.verifyNetTopology(p.curProcessReq.req.Header); err != nil {
		log.ERROR(p.logExtraInfo(), "验证拓扑信息失败", err, "高度", p.number)
		p.startDPOSVerify(localVerifyResultFailedButCanRecover)
		return
	}

	//todo Version

	p.startTxsVerify()
}

func (p *Process) startTxsVerify() {
	if p.checkState(StateReqVerify) == false {
		return
	}
	log.INFO(p.logExtraInfo(), "交易获取", "开始", "当前身份", p.role.String(), "高度", p.number)

	p.changeState(StateTxsVerify)

	p.txsAcquireSeq++
	leader := p.curProcessReq.req.Header.Leader
	log.INFO(p.logExtraInfo(), "开始交易获取,seq", p.txsAcquireSeq, "数量", len(p.curProcessReq.req.TxsCode), "leader", leader.Hex(), "高度", p.number)
	txAcquireCh := make(chan *core.RetChan, 1)
	go p.txPool().ReturnAllTxsByN(p.curProcessReq.req.TxsCode, p.txsAcquireSeq, leader, txAcquireCh)
	go p.processTxsAcquire(txAcquireCh, p.txsAcquireSeq)
}

func (p *Process) processTxsAcquire(txsAcquireCh <-chan *core.RetChan, seq int) {
	log.INFO(p.logExtraInfo(), "交易获取协程", "启动", "当前身份", p.role.String(), "高度", p.number)
	defer log.INFO(p.logExtraInfo(), "交易获取协程", "退出", "当前身份", p.role.String(), "高度", p.number)

	outTime := time.NewTimer(time.Second * 5)
	select {
	case txsResult := <-txsAcquireCh:

		go p.VerifyTxs(txsResult)
	case <-outTime.C:
		log.INFO(p.logExtraInfo(), "交易获取协程", "获取交易超时", "高度", p.number, "seq", seq)
		go p.ProcessTxsAcquireTimeOut(seq)
		return
	}
}

func (p *Process) ProcessTxsAcquireTimeOut(seq int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	log.INFO(p.logExtraInfo(), "交易获取超时处理", "开始", "高度", p.number, "seq", seq, "cur seq", p.txsAcquireSeq)
	defer log.INFO(p.logExtraInfo(), "交易获取超时处理", "结束", "高度", p.number, "seq", seq)

	if seq != p.txsAcquireSeq {
		log.WARN(p.logExtraInfo(), "交易获取超时处理", "Seq不匹配，忽略", "高度", p.number, "seq", seq, "cur seq", p.txsAcquireSeq)
		return
	}

	if p.checkState(StateTxsVerify) == false {
		log.INFO(p.logExtraInfo(), "交易获取超时处理", "状态不正确，不处理", "高度", p.number, "seq", seq)
		return
	}

	p.startDPOSVerify(localVerifyResultFailedButCanRecover)
}

func (p *Process) VerifyTxs(result *core.RetChan) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.checkState(StateTxsVerify) == false {
		return
	}

	log.INFO(p.logExtraInfo(), "交易验证，交易数据 result.seq", result.Resqe, "当前 reqSeq", p.txsAcquireSeq, "高度", p.number)
	if result.Resqe != p.txsAcquireSeq {
		log.WARN(p.logExtraInfo(), "交易验证", "seq不匹配，跳过", "高度", p.number)
		return
	}

	if result.Err != nil {
		log.ERROR(p.logExtraInfo(), "交易验证，交易数据错误", result.Err, "高度", p.number)
		p.startDPOSVerify(localVerifyResultFailedButCanRecover)
		return
	}

	log.INFO(p.logExtraInfo(), "开始交易验证, 数量", len(result.Rxs), "高度", p.number)
	p.curProcessReq.txs = result.Rxs

	//todo 跑交易交易验证， Root TxHash ReceiptHash Bloom GasLimit GasUsed
	remoteHeader := p.curProcessReq.req.Header
	localHeader := types.CopyHeader(remoteHeader)
	localHeader.GasUsed = 0

	work, err := matrixwork.NewWork(p.blockChain().Config(), p.blockChain(), nil, localHeader)
	if err != nil {
		log.ERROR(p.logExtraInfo(), "交易验证，创建work失败!", err, "高度", p.number)
		p.startDPOSVerify(localVerifyResultFailedButCanRecover)
		return
	}
	//todo add handleuptime
	/*	if common.IsBroadcastNumber(p.number-1) && p.number > common.GetBroadcastInterval() {
		upTimeAccounts, err := work.GetUpTimeAccounts(p.number)
		if err != nil {
			log.ERROR(p.logExtraInfo(), "获取所有抵押账户错误!", err, "高度", p.number)
			return
		}
		calltherollMap, heatBeatUnmarshallMMap, err := work.GetUpTimeData(p.number)
		if err != nil {
			log.WARN(p.logExtraInfo(), "获取心跳交易错误!", err, "高度", p.number)
		}
		err = work.HandleUpTime(work.State, upTimeAccounts, calltherollMap, heatBeatUnmarshallMMap, p.number, p.pm.bc)
		if nil != err {
			log.ERROR(p.logExtraInfo(), "处理uptime错误", err)
			return
		}
	}*/
	err = work.ConsensusTransactions(p.pm.event, p.curProcessReq.txs, p.pm.bc)
	if err != nil {
		log.ERROR(p.logExtraInfo(), "交易验证，共识执行交易出错!", err, "高度", p.number)
		p.startDPOSVerify(localVerifyResultStateFailed)
		return
	}
	_, err = p.blockChain().Engine().Finalize(p.blockChain(), localHeader, work.State,
		p.curProcessReq.txs, nil, work.Receipts)
	if err != nil {
		log.ERROR(p.logExtraInfo(), "交易验证,错误", "Failed to finalize block for sealing", "err", err)
		p.startDPOSVerify(localVerifyResultStateFailed)
		return
	}
	//localBlock check
	localHash := localHeader.HashNoSignsAndNonce()

	if localHash != p.curProcessReq.hash {
		log.ERROR(p.logExtraInfo(), "交易验证，错误", "block hash不匹配",
			"local hash", localHash.TerminalString(), "remote hash", p.curProcessReq.hash.TerminalString(),
			"local root", localHeader.Root.TerminalString(), "remote root", remoteHeader.Root.TerminalString(),
			"local txHash", localHeader.TxHash.TerminalString(), "remote txHash", remoteHeader.TxHash.TerminalString(),
			"local ReceiptHash", localHeader.ReceiptHash.TerminalString(), "remote ReceiptHash", remoteHeader.ReceiptHash.TerminalString(),
			"local Bloom", localHeader.Bloom.Big(), "remote Bloom", remoteHeader.Bloom.Big(),
			"local GasLimit", localHeader.GasLimit, "remote GasLimit", remoteHeader.GasLimit,
			"local GasUsed", localHeader.GasUsed, "remote GasUsed", remoteHeader.GasUsed)
		p.startDPOSVerify(localVerifyResultStateFailed)
		return
	}

	p.curProcessReq.receipts = work.Receipts
	p.curProcessReq.stateDB = work.State

	// 开始DPOS共识验证
	p.startDPOSVerify(localVerifyResultSuccess)
}

func (p *Process) sendVote(validate bool) {
	signHash := p.curProcessReq.hash
	sign, err := p.signHelper().SignHashWithValidate(signHash.Bytes(), validate)
	if err != nil {
		log.ERROR(p.logExtraInfo(), "投票签名失败", err, "高度", p.number)
		return
	}

	voteMsg := mc.HD_ConsensusVote{SignHash: signHash, Sign: sign, Round: p.number}
	log.INFO(p.logExtraInfo(), "发出成功投票 signHash", voteMsg.SignHash.TerminalString(), "高度", p.number)
	p.pm.hd.SendNodeMsg(mc.HD_BlkConsensusVote, &voteMsg, common.RoleValidator, nil)

	//将自己的投票加入票池
	if err := p.votePool().AddVote(signHash, sign, ca.GetAddress(), p.number); err != nil {
		log.ERROR(p.logExtraInfo(), "自己的投票加入票池失败", err, "高度", p.number)
	}

	// notify block genor server the result
	result := mc.BlockVerifyConsensusOK{
		Header:    p.curProcessReq.req.Header,
		BlockHash: p.curProcessReq.hash,
		Txs:       p.curProcessReq.txs,
		Receipts:  p.curProcessReq.receipts,
		State:     p.curProcessReq.stateDB,
	}
	//log.INFO(p.logExtraInfo(), "发出区块共识结果消息", result, "高度", p.number)
	mc.PublishEvent(mc.BlkVerify_VerifyConsensusOK, &result)
}

func (p *Process) startDPOSVerify(lvResult uint8) {
	if p.state >= StateDPOSVerify {
		return
	}

	if p.role == common.RoleBroadcast {
		//广播节点，跳过DPOS投票验证阶段
		p.bcFinishedProcess(lvResult)
		return
	}

	log.INFO(p.logExtraInfo(), "开始DPOS阶段,验证结果", lvResult, "高度", p.number)

	if lvResult == localVerifyResultSuccess {
		p.sendVote(true)
	}
	p.curProcessReq.localVerifyResult = lvResult

	p.state = StateDPOSVerify
	p.processDPOSOnce()
}

func (p *Process) processDPOSOnce() {
	if p.checkState(StateDPOSVerify) == false {
		return
	}

	if p.curProcessReq.req == nil {
		return
	}

	signs := p.votePool().GetVotes(p.curProcessReq.hash)
	log.INFO(p.logExtraInfo(), "执行DPOS, 投票数量", len(signs), "hash", p.curProcessReq.hash.TerminalString(), "高度", p.number)
	rightSigns, err := p.blockChain().DPOSEngine().VerifyHashWithVerifiedSignsAndNumber(signs, p.number)
	if err != nil {
		log.ERROR(p.logExtraInfo(), "共识引擎验证失败", err, "高度", p.number)
		return
	}
	log.INFO(p.logExtraInfo(), "DPOS通过，正确签名数量", len(rightSigns), "高度", p.number)
	p.curProcessReq.req.Header.Signatures = rightSigns

	p.finishedProcess()
}

func (p *Process) finishedProcess() {
	result := p.curProcessReq.localVerifyResult
	if result == localVerifyResultProcessing {
		log.ERROR(p.logExtraInfo(), "req is processing now, can't finish!", "validator", "高度", p.number)
		return
	}
	if result == localVerifyResultStateFailed {
		log.Error(p.logExtraInfo(), "local verify header err, but dpos pass! please check your state!", "validator", "高度", p.number)
		//todo 硬分叉了，以后加需要处理
		return
	}

	if result == localVerifyResultSuccess {
		// notify leader server the verify state
		notify := mc.BlockVerifyStateNotify{Leader: p.leader, Number: p.number, State: false}
		mc.PublishEvent(mc.BlkVerify_VerifyStateNotify, &notify)
	}

	hash := p.curProcessReq.req.Header.HashNoSignsAndNonce()
	//给矿工发送区块验证结果
	log.INFO(p.logExtraInfo(), "发出挖矿请求, Header hash with signs", hash, "高度", p.number)
	p.pm.hd.SendNodeMsg(mc.HD_MiningReq, &mc.HD_MiningReqMsg{Header: p.curProcessReq.req.Header}, common.RoleMiner, nil)

	//给广播节点发送区块验证请求(带签名列表)
	log.INFO(p.logExtraInfo(), "向广播节点发送 leader", p.curProcessReq.req.Header.Leader.Hex(), "高度", p.number)
	p.pm.hd.SendNodeMsg(mc.HD_BlkConsensusReq, p.curProcessReq.req, common.RoleBroadcast, nil)

	p.votePool().DelVotes(p.curProcessReq.hash)
	p.state = StateEnd
}

func (p *Process) checkState(state State) bool {
	return p.state == state
}

func (p *Process) changeState(targetState State) {
	if p.state == targetState-1 {
		log.WARN(p.logExtraInfo(), "切换状态成功, 原状态", p.state.String(), "新状态", targetState.String(), "高度", p.number)
		p.state = targetState
	} else {
		log.WARN(p.logExtraInfo(), "切换状态失败, 原状态", p.state.String(), "目标状态", targetState.String(), "高度", p.number)
	}
}

func (p *Process) votePool() *votepool.VotePool { return p.pm.votePool }

func (p *Process) signHelper() *signhelper.SignHelper { return p.pm.signHelper }

func (p *Process) blockChain() *core.BlockChain { return p.pm.bc }

func (p *Process) txPool() *core.TxPool { return p.pm.txPool }

func (p *Process) reElection() *reelection.ReElection { return p.pm.reElection }

func (p *Process) logExtraInfo() string { return p.pm.logExtraInfo() }

func (p *Process) eventMux() *event.TypeMux { return p.pm.event }

func (p *Process) topNode() *topnode.TopNodeService { return p.pm.topNode }
