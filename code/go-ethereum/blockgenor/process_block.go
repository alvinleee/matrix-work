package blockgenor

import (
	"math/big"

	"github.com/ethereum/go-ethereum/ca"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/mc"
	"github.com/pkg/errors"
)

func (p *Process) AddMinerResult(minerResult *mc.HD_MiningRspMsg) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := p.powPool.AddMinerResult(minerResult.Blockhash, minerResult.Difficulty, minerResult); err != nil {
		log.ERROR(p.logExtraInfo(), "矿工挖矿结果入池失败", err, "高度", p.number)
		return
	}
	p.processMinerResultVerify()
}

func (p *Process) AddConsensusBlock(block *mc.BlockVerifyConsensusOK) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.consensusBlock = block
	p.processMinerResultVerify()
}

func (p *Process) processMinerResultVerify() {
	if p.checkState(StateMinerResultVerify) == false {
		log.WARN(p.logExtraInfo(), "准备进行挖矿结果验证，状态错误", p.state.String())
		return
	}

	if common.IsBroadcastNumber(p.number) {
		log.INFO(p.logExtraInfo(), "当前高度为广播区块, 进行广播挖矿结果验证, 高度", p.number)
		p.dealMinerResultVerifyBroadcast()
	} else {
		log.INFO(p.logExtraInfo(), "当前高度为普通区块, 进行普通挖矿结果验证, 高度", p.number)
		p.dealMinerResultVerifyCommon()
	}
}

func (p *Process) dealMinerResultVerifyCommon() {
	if nil == p.consensusBlock {
		log.WARN(p.logExtraInfo(), "准备进行挖矿结果验证，验证区块还未收到！等待验证区块, 高度", p.number)
		return
	}

	if p.role != common.RoleBroadcast && p.consensusBlock.Header.Leader != p.curLeader {
		log.WARN(p.logExtraInfo(), "准备进行挖矿结果验证，共识结果leader不匹配, 高度", p.number,
			"共识结果leader", p.consensusBlock.Header.Leader.Hex(),
			"cur leader", p.curLeader.Hex())
		return
	}
	//todo 可以改进为获取比该难度大的挖矿结果
	diff := big.NewInt(p.consensusBlock.Header.Difficulty.Int64())
	results, err := p.powPool.GetMinerResults(p.consensusBlock.BlockHash, diff)
	if err != nil {
		log.WARN(p.logExtraInfo(), "挖矿结果验证，挖矿结果获取失败", err, "高度", p.number, "难度", diff, "block hash", p.consensusBlock.BlockHash.TerminalString())
		return
	}

	satisfyResult, err := p.pickSatisfyMinerResults(results)
	if err != nil {
		log.WARN(p.logExtraInfo(), "挖矿结果验证，获取合适挖矿结果错误", err, "高度", p.number)
		return
	}

	header := p.copyHeader(satisfyResult)
	p.consensusBlock.Header = header
	p.genBlockData = p.consensusBlock
	p.consensusBlock = nil

	validators, err := p.genValidatorList(p.genBlockData.Header)
	if err != nil {
		log.ERROR(p.logExtraInfo(), "挖矿结果验证，生成验证者列表失败", err, "高度", p.number)
		return
	}

	readyMsg := &mc.NewBlockReady{
		Leader:     p.genBlockData.Header.Leader,
		Number:     p.number,
		Validators: validators,
	}
	log.INFO(p.logExtraInfo(), "普通区块验证完成", "发送新区块准备完毕消息", "高度", p.number, "验证者列表数量", len(readyMsg.Validators.NodeList))
	mc.PublishEvent(mc.BlockGenor_NewBlockReady, readyMsg)

	p.state = StateBlockInsert
	p.processBlockInsert()
}

func (p *Process) processBlockInsert() {
	if p.state < StateBlockInsert {
		log.WARN(p.logExtraInfo(), "准备进行区块插入，状态错误", p.state.String(), "高度", p.number)
		return
	}

	if common.IsBroadcastNumber(p.number + 1) {
		if p.role != common.RoleBroadcast {
			log.WARN(p.logExtraInfo(), "准备进行区块插入，广播区块前一个区块，由广播节点插入", p.role.String(), "高度", p.number)
			return
		}
	} else {
		if p.role != common.RoleValidator {
			log.WARN(p.logExtraInfo(), "准备进行区块插入，身份错误", "当前身份不是验证者", "高度", p.number, "身份", p.role.String())
			return
		}

		if (p.nextLeader == common.Address{}) {
			log.WARN(p.logExtraInfo(), "准备进行区块插入", "下个区块leader为空", "需要等待leader的高度", p.number+1)
			return
		}

		if p.nextLeader != ca.GetAddress() {
			log.INFO(p.logExtraInfo(), "准备进行区块广播,自己不是下个区块leader,高度", p.number, "next leader", p.nextLeader.Hex(), "self", ca.GetAddress())
			return
		}
	}

	log.INFO(p.logExtraInfo(), "~~~~区块插入~~~~", "开始", "高度", p.number)
	hash, err := p.insertAndBcBlock(true, nil)
	if err != nil {
		log.ERROR(p.logExtraInfo(), "区块插入，错误", err)
		return
	}

	log.INFO(p.logExtraInfo(), "~~~~区块插入~~~~", "完成", "高度", p.number, "插入区块hash", hash.TerminalString())
	p.state = StateEnd
}

