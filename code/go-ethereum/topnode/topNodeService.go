package topnode

import (
	"errors"
	"reflect"

	"github.com/ethereum/go-ethereum/ca"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/mc"
)

var (
	voteFailed        = errors.New("Vote error")
	topologyValidator = 10
)

type TopNodeService struct {
	stateMap       *topNodeState
	msgCheck       messageCheck
	dposRing       *DPosVoteRing
	dposResultRing *DPosVoteRing

	topNodeState  TopNodeStateInterface
	validatorSign ValidatorAccountInterface
	msgSender     MessageSendInterface
	msgCenter     MessageCenterInterface
	cd            consensus.DPOSEngine

	leaderChangeCh     chan *mc.LeaderChangeNotify
	leaderChangeSub    event.Subscription
	consensusReqCh     chan *mc.HD_OnlineConsensusReqs //顶层节点共识请求消息
	consensusReqSub    event.Subscription
	consensusVoteCh    chan *mc.HD_OnlineConsensusVotes //顶层节点共识投票消息
	consensusVoteSub   event.Subscription
	consensusResultCh  chan *mc.HD_OnlineConsensusVoteResultMsg //顶层节点共识结果消息
	consensusResultSub event.Subscription
	quitCh             chan struct{}
	extraInfo          string
}

func NewTopNodeService(cd consensus.DPOSEngine) *TopNodeService {
	t := &TopNodeService{
		stateMap:          newTopNodeState(64),
		msgCheck:          messageCheck{},
		dposRing:          NewDPosVoteRing(64),
		dposResultRing:    NewDPosVoteRing(32),
		cd:                cd,
		leaderChangeCh:    make(chan *mc.LeaderChangeNotify, 5),
		consensusReqCh:    make(chan *mc.HD_OnlineConsensusReqs, 5),
		consensusVoteCh:   make(chan *mc.HD_OnlineConsensusVotes, 5),
		consensusResultCh: make(chan *mc.HD_OnlineConsensusVoteResultMsg, 5),
		quitCh:            make(chan struct{}, 2),
		extraInfo:         "TopnodeOnline",
	}
	//	go t.update()

	return t
}

func (self *TopNodeService) SetTopNodeStateInterface(inter TopNodeStateInterface) {
	self.topNodeState = inter
}

func (self *TopNodeService) SetValidatorAccountInterface(inter ValidatorAccountInterface) {
	self.validatorSign = inter
}

func (self *TopNodeService) SetMessageSendInterface(inter MessageSendInterface) {
	self.msgSender = inter
}

func (self *TopNodeService) SetMessageCenterInterface(inter MessageCenterInterface) {
	self.msgCenter = inter
}

func (self *TopNodeService) Start() error {
	err := self.subMsg()
	if err != nil {
		return err
	}

	go self.update()
	return nil
}

func (self *TopNodeService) subMsg() error {
	var err error

	//订阅leader变化消息
	if self.leaderChangeSub, err = self.msgCenter.SubscribeEvent(mc.Leader_LeaderChangeNotify, self.leaderChangeCh); err != nil {
		log.Error(self.extraInfo, "SubscribeEvent LeaderChangeNotify failed.", err)
		return err
	}
	//订阅顶层节点状态共识请求消息
	if self.consensusReqSub, err = self.msgCenter.SubscribeEvent(mc.HD_TopNodeConsensusReq, self.consensusReqCh); err != nil {
		log.Error(self.extraInfo, "SubscribeEvent HD_TopNodeConsensusReq failed.", err)
		return err
	}
	//订共识投票消息
	if self.consensusVoteSub, err = self.msgCenter.SubscribeEvent(mc.HD_TopNodeConsensusVote, self.consensusVoteCh); err != nil {
		log.Error(self.extraInfo, "SubscribeEvent HD_TopNodeConsensusVote failed.", err)
		return err
	}
	//订阅共识结果消息
	if self.consensusResultSub, err = self.msgCenter.SubscribeEvent(mc.HD_TopNodeConsensusVoteResult, self.consensusResultCh); err != nil {
		log.Error(self.extraInfo, "SubscribeEvent HD_TopNodeConsensusVoteResult failed.", err)
		return err
	}

	log.Info(self.extraInfo, "服务订阅完成", "")
	return nil
}

func (self *TopNodeService) unSubMsg() {
	log.Info(self.extraInfo, "开始取消服务订阅", "")
	//取消订阅leader变化消息

	self.leaderChangeSub.Unsubscribe()

	//取消订阅顶层节点状态共识请求消息

	self.consensusReqSub.Unsubscribe()
	//取消订共识投票消息

	self.consensusVoteSub.Unsubscribe()
	//取消订阅共识结果消息

	self.consensusResultSub.Unsubscribe()
	log.Info(self.extraInfo, "取消服务订阅完成", "")

}

