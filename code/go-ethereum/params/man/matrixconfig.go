package man

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/discover"
	"github.com/ethereum/go-ethereum/params"
)

const (
	VerifyNetChangeUpTime = 6
	MinerNetChangeUpTime  = 4

	VerifyTopologyGenerateUpTime = 8
	MinerTopologyGenerateUpTime  = 8

	RandomVoteTime = 5

	LRSWaitingBlockReqTimer   = 40 * time.Second
	LRSWaitingDPOSResultTimer = 40 * time.Second
	LRSReelectLeaderTimer     = 40 * time.Second
	LRSReqTimeOut             = 3 * time.Second
)

var (
	//SignAccount         = "0xc47d9e507c1c5cb65cc7836bb668549fc8f547df"
	//SignAccountPassword = "12345"
	//HcMethod            = HCP2P
	//
	//NoBootNode      = errors.New("无boot节点")
	//NoBroadCastNode = errors.New("无广播节点")

	VotePoolTimeout    = 37 * 1000
	VotePoolCountLimit = 5

	//Difficultlist = []uint64{1, 2, 10, 50}
	Difficultlist = []uint64{1}
)

type NodeInfo struct {
	NodeID  discover.NodeID
	Address common.Address
}

var BroadCastNodes = []NodeInfo{}

func Config_Init(Config_PATH string) {
	log.INFO("Config_Init 函数", "Config_PATH", Config_PATH)

	JsonParse := NewJsonStruct()
	v := Config{}
	JsonParse.Load(Config_PATH, &v)
	params.MainnetBootnodes = v.BootNode
	if len(params.MainnetBootnodes) <= 0 {
		fmt.Println("无bootnode节点")
		os.Exit(-1)
	}
	BroadCastNodes = v.BroadNode
	if len(BroadCastNodes) <= 0 {
		fmt.Println("无广播节点")
		os.Exit(-1)
	}
}

type Config struct {
	BootNode  []string
	BroadNode []NodeInfo
}

type JsonStruct struct {
}

func NewJsonStruct() *JsonStruct {
	return &JsonStruct{}
}

func (jst *JsonStruct) Load(filename string, v interface{}) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		fmt.Println("读取通用配置文件失败 err", err, "file", filename)
		os.Exit(-1)
		return
	}
	err = json.Unmarshal(data, v)
	if err != nil {
		fmt.Println("通用配置文件数据获取失败 err", err)
		os.Exit(-1)
		return
	}
}