func (p *Process) pickSatisfyMinerResults(results []*mc.HD_MiningRspMsg) (*mc.HD_MiningRspMsg, error) {
	//todo 应该加入备选矿工滞后选择的流程
	for _, result := range results {
		if err := p.verifyOneResult(result); err != nil {
			log.WARN(p.logExtraInfo(), "验证挖矿结果失败，删除该挖矿结果, from", result.From, "diff", result.Difficulty,
				"高度", p.number, "block hash", result.Blockhash.TerminalString())
			p.powPool.DelOneResult(result.Blockhash, result.Difficulty, result.From)
			continue
		}
		return result, nil
	}
	return nil, HaveNotOKResultError
}

func (p *Process) verifyOneResult(result *mc.HD_MiningRspMsg) error {
	header := p.copyHeader(result)
	headerHash := header.HashNoSignsAndNonce()
	if headerHash != result.Blockhash {
		log.ERROR(p.logExtraInfo(), "挖矿结果不匹配, header hash", headerHash.TerminalString(), "挖矿结果hash", result.Blockhash.TerminalString())
		return MinerResultError
	}

	if err := p.dposEngine().VerifyBlock(header); err != nil {
		log.ERROR(p.logExtraInfo(), "挖矿结果DPOS共识失败", err)
		return err
	}

	//todo 不是原始难度的结果，需要修改POW seal验证过程
	if err := p.engine().VerifySeal(p.blockChain(), header); err != nil {
		log.ERROR(p.logExtraInfo(), "挖矿结果POW验证失败", err)
		return err
	}

	return nil
}

func (p *Process) copyHeader(minerResult *mc.HD_MiningRspMsg) *types.Header {
	header := types.CopyHeader(p.consensusBlock.Header)
	header.Nonce = minerResult.Nonce
	header.Coinbase = minerResult.Coinbase
	header.MixDigest = minerResult.MixDigest
	header.Signatures = make([]common.Signature, 0)
	header.Signatures = append(header.Signatures, minerResult.Signatures...)
	return header
}

func (p *Process) insertAndBcBlock(isSelf bool, header *types.Header) (common.Hash, error) {
	if p.genBlockData == nil {
		return common.Hash{}, HaveNoGenBlockError
	}

	insertHeader := p.genBlockData.Header
	if isSelf == false {
		if header.HashNoSignsAndNonce() != p.genBlockData.Header.HashNoSignsAndNonce() {
			return common.Hash{}, HashNoSignNotMatchError
		}
		insertHeader = header
	}

	txs := p.genBlockData.Txs
	receipts := p.genBlockData.Receipts
	state := p.genBlockData.State
	block := types.NewBlockWithTxs(insertHeader, txs)

	stat, err := p.blockChain().WriteBlockWithState(block, receipts, state)
	if err != nil {
		log.ERROR(p.logExtraInfo(), "Failed writing block to chain", err)
		return common.Hash{}, err
	}

	// Broadcast the block and announce chain insertion event
	hash := block.Hash()
	p.eventMux().Post(core.NewMinedBlockEvent{Block: block})
	var (
		events []interface{}
		logs   = state.Logs()
	)
	events = append(events, core.ChainEvent{Block: block, Hash: hash, Logs: logs})
	if stat == core.CanonStatTy {
		events = append(events, core.ChainHeadEvent{Block: block})
	}
	p.blockChain().PostChainEvents(events, logs)
	mc.PublishEvent(mc.BlockGenor_HeaderGenerateReq, p.number+1)
	return hash, nil
}

func (p *Process) genValidatorList(newHeader *types.Header) (*mc.TopologyGraph, error) {
	newGraph, err := p.blockChain().NewTopologyGraph(newHeader)
	if err != nil {
		return nil, errors.Errorf("创建新拓扑图失败:%v", err)
	}

	validators := &mc.TopologyGraph{
		Number:   newGraph.Number,
		NodeList: make([]mc.TopologyNodeInfo, 0),
	}

	for _, node := range newGraph.NodeList {
		if node.Type == common.RoleValidator {
			validators.NodeList = append(validators.NodeList, node)
		}
	}
	return validators, nil
}
