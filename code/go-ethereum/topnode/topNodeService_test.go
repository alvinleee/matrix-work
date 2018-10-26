package topnode

import (
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/mtxdpos"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/mc"
	"github.com/pborman/uuid"
	"reflect"
	"testing"
	"time"
)

var (
	testServs  []testNodeService
	fullstate  = []uint8{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}
	offState   = []uint8{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 0, 0}
	dposStocks = make(map[common.Address]uint16)
	nodeInfo   = make([]NodeOnLineInfo, 11)
)

type testDPOSEngine struct {
	dops *mtxdpos.MtxDPOS
}

func (tsdpos *testDPOSEngine) VerifyBlock(header *types.Header) error {
	return tsdpos.dops.VerifyBlock(header)
}

func (tsdpos *testDPOSEngine)VerifyBlocks(headers []*types.Header) error{
	return nil
}
//verify hash in current block
func (tsdpos *testDPOSEngine) VerifyHash(signHash common.Hash, signs []common.Signature) ([]common.Signature, error) {
	return tsdpos.dops.VerifyHash(signHash, signs)
}
func (tsdpos *testDPOSEngine)VerifyStocksWithNumber(validators []common.Address, number uint64) bool{
	return tsdpos.dops.VerifyStocksWithNumber(validators,number)
}

//verify hash in given number block
func (tsdpos *testDPOSEngine) VerifyHashWithNumber(signHash common.Hash, signs []common.Signature, number uint64) ([]common.Signature, error) {
	return tsdpos.dops.VerifyHashWithStocks(signHash, signs, dposStocks)
}

//VerifyHashWithStocks(signHash common.Hash, signs []common.Signature, stocks map[common.Address]uint16) ([]common.Signature, error)

func (tsdpos *testDPOSEngine) VerifyHashWithVerifiedSigns(signs []*common.VerifiedSign) ([]common.Signature, error) {
	return tsdpos.dops.VerifyHashWithVerifiedSigns(signs)
}

func (tsdpos *testDPOSEngine) VerifyHashWithVerifiedSignsAndNumber(signs []*common.VerifiedSign, number uint64) ([]common.Signature, error) {
	return tsdpos.dops.VerifyHashWithVerifiedSignsAndNumber(signs, number)
}

type Center struct {
	FeedMap map[mc.EventCode]*event.Feed
}

func newCenter() *Center {
	msgCenter := &Center{FeedMap: make(map[mc.EventCode]*event.Feed)}
	for i := 0; i < int(mc.LastEventCode); i++ {
		msgCenter.FeedMap[mc.EventCode(i)] = new(event.Feed)
	}
	return msgCenter
}
func (cen *Center) SubscribeEvent(aim mc.EventCode, ch interface{}) (event.Subscription, error) {
	feed, ok := cen.FeedMap[aim]
	if !ok {
		return nil, mc.SubErrorNoThisEvent
	}
	return feed.Subscribe(ch), nil
}

func (cen *Center) PublishEvent(aim mc.EventCode, data interface{}) error {
	feed, ok := cen.FeedMap[aim]
	if !ok {
		return mc.PostErrorNoThisEvent
	}
	feed.Send(data)
	return nil
}

type testNodeState struct {
	self keystore.Key
}

func newTestNodeState() *testNodeState {
	key, _ := crypto.GenerateKey()
	id := uuid.NewRandom()
	keystore := keystore.Key{
		Id:         id,
		Address:    crypto.PubkeyToAddress(key.PublicKey),
		PrivateKey: key,
	}
	return &testNodeState{keystore}
}
func (ts *testNodeState) GetTopNodeOnlineState() []NodeOnLineInfo {

	return nodeInfo
}
func (ts *testNodeState) SendNodeMsg(subCode mc.EventCode, msg interface{}, Roles common.RoleType, address []common.Address){
	switch msg.(type) {
	case *mc.HD_OnlineConsensusReqs:
		data := msg.(*mc.HD_OnlineConsensusReqs)
		for i := 0; i < len(data.ReqList); i++ {
			data.ReqList[i].Leader = ts.self.Address
		}
		for _, serv := range testServs {
			serv.msgChan <- msg
		}

		//		serv.TN.msgCenter.PublishEvent(mc.HD_TopNodeConsensusReq,data.(*mc.OnlineConsensusReqs))
	case *mc.HD_OnlineConsensusVotes:
		data := msg.(*mc.HD_OnlineConsensusVotes)
		for i := 0; i < len(data.Votes); i++ {
			data.Votes[i].From= ts.self.Address
		}
		for _, serv := range testServs {
			serv.msgChan <- msg
		}

		//		testServs[1].msgChan <-msg
		//		serv.TN.msgCenter.PublishEvent(mc.HD_TopNodeConsensusVote,data.(*mc.HD_OnlineConsensusVotes))
	default:
		for _, serv := range testServs {
			serv.msgChan <- msg
		}
		//		log.Error("Type Error","type",reflect.TypeOf(data))
	}
	//	for _,serv := range testServs{
	//		serv.msgChan <-msg
	//	}
}