func (serv *TopNodeService) update() {
	log.Info(serv.extraInfo, "启动顶层节点服务，等待接收消息", "")
	defer serv.unSubMsg()
	for {
		select {

		case data := <-serv.leaderChangeCh:
			if !data.Leader.Equal(serv.msgCheck.getLeader()) {
				if serv.msgCheck.checkLeaderChangeNotify(data) {
					log.Info(serv.extraInfo, "收到leader变更通知消息", "", "块高", data.Number)

					go serv.LeaderChangeNotifyHandler(data)
				}
			}
		case data := <-serv.consensusReqCh:
			log.Info(serv.extraInfo, "收到共识请求消息", "", "from", data.From.String())
			go serv.consensusReqMsgHandler(data.ReqList)

		case data := <-serv.consensusVoteCh:
			log.Info(serv.extraInfo, "收到共识投票消息", "")
			go serv.consensusVoteMsgHandler(data.Votes)
		case data := <-serv.consensusResultCh:
			log.Info(serv.extraInfo, "收到共识结果消息", "")
			go serv.OnlineConsensusVoteResultMsgHandler(data)
		case <-serv.quitCh:
			log.Info(serv.extraInfo, "收到退出消息", "")
			return
		}
	}
}
func (self *TopNodeService) LeaderChangeNotifyHandler(msg *mc.LeaderChangeNotify) {
	if msg == nil {
		log.Error(self.extraInfo, "leader变更消息", "空消息")
		return
	}

	if self.validatorSign.IsSelfAddress(msg.Leader) {
		log.Info(self.extraInfo, "我是leader", "准备检查顶层节点在线状态")

		self.checkTopNodeState()
	} else {
		for _, item := range self.dposRing.DPosVoteS {
			go self.consensusVotes(item.getVotes())
		}
	}
}

func (serv *TopNodeService) getTopNodeState() (online, offline []common.Address) {
	log.Info("topnode", "获取顶层节点在线状态", "")
	return serv.stateMap.newTopNodeState(serv.topNodeState.GetTopNodeOnlineState(), serv.msgCheck.getLeader())
}
func (serv *TopNodeService) checkTopNodeState() {

	serv.sendRequest(serv.getTopNodeState())
}
func (serv *TopNodeService) sendRequest(online, offline []common.Address) {
	leader := ca.GetAddress()
	reqMsg := mc.HD_OnlineConsensusReqs{}
	turn := serv.msgCheck.getRound()
	for _, item := range online {
		val := mc.OnlineConsensusReq{
			OnlineState: onLine,
			Leader:      leader,
			Node:        item,
			Seq:         turn,
		}
		reqMsg.ReqList = append(reqMsg.ReqList, &val)
	}
	for _, item := range offline {
		val := mc.OnlineConsensusReq{
			OnlineState: offLine,
			Leader:      leader,
			Node:        item,
			Seq:         turn,
		}
		reqMsg.ReqList = append(reqMsg.ReqList, &val)
	}
	if len(reqMsg.ReqList) > 0 {
		log.Info(serv.extraInfo, "向其他验证者发送共识投票请求", "start", "轮次", turn, "共识数量", len(reqMsg.ReqList))
		serv.msgSender.SendNodeMsg(mc.HD_TopNodeConsensusReq, &reqMsg, common.RoleValidator, nil)
		log.Info(serv.extraInfo, "发送共识投票请求", "done")
		go func() {
			log.Info(serv.extraInfo, "向自己发送共识投票请求", "start", "轮次", turn, "共识数量", len(reqMsg.ReqList))
			serv.consensusReqCh <- &reqMsg
			log.Info(serv.extraInfo, "向自己发送共识投票请求", "done")
		}()
	}
}

