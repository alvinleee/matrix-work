package reelection

import (
	"errors"
	"math/big"
	"math/rand"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/ca"
	"github.com/ethereum/go-ethereum/core/vm"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/mc"
	"github.com/ethereum/go-ethereum/p2p/discover"
	"github.com/ethereum/go-ethereum/election"
)

func (self *ReElection) TopoUpdate(offline []common.Address,allNative election.AllNative,top *mc.TopologyGraph) []mc.Alternative {
	return self.elect.ToPoUpdate(offline,allNative,top)
}
func (self *ReElection) NativeUpdate(Q0, Q1, Q2 []mc.TopologyNodeInfo, online mc.TopologyNodeInfo, flag int) ([]mc.TopologyNodeInfo, []mc.TopologyNodeInfo, []mc.TopologyNodeInfo) {
	return self.elect.PrimarylistUpdate(Q0, Q1, Q1, online, flag)
}

//得到特殊交易
func getKeyTransInfo(Height uint64, types string) map[common.Address][]byte {
	//asd()
	ans, err := core.GetBroadcastTxs(big.NewInt(int64(Height)), types)
	if err != nil {
		log.Error(Module, "获取特殊交易失败 Height", Height, "types", types)
	}
	return ans
}

func GetCurrentTopology(height uint64, reqtypes common.RoleType) (*mc.TopologyGraph, error) {

	return ca.GetTopologyByNumber(reqtypes, height)
}

var count int32

func getrand() int {
	var flag bool
	flag = true
	var num int

	atomic.AddInt32(&count, 1)
	q := rand.Intn(15000)
	if flag == true {
		num = 30000 + q
		flag = false
	}
	if flag == false {
		num = 30000 - q
		flag = true
	}

	return num
}

type TestELECT struct {
	Deposit []vm.DepositDetail
	Add     []common.Address
}

var Dminer TestELECT
var Dalidator TestELECT

func Test() {
	Dminer.Deposit, Dminer.Add = GetDepost()
	Dalidator.Deposit, Dalidator.Add = GetDepost()

}

func GetDepost() ([]vm.DepositDetail, []common.Address) {
	var mmreqm mc.MasterMinerReElectionReqMsg
	var item vm.DepositDetail
	//	var str string
	var NodeID discover.NodeID
	var Address common.Address
	mmreqm.SeqNum = 20
	mmreqm.RandSeed = big.NewInt(int64(30))

	for i := 0; i < 64; i++ {
		NodeID[i] = byte(i)
	}

	for k := 0; k < 20; k++ {
		Address[k] = byte(k)
	}
	address_scf := make([]common.Address, 0)
	for j := 0; j < 3; j++ {
		NodeID[63] = byte(int(NodeID[63]) + j)
		Address[19] = byte(int(Address[19]) + j)

		item.Address = Address //common.BytesToAddress([]byte("32f7c8dae96abdae96ab")) //[]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0}
		item.Deposit = big.NewInt(int64(getrand() - j))
		item.NodeID = NodeID //discover.NodeID("discoverNodeIdBytestoaddress") //discover.NodeID.BytesToAddress({1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0})
		item.OnlineTime = big.NewInt(int64(300 - j))
		mmreqm.MinerList = append(mmreqm.MinerList, item)
		address_scf = append(address_scf, item.Address)
	}

	return mmreqm.MinerList, address_scf
}
func GetAllElectedByHeight(Heigh *big.Int, tp common.RoleType) ([]vm.DepositDetail, error) {

	switch tp {
	case common.RoleMiner:
		//todo：获取所有的抵押交易
		ans, err := ca.GetElectedByHeightAndRole(Heigh, common.RoleMiner)
		log.INFO("從CA獲取礦工抵押交易", "data", ans, "height", Heigh)
		if err != nil {
			return []vm.DepositDetail{}, errors.New("获取矿工交易身份不对")
		}
		return ans, nil
	case common.RoleValidator:
		ans, err := ca.GetElectedByHeightAndRole(Heigh, common.RoleValidator)
		log.Info("從CA獲取驗證者抵押交易", "data", ans, "height", Heigh)
		if err != nil {
			return []vm.DepositDetail{}, errors.New("获取验证者交易身份不对")
		}
		return ans, nil

	default:
		return []vm.DepositDetail{}, errors.New("获取抵押交易身份不对")
	}
}

func GetFound() []vm.DepositDetail {
	return []vm.DepositDetail{}
}