func (ts *testNodeState) SignWithValidate(hash []byte, validate bool) (common.Signature, error) {
	sigByte, err:=crypto.SignWithValidate(hash, validate, ts.self.PrivateKey)
	if err!=nil{
		return common.Signature{}, err
	}
	return common.BytesToSignature(sigByte), nil
}

func (ts *testNodeState) IsSelfAddress(addr common.Address) bool {
	return ts.self.Address == addr
}

type testNodeService struct {
	TN       *TopNodeService
	msgChan  chan interface{}
	testInfo *testNodeState
}

func (serv *testNodeService) getMessageLoop() {
	for {
		select {
		case data := <-serv.msgChan:
			switch data.(type) {
			case *mc.LeaderChangeNotify:
				serv.TN.msgCenter.PublishEvent(mc.Leader_LeaderChangeNotify, data.(*mc.LeaderChangeNotify))
			case *mc.HD_OnlineConsensusReqs:
				serv.TN.msgCenter.PublishEvent(mc.HD_TopNodeConsensusReq, data.(*mc.HD_OnlineConsensusReqs))
			case *mc.HD_OnlineConsensusVotes:
				serv.TN.msgCenter.PublishEvent(mc.HD_TopNodeConsensusVote, data.(*mc.HD_OnlineConsensusVotes))
			default:
				log.Error("Type Error", "type", reflect.TypeOf(data))
			}
		}
	}
}
func newTestNodeService(testInfo *testNodeState) *TopNodeService {
	testDpos:=testDPOSEngine{ dops:mtxdpos.NewMtxDPOS(nil)}
	testServ := NewTopNodeService(testDpos.dops)
	testServ.topNodeState = testInfo
	testServ.validatorSign = testInfo
	testServ.msgSender = testInfo
	testServ.msgCenter = newCenter()

	testServ.Start()

	return testServ

}
func newTestServer() {

	testServs = make([]testNodeService, 11)
	nodes := make([]common.Address, 11)
	for i := 0; i < 11; i++ {
		testServs[i].msgChan = make(chan interface{}, 10)
		testServs[i].testInfo = newTestNodeState()
		testServs[i].TN = newTestNodeService(testServs[i].testInfo)
		nodes[i] = testServs[i].testInfo.self.Address
		dposStocks[nodes[i]] = 1
		go testServs[i].getMessageLoop()
	}
	for i := 0; i < 11; i++ {
		testServs[i].TN.stateMap.setElectNodes(nodes, 10)
	}
	for i := 0; i < 11; i++ {
		nodeInfo[i].Address = testServs[i].testInfo.self.Address
		if i == 9 {
			nodeInfo[i].OnlineState = offState
		} else {
			nodeInfo[i].OnlineState = fullstate
		}
	}

}
func setLeader(index int, number uint64, turn uint8) {
	serv := testServs[index]
	leader := mc.LeaderChangeNotify{
		ConsensusState: true,
		Leader:         serv.testInfo.self.Address,
		Number:         number,
		ReelectTurn:    turn,
	}
	serv.TN.msgSender.SendNodeMsg(mc.Leader_LeaderChangeNotify, &leader, common.RoleValidator, nil)
}
func TestNewTopNodeService(t *testing.T) {
	log.InitLog(1)
	newTestServer()
	go setLeader(0, 1, 0)
	time.Sleep(time.Second * 5)
	for i := 0; i < 11; i++ {
		t.Log(testServs[i].TN.stateMap.finishedProposal.DPosVoteS[0].Proposal)
	}

}
func TestNewTopNodeServiceRound(t *testing.T) {
	log.InitLog(1)
	newTestServer()
	go func() {
		setLeader(0, 1, 0)
		time.Sleep(time.Second)
		nodeInfo[7].OnlineState = offState
		setLeader(1, 2, 0)
	}()
	time.Sleep(time.Second * 5)
	for i := 0; i < 11; i++ {
		t.Log(testServs[i].TN.stateMap.finishedProposal.DPosVoteS[0].Proposal)
		t.Log(testServs[i].TN.stateMap.finishedProposal.DPosVoteS[1].Proposal)
	}

}