func (serv *TopNodeService) consensusReqMsgHandler(requests []*mc.OnlineConsensusReq) {

	if requests == nil || len(requests) == 0 {
		log.Error(serv.extraInfo, "invalid parameter", "")
		return
	}
	var votes mc.HD_OnlineConsensusVotes
	log.Info(serv.extraInfo, "开始投票", "")
	for _, item := range requests {
		if serv.msgCheck.checkOnlineConsensusReq(item) {
			if serv.dposRing.addProposal(types.RlpHash(item), item) {
				sign, reqHash, err := serv.voteToReq(item)
				if err == nil {
					vote := mc.HD_ConsensusVote{
						SignHash: reqHash,
						Sign:     sign,
						Round:    item.Seq,
						From:     ca.GetAddress(),
					}
					votes.Votes = append(votes.Votes, vote)

				} else {
					log.Error(serv.extraInfo, "error", err)
				}

			} else {
				log.Error(serv.extraInfo, "addProposal", "false", "item", item.Node.String(), "seq", item.Seq, "leader", item.Leader.String())
			}
		} else {
			log.Error(serv.extraInfo, "checkOnlineConsensusReq", "false", "node", item.Node.String(), "轮次", item.Seq)
		}
	}
	if len(votes.Votes) > 0 {
		log.Info(serv.extraInfo, "完成投票", "发送共识投票消息")
		serv.msgSender.SendNodeMsg(mc.HD_TopNodeConsensusVote, &votes, common.RoleValidator, nil)
		go func() {
			log.Info(serv.extraInfo, "发送共识投票消息", "start")
			serv.consensusVoteCh <- &votes
			log.Info(serv.extraInfo, "发送共识投票消息", "done")
		}()
		log.Info("test info", "type", reflect.TypeOf(votes))
	}
}
func (serv *TopNodeService) consensusVoteMsgHandler(msg []mc.HD_ConsensusVote) {
	if msg == nil || len(msg) == 0 {
		log.Error(serv.extraInfo, "invalid parameter", "", "len(msg)", len(msg))
		return
	}

	for _, item := range msg {
		serv.consensusVotes(serv.dposRing.addVote(item.SignHash, &item))
	}
}
func (serv *TopNodeService) OnlineConsensusVoteResultMsgHandler(msg *mc.HD_OnlineConsensusVoteResultMsg) {
	tempSigns, err := serv.cd.VerifyHashWithNumber(types.RlpHash(msg.Req), msg.SignList, serv.msgCheck.getBlockHeight())
	if err != nil {
		log.Info(serv.extraInfo, "handle DPOS result message error", err)
	} else {
		log.Info(serv.extraInfo, "handle DPOS result message", "sucess", "状态", msg.Req.OnlineState, "投票数", len(tempSigns))
		finishHash := getFinishedPropocalHash(msg.Req.Node, uint8(msg.Req.OnlineState))
		vote := mc.HD_ConsensusVote{
			finishHash,
			0,
			common.Signature{},
			msg.From,
		}
		log.Info("topnodeOnline", "finishHash", finishHash)
		proposal, voteInfo := serv.dposResultRing.addVote(finishHash, &vote)
		for _, value := range voteInfo {
			log.Info("topnodeOnline", "投票from", value.data.From.String())

		}
		log.Info("topnodeOnline", "投票数", len(voteInfo))

		if serv.checkPosVoteResults(proposal, voteInfo) {
			log.Info("topnodeOnline", "保存经过共识的节点", msg.Req.Node.String(), "在线状态", msg.Req.OnlineState)
			serv.stateMap.saveConsensusNodeState(msg.Req.Node, OnlineState(msg.Req.OnlineState))
		}
	}
}
func (serv *TopNodeService) checkPosVoteResults(proposal interface{}, votes []voteInfo) bool {
	if votes == nil || len(votes) == 0 {
		log.Error("topnodeOnline", "len of votes", len(votes))
		return false
	}
	validators := make([]common.Address, 0)
	for _, value := range votes {
		validators = append(validators, value.data.From)
	}
	return serv.cd.VerifyStocksWithNumber(validators, serv.msgCheck.getBlockHeight())
}

func (serv *TopNodeService) consensusVotes(proposal interface{}, votes []voteInfo) {
	if proposal == nil || votes == nil || len(votes) == 0 {
		log.Error("topnodeOnline", "proposal", proposal, "votes", len(votes))
		return
	}
	log.Info(serv.extraInfo, "开始处理共识投票", "")
	prop := proposal.(*mc.OnlineConsensusReq)
	if serv.msgCheck.getLeader() != prop.Leader {
		log.Info(serv.extraInfo, "invalid Leader", prop.Leader, "leader", serv.msgCheck.getLeader())
		return
	}
	if !serv.msgCheck.checkRound(prop.Seq) {
		log.Info(serv.extraInfo, "invalid Round", prop.Seq)
	}
	signList := make([]common.Signature, 0)
	for _, value := range votes {
		signList = append(signList, value.data.Sign)
	}
	tempSigns, err := serv.cd.VerifyHashWithNumber(votes[0].data.SignHash, signList, serv.msgCheck.getBlockHeight())
	if err != nil {
		log.Info(serv.extraInfo, "DPOS共识失败", err)
		return
	}
	log.Info(serv.extraInfo, "处理共识投票消息", "DPOS共识成功", "节点", prop.Node, "状态", prop.OnlineState, "投票数", len(tempSigns))
	serv.stateMap.finishedProposal.addProposal(getFinishedPropocalHash(prop.Node, uint8(prop.OnlineState)), proposal)
	log.Info(serv.extraInfo, "finishHash", getFinishedPropocalHash(prop.Node, uint8(prop.OnlineState)))
	//send DPos Success message
	result := mc.HD_OnlineConsensusVoteResultMsg{
		Req:      prop,
		SignList: signList,
		From:     ca.GetAddress(),
	}
	log.Info(serv.extraInfo, "发送共识投票结果消息", "start")

	serv.msgSender.SendNodeMsg(mc.HD_TopNodeConsensusVoteResult, &result, common.RoleValidator, nil)
	go func() {
		serv.consensusResultCh <- &result
		log.Info(serv.extraInfo, "send consensus votes Result Msg to self", "done")
	}()
	log.Info(serv.extraInfo, "发送共识投票结果消息", "done")

	//serv.stateMap.saveTopnodeState(prop.Node, OnlineState(prop.OnlineState))

}

