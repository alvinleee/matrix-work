package reelection

import (
	"testing"

	"fmt"

	"github.com/ethereum/go-ethereum/common"
)

//func Post() {
//	blockNum := 20
//	for {
//
//		err := mc.PostEvent("CA_RoleUpdated", mc.RoleUpdatedMsg{Role: common.RoleValidator, BlockNum: uint64(blockNum)})
//		blockNum++
//		//fmt.Println("CA_RoleUpdated", mc.RoleUpdatedMsg{Role: common.RoleValidator, BlockNum: uint64(blockNum)})
//		log.Info("err", err)
//		time.Sleep(5 * time.Second)
//
//	}
//}
//
//func TestReElect(t *testing.T) {
//
//	electseed, err := random.NewElectionSeed()
//
//	log.Info("electseed", electseed)
//	log.Info("seed err", err)
//
//	var eth *eth.Ethereum
//	reElect, err := New(eth)
//	log.Info("err", err)
//
//	go Post()
//
//	time.Sleep(10000 * time.Second)
//	time.Sleep(3 * time.Second)
//	ans1, ans2, ans3 := reElect.readElectData(common.RoleMiner, 240)
//	fmt.Println("READ ELECT", ans1, ans2, ans3)
//	fmt.Println("READ ELECT", 240)
//
//	fmt.Println(reElect)
//}
type A struct {
	AA   int
	MMPP map[string]int
}

func TestT(t *testing.T) {
	a := A{
		AA: 1,
	}
	a.MMPP = make(map[string]int, 0)
	a.MMPP["1"] = 1
	fmt.Println(a)
	b := a.MMPP
	fmt.Println(b)

}

func TestUU(t *testing.T) {
	k := common.GeneratePosition(uint16(0), common.ElectRoleValidatorBackUp)
	fmt.Println(k)

}
