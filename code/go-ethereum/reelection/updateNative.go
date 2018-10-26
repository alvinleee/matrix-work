package reelection

import (
	"encoding/json"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/election"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/mc"
	"github.com/ethereum/go-ethereum/params/man"
)

func locate(address common.Address, master []mc.TopologyNodeInfo, backUp []mc.TopologyNodeInfo, cand []mc.TopologyNodeInfo) (int, mc.TopologyNodeInfo) {
	for _, v := range master {
		if v.Account == address {
			return 0, v
		}
	}
	for _, v := range backUp {
		if v.Account == address {
			return 1, v
		}
	}
	for _, v := range cand {
		if v.Account == address {
			return 2, v
		}
	}
	return -1, mc.TopologyNodeInfo{}
}

func (self *ReElection) whereIsV(address common.Address, role common.RoleType, height uint64) (int, mc.TopologyNodeInfo, error) {
	switch {
	case role == common.RoleMiner:
		height = height / common.GetBroadcastInterval()
		height = height*common.GetBroadcastInterval() - man.MinerTopologyGenerateUpTime
		ans, _, err := self.readElectData(common.RoleMiner, height)
		if err != nil {
			return -1, mc.TopologyNodeInfo{}, err
		}
		flag, aimOnline := locate(address, ans.MasterMiner, ans.BackUpMiner, []mc.TopologyNodeInfo{})
		return flag, aimOnline, nil

	case role == common.RoleValidator:
		height = height / common.GetBroadcastInterval()
		height = height*common.GetBroadcastInterval() - man.VerifyTopologyGenerateUpTime
		_, ans, err := self.readElectData(common.RoleValidator, height)
		if err != nil {
			return -1, mc.TopologyNodeInfo{}, err
		}
		flag, aimOnline := locate(address, ans.MasterValidator, ans.BackUpValidator, ans.CandidateValidator)
		return flag, aimOnline, nil
	default:
		log.ERROR(Module, "whereIsV ", "role must be role or validatoe")
		return -1, mc.TopologyNodeInfo{}, errors.New("whereIsV role must be role or validatoe")
	}
}

/*
func (self *ReElection) ToNativeMinerStateUpdate(height uint64, allNative AllNative) (AllNative, error) {
	DiffFromBlock := self.bc.GetHeaderByNumber(height).NetTopology
	//测试
	//DiffFromBlock := common.NetTopology{}
	//aim := 0x04 + 0x08
	TopoGrap, err := GetCurrentTopology(height-1, common.RoleMiner)
	if err != nil {
		log.ERROR(Module, "从CA获取验证者拓扑图错误 err", err)
		return allNative, err
	}
	online, offline := self.CalOnline(DiffFromBlock, TopoGrap)
	log.INFO(Module, "ToNativeMinerStateUpdate online", online, "offline", offline)
	allNative.MasterMiner, allNative.BackUpMiner = deleteOfflineNode(offline, allNative.MasterMiner, allNative.BackUpMiner)

	for _, v := range online {
		flag, aimonline, err := self.whereIsV(v, common.RoleMiner, height)
		if err != nil {
			return AllNative{}, err
		}
		if flag == -1 {
			continue
		}
		allNative.MasterMiner, allNative.BackUpMiner, _ = self.NativeUpdate(allNative.MasterMiner, allNative.BackUpMiner, []mc.TopologyNodeInfo{}, aimonline, flag)
	}

	return allNative, nil
}
*/
func (self *ReElection) ToNativeValidatorStateUpdate(height uint64, allNative election.AllNative) (election.AllNative, error) {

	header := self.bc.GetHeaderByNumber(height)
	if header == nil {
		log.ERROR(Module, "获取指定高度的区块头失败 高度", height)
		return election.AllNative{}, errors.New("获取指定高度的区块头失败")
	}
	DiffFromBlock := header.NetTopology
	log.INFO(Module, "更新初选列表信息 差值的高度", height, "差值", DiffFromBlock)

	TopoGrap, err := GetCurrentTopology(height-1, common.RoleValidator|common.RoleBackupValidator)
	log.INFO(Module, "更新初选列表信息 拓扑的高度", height-1, "拓扑值", TopoGrap)
	if err != nil {
		log.ERROR(Module, "从ca获取验证者拓扑图失败", err)
		return allNative, err
	}
	log.INFO(Module, "更新上下线状态", "开始", "高度", height)
	allNative = self.CalOnline(DiffFromBlock, TopoGrap, allNative)
	log.INFO(Module, "更新上下线状态", "结束", "高度", height)

	return allNative, nil
}

