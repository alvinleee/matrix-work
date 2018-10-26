package election

import (
	"fmt"
	"math/big"
	"os"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/election/ManElec100/mt19937"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/mc"
	//"github.com/ethereum/go-ethereum/mc"
)

/*
const (
	maxMinerNum              = params.MaxMinerNum              //支持的最大参选矿工数
	maxValidatorNum          = params.MaxValidatorNum          //支持的最大参选验证者数
	maxMasterMinerNum        = params.MaxMasterMinerNum        //顶层节点最大矿工主节点数
	maxMasterValidatorNum    = params.MaxMasterValidatorNum    //顶层节点最大矿工主节点数
	maxBackUpMinerNum        = params.MaxBackUpMinerNum        //最大备份矿工数
	maxCandidateMinerNum     = params.MaxCandidateMinerNum     //最大候补矿工数
	maxBackUpValidatorNum    = params.MaxBackUpValidatorNum    //最大备份验证者数
	maxCandidateValidatorNum = params.MaxCandidateValidatorNum //最大候补验证者数
)
*/

type ElectEventType string

type Mynode struct {
	Nodeid string
	Tps    int
	Stk    float64
	Uptime int
}

type foundnode struct {
	nodeid string
	tps    int
	stk    float64
	uptime int
}

type Elector struct {
	EleMMSub  ElectMMSub
	EleMVSub  ElectMVSub
	EleMMRs   chan *mc.MasterMinerReElectionRsp
	EleMVRs   chan *mc.MasterValidatorReElectionRsq
	Engine    func(probVal []Stf, seed int64, M int, P int, J int) ([]Strallyint, []Strallyint, []Strallyint) //Engine    func(probVal []Stf, seed int64) ([]Strallyint, []Strallyint, []Strallyint) //func(probVal map[string]float32, seed int64) ([]Strallyint, []Strallyint, []Strallyint)
	msgcenter *mc.Center
	MaxSample int //配置参数,采样最多发生1000次,是一个离P+M较远的值
	J         int //基金会验证节点个数tps_weight
	M         int //验证主节点个数
	P         int //备份主节点个数
	N         int //矿工主节点个数
}

type ElectMMSub struct {
	MasterMinerReElectionReqMsgCH  chan *mc.MasterMinerReElectionReqMsg
	MasterMinerReElectionReqMsgSub event.Subscription
}
type ElectMVSub struct {
	MasterValidatorReElectionReqMsgCH  chan *mc.MasterValidatorReElectionReqMsg
	MasterValidatorReElectionReqMsgSub event.Subscription
}

func NewEle() *Elector {
	var ele Elector

	ele.MaxSample = 1000
	ele.J = 0
	ele.M = common.MasterValidatorNum
	ele.P = common.BackupValidatorNum
	ele.N = 21
	ele.EleServer()
	ele.EleMMRs = make(chan *mc.MasterMinerReElectionRsp, 10)
	ele.EleMVRs = make(chan *mc.MasterValidatorReElectionRsq, 10)

	return &ele
}

/////////////////////////////

type Self struct {
	nodeid   string
	stk      float64
	uptime   int
	tps      int
	Coef_tps float64
	Coef_stk float64
}

func (self *Self) TPS_POWER() float64 {
	tps_weight := 1.0
	if self.tps >= 16000 {
		tps_weight = 5.0
	} else if self.tps >= 8000 {
		tps_weight = 4.0
	} else if self.tps >= 4000 {
		tps_weight = 3.0
	} else if self.tps >= 2000 {
		tps_weight = 2.0
	} else if self.tps >= 1000 {
		tps_weight = 1.0
	} else {
		tps_weight = 0.0
	}
	return tps_weight
}

func (self *Self) Last_Time() float64 {
	CandidateTime_weight := 4.0
	if self.uptime <= 64 {
		CandidateTime_weight = 0.25
	} else if self.uptime <= 128 {
		CandidateTime_weight = 0.5
	} else if self.uptime <= 256 {
		CandidateTime_weight = 1
	} else if self.uptime <= 512 {
		CandidateTime_weight = 2
	} else {
		CandidateTime_weight = 4
	}
	return CandidateTime_weight
}

func (self *Self) deposit_stake() float64 {
	stake_weight := 1.0
	if self.stk >= 40000 {
		stake_weight = 4.5
	} else if self.stk >= 20000 {
		stake_weight = 2.15
	} else if self.stk >= 10000 {
		stake_weight = 1.0
	} else {
		stake_weight = 0.0
	}
	return stake_weight
}