func (serv *TopNodeService) voteToReq(tempReq *mc.OnlineConsensusReq) (common.Signature, common.Hash, error) {
	var sign common.Signature
	var err error

	reqHash := types.RlpHash(tempReq)
	log.Info(serv.extraInfo, "对共识请求进行投票", "", "轮次", tempReq.Seq, "node", tempReq.Node, "状态", tempReq.OnlineState, "leader", tempReq.Leader)
	var ok bool
	if tempReq.OnlineState == onLine {
		ok = serv.stateMap.checkNodeOnline(tempReq.Node, serv.topNodeState.GetTopNodeOnlineState())
		log.Info(serv.extraInfo, "检查状态", "在线", "node", tempReq.Node.String(), "ok", ok)
	} else {
		ok = serv.stateMap.checkNodeOffline(tempReq.Node, serv.topNodeState.GetTopNodeOnlineState())
		log.Info(serv.extraInfo, "检查状态", "离线", "node", tempReq.Node.String(), "ok", ok)

	}
	if ok {
		//投赞成票
		sign, err = serv.validatorSign.SignWithValidate(reqHash.Bytes(), true)
		if err != nil {
			log.Info(serv.extraInfo, "Vote failed:", err)
			return common.Signature{}, common.Hash{}, voteFailed
		}
		log.Info(serv.extraInfo, "投赞成票", "", "reqNode", tempReq.Node)
	} else {
		//投反对票
		sign, err = serv.validatorSign.SignWithValidate(reqHash.Bytes(), false)
		if err != nil {
			log.Info(serv.extraInfo, "Vote failed:", err)
			return common.Signature{}, common.Hash{}, voteFailed
		}
		log.Info(serv.extraInfo, "投反对票", "", "reqNode", tempReq.Node)

	}
	log.Info(serv.extraInfo, "保存共识请求", "", "reqhash", reqHash)
	//	p.consensusReqCache[reqHash] = tempReq
	return sign, reqHash, nil
}

//set ElectNodes
func (serv *TopNodeService) SetElectNodes(nodes []common.Address, height uint64) {
	serv.stateMap.setElectNodes(nodes, height)
}

//设置当前区块的顶点拓扑图，每个验证者都调用
func (serv *TopNodeService) SetCurrentOnlineState(onLineNode, onElectNode []common.Address) {
	//	offLineNode1 := append(offLineNode, offElectNode...)
	serv.stateMap.setCurrentTopNodeState(onLineNode, onElectNode)
}

//提供换届服务获取当前经过共识的在线状态，只有leader调用
func (serv *TopNodeService) GetConsensusOnlineState() (ret_offLineNode, ret_onElectNode, ret_offElectNode []common.Address) {
	ret_offLineNode, ret_onElectNode, ret_offElectNode = serv.stateMap.getCurrentTopNodeChange()
	log.Info("topnodeOnline", "offline长度", len(ret_offLineNode), "online长度", len(ret_onElectNode), "offlineElect长度", len(ret_offElectNode))

	return
}

//verify Online State
func (serv *TopNodeService) CheckAddressConsensusOnlineState(offLineNode, onElectNode, offElectNode []common.Address) bool {
	offLineNode1 := append(offLineNode, offElectNode...)
	check := true
	for _, item := range onElectNode {
		if !serv.stateMap.checkAddressConsesusOnlineState(item, onLine) {
			check = false
			break
		}
	}
	log.Info("topnodeOnline", "检查在线状态", check)

	if check {
		for _, item := range offLineNode1 {
			if !serv.stateMap.checkAddressConsesusOnlineState(item, offLine) {
				check = false
				break
			}
		}
	}
	log.Info("topnodeOnline", "检查离线状态", check)
	return check
}