func deleteOfflineNode(offline []common.Address, Master []mc.TopologyNodeInfo, BackUp []mc.TopologyNodeInfo) ([]mc.TopologyNodeInfo, []mc.TopologyNodeInfo) {
	for k, v := range Master {
		if IsInArray(v.Account, offline) {
			Master = append(Master[:k], Master[k+1:]...)
		}
	}
	for k, v := range BackUp {
		if IsInArray(v.Account, offline) {
			BackUp = append(BackUp[:k], BackUp[k+1:]...)
		}
	}
	return Master, BackUp
}

func deleteQueue(address common.Address, allNative election.AllNative) election.AllNative {
	log.INFO(Module, "在缓存中删除节点阶段-开始 地址", address, "缓存", allNative)
	for k, v := range allNative.MasterQ {
		if v == address {
			allNative.MasterQ = append(allNative.MasterQ[k:], allNative.MasterQ[k+1:]...)
			log.INFO(Module, "在缓存中删除节点阶段-master 地址 ", address, "缓存", allNative)
			return allNative
		}
	}
	for k, v := range allNative.BackUpQ {
		if v == address {
			allNative.BackUpQ = append(allNative.BackUpQ[k:], allNative.BackUpQ[k+1:]...)
			log.INFO(Module, "在缓存中删除节点阶段-backup 地址", address, "缓存", allNative)
			return allNative
		}
	}
	for k, v := range allNative.CandidateQ {
		if v == address {
			allNative.CandidateQ = append(allNative.CandidateQ[k:], allNative.CandidateQ[k+1:]...)
			log.INFO(Module, "在缓存中删除节点阶段-candidate 地址", address, "缓存", allNative)
			return allNative
		}
	}

	log.INFO(Module, "在缓存中删除节点阶段-结束-不再任何一个梯队 地址", address, "缓存", allNative)
	return allNative
}

func addQueue(address common.Address, allNative election.AllNative) election.AllNative {
	log.INFO(Module, "在缓存中增加节点阶段-开始 地址", address, "allNative", allNative)
	for _, v := range allNative.Master {
		if v.Account == address {

			allNative.MasterQ = append(allNative.MasterQ, address)
			log.INFO(Module, "在缓存中增加节点阶段-master 地址", address, "allNative", allNative)
			return allNative
		}
	}
	for _, v := range allNative.BackUp {
		if v.Account == address {
			allNative.BackUpQ = append(allNative.BackUpQ, address)
			log.INFO(Module, "在缓存中增加节点阶段-backup 地址", address, "allNative", allNative)
			return allNative
		}
	}
	for _, v := range allNative.Candidate {
		if v.Account == address {
			allNative.CandidateQ = append(allNative.CandidateQ, address)
			log.INFO(Module, "在缓存中增加节点阶段-candidate 地址", address, "allNative", allNative)
			return allNative
		}
	}
	log.INFO(Module, "在缓存中增加节点阶段-结束 地址-不在任何一个梯队", address, "allNative", allNative)
	return allNative
}
func (self *ReElection) CalOnline(diff common.NetTopology, top *mc.TopologyGraph, allNative election.AllNative) election.AllNative {

	log.INFO(Module, "更新上下线阶段 拓扑差值-开始", diff.NetTopologyData, "allNative", allNative)

	for _, v := range diff.NetTopologyData {
		switch {
		case v.Position == common.PosOffline:
			allNative = deleteQueue(v.Account, allNative)
		case v.Position == common.PosOnline:
			allNative = addQueue(v.Account, allNative)
		case v.Account == common.Address{}:
			continue
		default:
			allNative = deleteQueue(v.Account, allNative)
		}
	}
	log.INFO(Module, "更新上下线阶段 拓扑差值-结束", diff.NetTopologyData, "allNative", allNative)
	return allNative

}
func checkInGraph(top *mc.TopologyGraph, pos uint16) common.Address {
	for _, v := range top.NodeList {
		if v.Position == pos {
			return v.Account
		}
	}
	return common.Address{}
}
func checkInDiff(diff common.NetTopology, add common.Address) bool {
	for _, v := range diff.NetTopologyData {
		if v.Account == add {
			return true
		}
	}
	return false
}
func IsInArray(aimAddress common.Address, offline []common.Address) bool {
	for _, v := range offline {
		if v == aimAddress {
			return true
		}
	}
	return false
}
func (self *ReElection) writeNativeData(height uint64, data election.AllNative) error {
	key := MakeNativeDBKey(height)
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	err = self.ldb.Put([]byte(key), jsonData, nil)
	return err
}