type Stf struct {
	Str  string
	Flot float64
}

func CalcAllValueFunction(nodelist []vm.DepositDetail) []Stf { //nodelist []Mynode) map[string]float32 {
	//	CapitalMap := make(map[string]float64)
	//	CapitalMap := make(map[string]float32)
	var CapitalMap []Stf

	for _, item := range nodelist {
		self := Self{nodeid: string(item.NodeID[:]), stk: float64(item.Deposit.Uint64()), uptime: int(item.OnlineTime.Uint64()), tps: 1000, Coef_tps: 0.2, Coef_stk: 0.25}
		value := self.Last_Time() * (self.TPS_POWER()*self.Coef_tps + self.deposit_stake()*self.Coef_stk)
		//		CapitalMap[self.nodeid] = float32(value)
		CapitalMap = append(CapitalMap, Stf{Str: self.nodeid, Flot: float64(value)})
	}
	return CapitalMap
}

type pnormalized struct {
	Value  float64
	Nodeid string
}

type Strallyint struct {
	Value  int
	Nodeid string
}

func Normalize(probVal []Stf) []pnormalized {

	fmt.Println(probVal)
	var total float64
	//	var mlen int
	for _, item := range probVal {
		total += item.Flot
		//		fmt.Println("There are", views, "views for", key)
	}
	var pnormalizedlist []pnormalized
	for _, item := range probVal {
		var tmp pnormalized
		tmp.Value = item.Flot / total
		tmp.Nodeid = item.Str
		pnormalizedlist = append(pnormalizedlist, tmp)
		//		fmt.Println("There are", views, "views for", key)
	}
	return pnormalizedlist
}

func Sample1NodesInValNodes(probnormalized []pnormalized, rand01 float64) string {

	for _, iterm := range probnormalized {
		rand01 -= iterm.Value
		if rand01 < 0 {
			return iterm.Nodeid
		}
	}
	return probnormalized[0].Nodeid
}

func WriteWithFileWrite(name, content string) {
	fileObj, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		fmt.Println("Failed to open the file", err.Error())
		os.Exit(2)
	}
	defer fileObj.Close()
	if _, err := fileObj.WriteString(content); err == nil {
		fmt.Println("Successful writing to the file with os.OpenFile and *File.WriteString method.", content)
	}
}

func sortfunc(probnormalized []pnormalized, PricipalValNodes, BakValNodes, RemainingProbNormalizedNodes []Strallyint) ([]Strallyint, []Strallyint, []Strallyint) {
	Pricipal := make(map[string]int)
	BakVal := make(map[string]int)
	Remain := make(map[string]int)

	var RPricipalValNodes []Strallyint
	var RBakValNodes []Strallyint
	var RRemainingProbNormalizedNodes []Strallyint

	for _, item := range PricipalValNodes {
		Pricipal[item.Nodeid] = item.Value
	}
	for _, item := range BakValNodes {
		BakVal[item.Nodeid] = item.Value
	}
	for _, item := range RemainingProbNormalizedNodes {
		Remain[item.Nodeid] = item.Value
	}

	for _, item := range probnormalized {
		var ok bool
		_, ok = Pricipal[item.Nodeid]
		if ok == true {
			RPricipalValNodes = append(RPricipalValNodes, Strallyint{Nodeid: item.Nodeid, Value: Pricipal[item.Nodeid]})
			continue
		}

		_, ok = BakVal[item.Nodeid]
		if ok == true {
			RBakValNodes = append(RBakValNodes, Strallyint{Nodeid: item.Nodeid, Value: BakVal[item.Nodeid]})
			continue
		}

		_, ok = Remain[item.Nodeid]
		if ok == true {
			RRemainingProbNormalizedNodes = append(RRemainingProbNormalizedNodes, Strallyint{Nodeid: item.Nodeid, Value: Remain[item.Nodeid]})
			continue
		}

	}
	return RPricipalValNodes, RBakValNodes, RRemainingProbNormalizedNodes
}

