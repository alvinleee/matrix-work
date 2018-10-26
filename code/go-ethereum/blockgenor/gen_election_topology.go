package blockgenor

import (
	"github.com/ethereum/go-ethereum/ca"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/mc"
	"github.com/ethereum/go-ethereum/topnode"
)

func (p *Process) genElection(num uint64) []common.Elect {
	info, err := p.reElection().GetElection(num)
	if err != nil {
		log.Warn(p.logExtraInfo(), "verifyElection: get election err", err)
		return nil
	}

	return p.reElection().TransferToElectionStu(info)
}

func (p *Process) getNetTopology(currentNetTopology common.NetTopology, num uint64) *common.NetTopology {
	if common.IsReElectionNumber(num + 1) {
		return p.genAllNetTopology(num)
	}

	return p.genChgNetTopology(currentNetTopology, num)
}

func (p *Process) genAllNetTopology(num uint64) *common.NetTopology {
	info, err := p.reElection().GetNetTopologyAll(num)
	if err != nil {
		log.Warn(p.logExtraInfo(), "verifyNetTopology: get prev topology from ca err", err)
		return nil
	}

	return p.reElection().TransferToNetTopologyAllStu(info)
}

func (p *Process) getPrevTopology(num uint64) (*mc.TopologyGraph, error) {
	reqRoles := common.RoleType(common.RoleValidator | common.RoleBackupValidator | common.RoleMiner | common.RoleBackupMiner)

	return ca.GetTopologyByNumber(reqRoles, num-1)
}

func (p *Process) getOfflineTopState(primaryNodeStateMap map[common.Address]topnode.OnlineState, currentNodeStateMap map[common.Address]topnode.OnlineState) ([]common.Address, []common.Address, []common.Address) {

	offlineTopNodes := make([]common.Address, 0)
	onlinePrimaryNods := make([]common.Address, 0)
	offlinePrimaryNodes := make([]common.Address, 0)
	for key, value := range primaryNodeStateMap {
		if value == topnode.Online {
			onlinePrimaryNods = append(onlinePrimaryNods, key)
		} else {
			offlinePrimaryNodes = append(offlinePrimaryNodes, key)
		}
	}

	for key, value := range currentNodeStateMap {
		if value == topnode.Offline {
			offlineTopNodes = append(offlineTopNodes, key)
		}
	}
	return offlineTopNodes, onlinePrimaryNods, offlinePrimaryNodes
}

func (p *Process) genChgNetTopology(currentNetTopology common.NetTopology, num uint64) *common.NetTopology {

	// get local consensus on-line state
	var eleNum uint64
	if p.number < common.GetReElectionInterval() {
		eleNum = 0
	} else {
		eleNum = common.GetLastReElectionNumber(p.number) - 1
	}
	originTopology, err := ca.GetTopologyByNumber(common.RoleValidator|common.RoleBackupValidator|common.RoleMiner|common.RoleBackupMiner, eleNum)
	if err != nil {
		log.Warn(p.logExtraInfo(), "get topology by number error", err)
		return nil
	}
	originTopNodes := make([]common.Address, 0)
	for _, node := range originTopology.NodeList {
		originTopNodes = append(originTopNodes, node.Account)
	}

	p.pm.topNode.SetElectNodes(originTopNodes, eleNum)
	var currentNum uint64

	if p.number < 1 {
		currentNum = 0
	} else {
		currentNum = p.number - 1
	}
	// get prev topology
	currentTopology, err := ca.GetTopologyByNumber(common.RoleValidator|common.RoleBackupValidator|common.RoleMiner|common.RoleBackupMiner, currentNum)
	if err != nil {
		log.Warn(p.logExtraInfo(), "get topology by number error", err)
		return nil
	}

	// get online and offline info from header and prev topology
	onlineTopNodes := make([]common.Address, 0)
	for _, node := range currentTopology.NodeList {
		onlineTopNodes = append(onlineTopNodes, node.Account)
		log.Info(p.logExtraInfo(), "onlineTopNodes", node.Account)

	}

	onlineElectNodes := make([]common.Address, 0)
	for _, node := range currentTopology.ElectList {
		onlineElectNodes = append(onlineElectNodes, node.Account)
		log.Info(p.logExtraInfo(), "onlineElectNodes", node.Account)

	}
	log.Info(p.logExtraInfo(), "SetCurrentOnlineState:高度", p.number, "onlineElect", len(onlineElectNodes), "onlineTopnode", len(onlineTopNodes))

	p.pm.topNode.SetCurrentOnlineState(onlineTopNodes, onlineElectNodes)
	offlineTopNodes, onlineElectNods, offlineElectNodes := p.pm.topNode.GetConsensusOnlineState()

	for _, value := range offlineTopNodes {
		log.Info(p.logExtraInfo(), "offlineTopNodes", value.String())
	}
	for _, value := range onlineElectNods {
		log.Info(p.logExtraInfo(), "onlineElectNods", value.String())
	}

	for _, value := range offlineElectNodes {
		log.Info(p.logExtraInfo(), "offlineElectNodes", value.String())
	}

	for k, v := range offlineElectNodes {
		flag := 0
		for _, vv := range offlineTopNodes {
			if v == vv {
				flag = 1
			}
		}
		if flag == 1 {
			offlineElectNodes = append(offlineElectNodes[:k], offlineElectNodes[k+1:]...)
		}
	}

	log.INFO("scffffff-Gen-GetTopoChange start ", "num", num, "onlineElectNods", onlineElectNods, "offlineTopNodes", offlineTopNodes)
	// generate topology alter info
	alterInfo, err := p.reElection().GetTopoChange(num, offlineTopNodes)
	log.INFO("scffffff-Gen-GetTopoChange end", "alterInfo", alterInfo, "err", err)
	if err != nil {
		log.Warn(p.logExtraInfo(), "get topology change info by reelection server err", err)
		return nil
	}
	for _, value := range alterInfo {
		log.Info(p.logExtraInfo(), "alter-A", value.A, "alter-B", "position", value.Position)
	}

	// generate self net topology
	ans := p.reElection().TransferToNetTopologyChgStu(alterInfo, onlineElectNods, offlineElectNodes)
	log.INFO("scfffff-TransferToNetTopologyChgStu", "ans", ans)
	return ans
}

func (p *Process) parseOnlineState(header *types.Header, prevTopology *mc.TopologyGraph) ([]common.Address, []common.Address, []common.Address) {
	offlineTopNodes := p.reElection().ParseTopNodeOffline(header.NetTopology, prevTopology)
	for _, value := range header.NetTopology.NetTopologyData {
		log.Info("topnodeOnline", "header-netTopo", value.Account.String(), "高度", header.Number.Int64())

	}
	onlinePrimaryNods, offlinePrimaryNodes := p.reElection().ParsePrimaryTopNodeState(header.NetTopology)
	return offlineTopNodes, onlinePrimaryNods, offlinePrimaryNodes
}