func (self *ReElection) readNativeData(height uint64) (election.AllNative, error) {
	key := MakeNativeDBKey(height)
	ans, err := self.ldb.Get([]byte(key), nil)
	if err != nil {
		return election.AllNative{}, err
	}
	var realAns election.AllNative
	err = json.Unmarshal(ans, &realAns)
	if err != nil {
		return election.AllNative{}, err
	}

	return realAns, nil

}
func MakeNativeDBKey(height uint64) string {
	t := big.NewInt(int64(height))
	ss := t.String() + "---" + "Native"
	return ss
}
func needReadFromGenesis(height uint64) bool {
	if height == 1 {
		return true
	}
	return false
}
func (self *ReElection) GetNativeFromDB(height uint64) error {
	if needReadFromGenesis(height) {
		preBroadcast := election.AllNative{}
		header := self.bc.GetBlockByNumber(height - 1).Header()
		if header == nil {
			return errors.New("第0块区块头拿不到")
		}
		for _, v := range header.NetTopology.NetTopologyData {
			switch common.GetRoleTypeFromPosition(v.Position) {
			case common.RoleValidator:
				temp := mc.TopologyNodeInfo{
					Account:  v.Account,
					Position: v.Position,
					Type:     common.RoleValidator,
				}
				preBroadcast.Master = append(preBroadcast.Master, temp)
			case common.RoleBackupValidator:
				temp := mc.TopologyNodeInfo{
					Account:  v.Account,
					Position: v.Position,
					Type:     common.RoleBackupValidator,
				}
				preBroadcast.BackUp = append(preBroadcast.BackUp, temp)
			}
		}
		log.INFO(Module, "第0块到达处理阶段 更新初选列表", "从0的区块头中获取", "初选列表", preBroadcast)
		err := self.writeNativeData(height-1, preBroadcast)
		log.INFO(Module, "第0块到达处理阶段 更新初选列表", "从0的区块头中获取 写数据到数据库", "err", err)
		return err

	}
	log.INFO(Module, "GetNativeFromDB", height)

	validatorH := height - man.VerifyNetChangeUpTime
	_, validatorElect, err := self.readElectData(common.RoleValidator, validatorH)
	if err != nil {
		return err
	}

	preBroadcast := election.AllNative{
		Master:    validatorElect.MasterValidator,
		BackUp:    validatorElect.BackUpValidator,
		Candidate: validatorElect.CandidateValidator,

		MasterQ:    []common.Address{},
		BackUpQ:    []common.Address{},
		CandidateQ: []common.Address{},
	}

	for _, v := range preBroadcast.Candidate {
		preBroadcast.CandidateQ = append(preBroadcast.CandidateQ, v.Account)
	}

	log.INFO(Module, "GetNativeFromDB", height-1, "ready to writeNativeData data", preBroadcast)
	err = self.writeNativeData(height-1, preBroadcast)
	log.INFO(Module, "writeNativeData", height-1, "err", err)
	return err
}