func (Ele *Elector) SampleMinerNodes(probnormalized []pnormalized, seed int64, Ms int) ([]Strallyint, []Strallyint) {

	var PricipalMinerNodes []Strallyint
	var BakMinerNodes []Strallyint

	sort := func(probnormalized []pnormalized, PricipalMinerNodes []Strallyint, BakMinerNodes []Strallyint) ([]Strallyint, []Strallyint) {
		Pricipal := make(map[string]int)
		BakMin := make(map[string]int)

		var RPricipalMinerNodes []Strallyint
		var RBakMinerNodes []Strallyint

		for _, item := range PricipalMinerNodes {
			Pricipal[item.Nodeid] = item.Value
		}
		for _, item := range BakMinerNodes {
			BakMin[item.Nodeid] = item.Value
		}
		for _, item := range probnormalized {
			var ok bool
			_, ok = Pricipal[item.Nodeid]
			if ok == true {
				RPricipalMinerNodes = append(RPricipalMinerNodes, Strallyint{Nodeid: item.Nodeid, Value: Pricipal[item.Nodeid]})
				continue
			}
			_, ok = BakMin[item.Nodeid]
			if ok == true {
				RBakMinerNodes = append(RBakMinerNodes, Strallyint{Nodeid: item.Nodeid, Value: BakMin[item.Nodeid]})
			}
		}
		return RPricipalMinerNodes, RBakMinerNodes
	}

	// 如果当选节点不到N个,其他列表为空
	dict := make(map[string]int)
	Ele.N = Ms
	if len(probnormalized) <= Ele.N { //加判断 定义为func
		for _, item := range probnormalized {
			//			probnormalized[index].value = 100 * iterm.value
			temp := Strallyint{Value: int(100 * item.Value), Nodeid: item.Nodeid}
			PricipalMinerNodes = append(PricipalMinerNodes, temp)
		}
		//		return [(e[0],int(100*e[1])) for e in probnormalized],[],[]
		return sort(probnormalized, PricipalMinerNodes, BakMinerNodes)
	}

	// 如果当选节点超过N,最多连续进行1000次采样或者选出N个节点
	rand := mt19937.RandUniformInit(seed)
	for i := 0; i < Ele.MaxSample; i++ {
		node := Sample1NodesInValNodes(probnormalized, float64(rand.Uniform(0.0, 1.0)))
		_, ok := dict[node]
		if ok == true {
			dict[node] = dict[node] + 1
		} else {
			dict[node] = 1
		}
		if len(dict) == Ele.N {
			break
		}
	}

	// 如果没有选够N个
	for _, item := range probnormalized {
		vint, ok := dict[item.Nodeid]

		if ok == true {
			var tmp Strallyint
			tmp.Nodeid = item.Nodeid
			tmp.Value = vint
			PricipalMinerNodes = append(PricipalMinerNodes, tmp)
		} else {
			BakMinerNodes = append(BakMinerNodes, Strallyint{Value: int(item.Value), Nodeid: item.Nodeid})
		}
	}
	lenPM := len(PricipalMinerNodes)
	if Ele.N > lenPM {
		PricipalMinerNodes = append(PricipalMinerNodes, BakMinerNodes[:Ele.N-lenPM]...)
		BakMinerNodes = BakMinerNodes[Ele.N-lenPM:]
	}
	return sort(probnormalized, PricipalMinerNodes, BakMinerNodes)
}

func CalcRemainingNodesVotes(RemainingProbNormalizedNodes []Strallyint) []Strallyint {
	for index, _ := range RemainingProbNormalizedNodes {
		RemainingProbNormalizedNodes[index].Value = 1
	}
	return RemainingProbNormalizedNodes
}

//做异常判断
func (Ele *Elector) SampleMPlusPNodes(probnormalized []pnormalized, seed int64) ([]Strallyint, []Strallyint, []Strallyint) {
	var PricipalValNodes []Strallyint
	var RemainingProbNormalizedNodes []Strallyint //[]pnormalized
	var BakValNodes []Strallyint

	// 如果当选节点不到M-J个(加上基金会节点不足M个),则全部当选,其他列表为空
	dict := make(map[string]int)
	if len(probnormalized) <= Ele.M-Ele.J { //加判断 定义为func
		for _, item := range probnormalized {
			temp := Strallyint{Value: int(100 * item.Value), Nodeid: item.Nodeid}
			PricipalValNodes = append(PricipalValNodes, temp)
		}
		//		return sortfunc(probnormalized, PricipalValNodes, BakValNodes, RemainingProbNormalizedNodes)
		return PricipalValNodes, BakValNodes, RemainingProbNormalizedNodes
	}

	// 如果当选节点超过M-J,最多连续进行1000次采样或者选出M+P-J个节点
	rand := mt19937.RandUniformInit(seed)

	for i := 0; i < Ele.MaxSample; i++ {
		node := Sample1NodesInValNodes(probnormalized, float64(rand.Uniform(0.0, 1.0)))

		_, ok := dict[node]
		if ok == true {
			dict[node] = dict[node] + 1
		} else {
			dict[node] = 1
		}

		if len(dict) == (Ele.M - Ele.J) {
			break
		}
	}
	for _, item := range probnormalized {
		_, ok := dict[item.Nodeid]
		if ok == false {
			RemainingProbNormalizedNodes = append(RemainingProbNormalizedNodes, Strallyint{Nodeid: item.Nodeid, Value: dict[item.Nodeid]})
		} else {
			PricipalValNodes = append(PricipalValNodes, Strallyint{Nodeid: item.Nodeid, Value: dict[item.Nodeid]})
		}
	}
	return PricipalValNodes, BakValNodes, RemainingProbNormalizedNodes
}

//func (Ele *Elector) ValNodesSelected(probVal []Stf, seed int64) ([]Strallyint, []Strallyint, []Strallyint) {
func (Ele *Elector) ValNodesSelected(probVal []Stf, seed int64, M int, P int, J int) ([]Strallyint, []Strallyint, []Strallyint) {

	Ele.M = M
	Ele.P = P
	Ele.J = J
	//归一化价值函数 成为 采样概率
	probnormalized := Normalize(probVal)
	fmt.Println(probnormalized)
	// 选出M+P-J个节点 或者 进行1000次采样
	PricipalValNodes, BakValNodes, RemainingProbNormalizedNodes := Ele.SampleMPlusPNodes(probnormalized, seed) //SampleMPlusPNodes(probnormalized=probnormalized,seed=seed,M=M,J=J,P=5,MaxSample=MaxSample)
	// 计算所有剩余节点的股权RemainingProbNormalizedNodes
	RemainingValNodes := CalcRemainingNodesVotes(RemainingProbNormalizedNodes)

	// 基金会节点加入验证主节点列表
	// 如果验证主节点不足N个,使用剩余节点列表补足M-J个
	for len(PricipalValNodes) < Ele.M-Ele.J && len(RemainingValNodes) > 0 {
		PricipalValNodes = append(PricipalValNodes, RemainingValNodes[0])
		RemainingValNodes = RemainingValNodes[1:]
	}
	// 如果备份主节点不足P个,使用剩余节点列表补足P个
	for len(BakValNodes) < Ele.P && len(RemainingValNodes) > 0 {
		BakValNodes = append(BakValNodes, RemainingValNodes[0])
		RemainingValNodes = RemainingValNodes[1:]
	}
	return PricipalValNodes, BakValNodes, RemainingValNodes
}

func (Ele *Elector) MinerNodesSelected(probVal []Stf, seed int64, Ms int) ([]Strallyint, []Strallyint) {
	probnormalized := Normalize(probVal)

	fmt.Println(probnormalized)
	PricipalMinerNodes, BakMinerNodes := Ele.SampleMinerNodes(probnormalized, seed, Ms)

	//计算所有剩余节点的股权
	BakMinerNodes = CalcRemainingNodesVotes(BakMinerNodes)
	return PricipalMinerNodes, BakMinerNodes
}

func (Ele *Elector) ChoiceEngine(flag int) {
	if flag == 1 {
		Ele.Engine = Ele.ValNodesSelected
	}
}

func updownlimit(a float64, ratiouplimit float64, ratiodnlimit float64) float64 {
	if a < ratiodnlimit {
		a = ratiodnlimit
	}
	if a > ratiouplimit {
		a = ratiouplimit
	}
	return a
}

func (Ele *Elector) CommbineFundNodesAndPricipal(probVal []Stf, probFund []Stf, PricipalValNodes []Strallyint, ratiodnlimit float64, ratiouplimit float64) []Strallyint {

	if (Ele.J == 0 || len(probFund) == 0) && len(PricipalValNodes) == 0 {
		var empty []Strallyint
		return empty
	}

	if Ele.J == 0 || len(probFund) == 0 {
		return PricipalValNodes
	}
	if len(PricipalValNodes) == 0 {
		var probFundnormalized []Strallyint
		temp := Normalize(probFund)
		for _, item := range temp {
			var elem *Strallyint
			elem = new(Strallyint)
			elem.Nodeid = item.Nodeid
			elem.Value = int(item.Value * 100)
			probFundnormalized = append(probFundnormalized, *elem)
		}
		return probFundnormalized
	}

	var PricipalVoteSum int
	var probPricipalSum float64
	var probFundSum float64
	var ratio float64

	for _, item := range PricipalValNodes {
		//		probFundnormalized[index].Value *= 100
		PricipalVoteSum += item.Value
	}

	//	var pricipalkeys []string
	var probValMap map[string]float64
	probValMap = make(map[string]float64)
	for _, item := range probVal {
		probValMap[item.Str] = item.Flot
	}

	//根据主验证节点找到对应的价值 计算选举出的主验证价值和
	for _, item := range PricipalValNodes {
		v := probValMap[item.Nodeid]
		probPricipalSum += v
	}

	//计算基金会节点价值和
	for _, item := range probFund {
		probFundSum += item.Flot
	}

	//计算比率
	ratio = updownlimit(probFundSum/probPricipalSum, 2.5, 4.0)

	var FundValNodes []Strallyint
	var temp *Strallyint
	temp = new(Strallyint)
	for _, item := range probFund {
		temp.Nodeid = item.Str
		//基金会节点价值 / 基金会总价值 * 比率 * 竞选到的主验证节点的投票总数
		temp.Value = int(item.Flot / probFundSum * ratio * float64(PricipalVoteSum))
		FundValNodes = append(FundValNodes, *temp)
	}

	PricipalValNodes = append(PricipalValNodes, FundValNodes...)
	//	FundValNodes = append(FundValNodes, PricipalValNodes...)
	return PricipalValNodes
}

func (Ele *Elector) EleServer() {

	log.Info("Elector EleServer")
	//订阅消息
	Ele.EleMMSub = ElectMMSub{MasterMinerReElectionReqMsgCH: make(chan *mc.MasterMinerReElectionReqMsg, 10)}
	Ele.EleMMSub.MasterMinerReElectionReqMsgSub, _ = mc.SubscribeEvent(mc.ReElec_MasterMinerReElectionReq, Ele.EleMMSub.MasterMinerReElectionReqMsgCH)

	Ele.EleMVSub = ElectMVSub{MasterValidatorReElectionReqMsgCH: make(chan *mc.MasterValidatorReElectionReqMsg, 10)}
	Ele.EleMVSub.MasterValidatorReElectionReqMsgSub, _ = mc.SubscribeEvent(mc.ReElec_MasterValidatorElectionReq, Ele.EleMVSub.MasterValidatorReElectionReqMsgCH)

	//选择引擎
	Ele.ChoiceEngine(1)

	//开启监听
	go Ele.Listen()

	//返回消息
	go Ele.Post()
}

func (Ele *Elector) Post() {
	log.Info("Elector Post")
	for {
		select {
		case mmrers := <-Ele.EleMMRs:
			//			time.Sleep(5 * time.Second)
			log.Info("Elector Post", "Topo_MasterMinerElectionRsp", mmrers)
			err := mc.PublishEvent(mc.Topo_MasterMinerElectionRsp, mmrers) //mc.MasterMinerReElectionRspMsg{SeqNum: 666})
			log.Info("Post发送状态", err)

		case mvrers := <-Ele.EleMVRs:
			log.Info("Elector Post", "Topo_MasterValidatorElectionRsp", mvrers)
			err1 := mc.PublishEvent(mc.Topo_MasterValidatorElectionRsp, mvrers) //mc.MasterValidatorReElectionRspMsg{SeqNum: 666})
			log.Info("Post发送状态", err1)
		}
	}
}

func (Ele *Elector) Listen() {

	defer Ele.EleMMSub.MasterMinerReElectionReqMsgSub.Unsubscribe()
	defer Ele.EleMVSub.MasterValidatorReElectionReqMsgSub.Unsubscribe()

	log.Info("Elector Listen")
	for {
		select {
		case mmrerm := <-Ele.EleMMSub.MasterMinerReElectionReqMsgCH:
			MinerElectMap := make(map[string]vm.DepositDetail)
			for i, item := range mmrerm.MinerList {
				//				MinerElectMap[string(item.Account[:])] = item
				MinerElectMap[string(item.NodeID[:])] = item
				if item.Deposit == nil {
					mmrerm.MinerList[i].Deposit = big.NewInt(50000)
				}
				if item.WithdrawH == nil {
					mmrerm.MinerList[i].WithdrawH = big.NewInt(0)
				}
				if item.OnlineTime == nil {
					mmrerm.MinerList[i].OnlineTime = big.NewInt(300)
				}
			}

			value := CalcAllValueFunction(mmrerm.MinerList)

			a, b := Ele.MinerNodesSelected(value, mmrerm.RandSeed.Int64(), 21) //Ele.Engine(value, mmrerm.RandSeed.Int64()) //0x12217)
			for index, item := range a {
				fmt.Println(index, item)
			}
			for index, item := range b {
				fmt.Println(index, item)
			}
			MinerEleRs := new(mc.MasterMinerReElectionRsp)
			MinerEleRs.SeqNum = mmrerm.SeqNum

			for index, item := range a {
				fmt.Println(item.Nodeid, []byte(item.Nodeid))
				tmp := MinerElectMap[item.Nodeid]
				var ToG mc.TopologyNodeInfo
				ToG.Account = tmp.Address
				ToG.Position = uint16(index)
				ToG.Type = common.RoleMiner
				ToG.Stock = uint16(item.Value)
				MinerEleRs.MasterMiner = append(MinerEleRs.MasterMiner, ToG)
			}

			for index, item := range b {
				tmp := MinerElectMap[item.Nodeid]
				var ToG mc.TopologyNodeInfo
				ToG.Account = tmp.Address
				//				ToG.OnlineState = true
				ToG.Position = uint16(index)
				ToG.Type = common.RoleMiner
				ToG.Stock = uint16(item.Value)
				MinerEleRs.BackUpMiner = append(MinerEleRs.BackUpMiner, ToG)
			}

			Ele.EleMMRs <- MinerEleRs
			//			Ele.EleMVRs <- fmt.Println(value)
			//fmt.Println("收到数据", mmrerm)

		case mvrerm := <-Ele.EleMVSub.MasterValidatorReElectionReqMsgCH:
			log.Info("Elector Listen", "ReElec_MasterValidatorElectionReq", mvrerm)
			ValidatorElectMap := make(map[string]vm.DepositDetail)
			for i, item := range mvrerm.ValidatorList {
				ValidatorElectMap[string(item.NodeID[:])] = item
				//todo: panic
				if item.Deposit == nil {
					mvrerm.ValidatorList[i].Deposit = big.NewInt(50000)
				}
				if item.WithdrawH == nil {
					mvrerm.ValidatorList[i].WithdrawH = big.NewInt(0)
				}
				if item.OnlineTime == nil {
					mvrerm.ValidatorList[i].OnlineTime = big.NewInt(300)
				}
			}

			ValidatorEleRs := new(mc.MasterValidatorReElectionRsq)
			ValidatorEleRs.SeqNum = mvrerm.SeqNum

			var a, b, c []Strallyint
			var value []Stf
			if len(mvrerm.FoundationValidatoeList) == 0 {
				value = CalcAllValueFunction(mvrerm.ValidatorList)
				a, b, c = Ele.Engine(value, mvrerm.RandSeed.Int64(), common.MasterValidatorNum, common.BackupValidatorNum, 0) //mvrerm.RandSeed.Int64(), 11, 5, 0) //0x12217)
			} else {
				value = CalcAllValueFunction(mvrerm.ValidatorList)
				valuefound := CalcAllValueFunction(mvrerm.FoundationValidatoeList)
				a, b, c = Ele.Engine(value, mvrerm.RandSeed.Int64(), common.MasterValidatorNum, common.BackupValidatorNum, len(mvrerm.FoundationValidatoeList)) //0x12217)
				a = Ele.CommbineFundNodesAndPricipal(value, valuefound, a, 0.25, 4.0)
			}

			for index, item := range a {
				tmp := ValidatorElectMap[item.Nodeid]
				var ToG mc.TopologyNodeInfo
				ToG.Account = tmp.Address
				ToG.Position = uint16(index)
				ToG.Type = common.RoleValidator
				ToG.Stock = uint16(item.Value)
				ValidatorEleRs.MasterValidator = append(ValidatorEleRs.MasterValidator, ToG)
			}

			for index, item := range b {
				tmp := ValidatorElectMap[item.Nodeid]
				var ToG mc.TopologyNodeInfo
				ToG.Account = tmp.Address
				ToG.Position = uint16(index)
				ToG.Type = common.RoleValidator
				ToG.Stock = uint16(item.Value)
				ValidatorEleRs.BackUpValidator = append(ValidatorEleRs.BackUpValidator, ToG)
			}

			for index, item := range c {
				tmp := ValidatorElectMap[item.Nodeid]
				var ToG mc.TopologyNodeInfo
				ToG.Account = tmp.Address

				ToG.Position = uint16(index)
				ToG.Type = common.RoleValidator
				ToG.Stock = uint16(item.Value)
				ValidatorEleRs.CandidateValidator = append(ValidatorEleRs.CandidateValidator, ToG)
			}
			Ele.EleMVRs <- ValidatorEleRs
			//			Ele.EleMVRs <- fmt.Println(value)
			//	fmt.Println("受到数据", mvrerm)
		}
	}
}

func locate(addr common.Address, top *mc.TopologyGraph) int {
	for _, v := range top.NodeList {
		if v.Account != addr {
			continue
		}
		if v.Type == common.RoleValidator {
			return 0
		}
		if v.Type == common.RoleBackupValidator {
			return 1
		}
		return -1
	}
	return -1

}

func SloveZeroOffline(pos uint16, alter []mc.Alternative, native AllNative, top *mc.TopologyGraph, flag int) ([]mc.Alternative, AllNative) {

	for k, v := range native.MasterQ { //0级缓存
		temp := mc.Alternative{
			A:        v,
			Position: pos,
		}
		alter = append(alter, temp)
		native.MasterQ = append(native.MasterQ[:k], native.MasterQ[k+1:]...)
		log.INFO("选举计算阶段-0", "当前掉线的节点是", pos, "用0即缓存里的节点去顶替 缓存的节点", v.String())
		return alter, native
	}

	for _, v := range top.NodeList { //1级在线
		if v.Type == common.RoleBackupValidator {
			temp := mc.Alternative{
				A:        v.Account,
				Position: pos,
			}
			alter = append(alter, temp)
			v.Type = common.RoleNil
			log.INFO("选举计算节点-0", "当前掉线的节点是", pos, "用一级在线的点去顶替", v.Account.String())
			for kB, vB := range native.BackUpQ { //1级缓存
				temp := mc.Alternative{
					A:        vB,
					Position: v.Position,
				}
				alter = append(alter, temp)
				native.BackUpQ = append(native.BackUpQ[:kB], native.BackUpQ[kB+1:]...)
				log.INFO("选举计算节点-0", "当前掉线的节点是", pos, "用一级在线的点去顶替", v.Account.String(), "一级缓存里有缓存 顶替一级在线的节点", vB.String())
				return alter, native
			}
			for kC, vC := range native.CandidateQ { //2级缓存
				temp := mc.Alternative{
					A:        vC,
					Position: v.Position,
				}
				alter = append(alter, temp)
				native.CandidateQ = append(native.CandidateQ[:kC], native.CandidateQ[kC+1:]...)
				log.INFO("选举计算节点-0", "当前掉线的节点是", pos, "用一级在线的点去顶替", v.Account.String(), "二级缓存里有缓存 顶替一级在线的节点", vC.String())
				return alter, native
			}
			temp = mc.Alternative{
				A: common.Address{},

				Position: v.Position,
			}
			alter = append(alter, temp)
			log.INFO("选举计算节点-0", "当前掉线的节点是", pos, "用一级在线的点去顶替", v.Account.String(), "无缓存可顶 原一级在线的顶点的位置直接删除", "")
			return alter, native
		}
	}
	for k, v := range native.BackUpQ { //1级缓存
		temp := mc.Alternative{
			A: v,

			Position: pos,
		}
		alter = append(alter, temp)
		native.BackUpQ = append(native.BackUpQ[:k], native.BackUpQ[k+1:]...)
		log.INFO("选举计算节点-0", "当前掉线节点是", pos, "用一级缓存的点去顶替", v.String())
		return alter, native

	}
	for k, v := range native.CandidateQ { //2级缓存
		temp := mc.Alternative{
			A:        v,
			Position: pos,
		}
		alter = append(alter, temp)
		native.CandidateQ = append(native.CandidateQ[:k], native.CandidateQ[k+1:]...)
		log.INFO("选举计算节点-0", "当前掉线节点是", pos, "用二级缓存的点去顶替", v.String())
		return alter, native
	}
	//该位置无人可补充
	temp := mc.Alternative{
		A: common.Address{},

		Position: pos,
	}
	if flag == IsOffline {
		alter = append(alter, temp)
	}

	log.INFO("选举计算节点-0", "当前掉线节点是", pos, "无候选节点可顶替 直接删除该位置", pos)
	return alter, native

}
func SloveFirstOffline(pos uint16, alter []mc.Alternative, native AllNative, flag int) ([]mc.Alternative, AllNative) {
	for k, v := range native.BackUpQ { //1级缓存
		temp := mc.Alternative{
			A: v,

			Position: pos,
		}
		alter = append(alter, temp)
		native.BackUpQ = append(native.BackUpQ[:k], native.BackUpQ[k+1:]...)
		log.INFO("选举计算节点-1", "当前掉线节点是", pos, "用一级缓存的点去顶替", v.String())
		return alter, native
	}
	for k, v := range native.CandidateQ { //2级缓存
		temp := mc.Alternative{
			A: v,

			Position: pos,
		}
		alter = append(alter, temp)
		native.CandidateQ = append(native.CandidateQ[:k], native.CandidateQ[k+1:]...)
		log.INFO("选举计算节点-1", "当前掉线节点是", pos, "用二级缓存的点去顶替", v.String())
		return alter, native
	}
	temp := mc.Alternative{
		A: common.Address{},

		Position: pos,
	}
	log.INFO("选举计算节点-1", "当前掉线节点是", pos, "无节点可顶替 直接删除该位置", "")
	if flag == IsOffline {
		alter = append(alter, temp)
	}

	return alter, native
}

func findAddress(addr common.Address, aim []common.Address) bool {
	for _, v := range aim {
		if v == addr {
			return true
		}
	}
	return false
}

const (
	IsOffline  = 1
	NotOffline = 0
)

func (Ele *Elector) ToPoUpdate(offline []common.Address, allNative AllNative, top *mc.TopologyGraph) []mc.Alternative {
	ans := []mc.Alternative{}

	mapMaster := make(map[uint16]common.Address)
	mapBackup := make(map[uint16]common.Address)
	for _, v := range top.NodeList {
		types := common.GetRoleTypeFromPosition(v.Position)
		if types == common.RoleValidator {
			mapMaster[v.Position] = v.Account
		}
		if types == common.RoleBackupValidator {
			mapBackup[v.Position] = v.Account
		}
	}

	for index := 0; index < common.MasterValidatorNum; index++ {
		k := common.GeneratePosition(uint16(index), common.ElectRoleValidator)
		_, ok := mapMaster[k]
		if ok == false {
			ans, allNative = SloveZeroOffline(k, ans, allNative, top, NotOffline)
			continue
		}
		if findAddress(mapMaster[k], offline) == true {
			ans, allNative = SloveZeroOffline(k, ans, allNative, top, IsOffline)
			continue
		}

	}
	for index := 0; index < common.BackupValidatorNum; index++ {
		k := common.GeneratePosition(uint16(index), common.ElectRoleValidatorBackUp)
		_, ok := mapBackup[k]
		if ok == false {
			ans, allNative = SloveFirstOffline(k, ans, allNative, NotOffline)
			continue
		}
		if findAddress(mapBackup[k], offline) == true {
			ans, allNative = SloveFirstOffline(k, ans, allNative, IsOffline)
			continue
		}

	}
	return ans
}

func (Ele *Elector) PrimarylistUpdate(Q0, Q1, Q2 []mc.TopologyNodeInfo, online mc.TopologyNodeInfo, flag int) ([]mc.TopologyNodeInfo, []mc.TopologyNodeInfo, []mc.TopologyNodeInfo) {
	log.Info("Elector PrimarylistUpdate")
	if flag == 0 {
		var tQ0 []mc.TopologyNodeInfo
		tQ0 = append(tQ0, online)
		tQ0 = append(tQ0, Q0...)
		Q0 = tQ0
	}

	if flag == 1 {
		var tQ1 []mc.TopologyNodeInfo
		tQ1 = append(tQ1, Q1...)
		tQ1 = append(tQ1, online)
		Q1 = tQ1
	}

	if flag == 2 {
		var tQ2 []mc.TopologyNodeInfo
		tQ2 = append(tQ2, Q2...)
		tQ2 = append(tQ2, online)
		Q2 = tQ2
	}
	return Q0, Q1, Q2
}
