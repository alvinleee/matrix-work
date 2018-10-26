package reelection

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/mc"

	"github.com/ethereum/go-ethereum/election"
	"github.com/ethereum/go-ethereum/params/man"
)

//身份变更消息带来
func (self *ReElection) roleUpdateProcess(data *mc.RoleUpdatedMsg) error {
	self.lock.Lock()
	defer self.lock.Unlock()
	self.currentID = data.Role

	log.INFO("data差值", "高度", self.bc.GetBlockByNumber(data.BlockNum).Header().Number.Uint64(), "data", self.bc.GetBlockByNumber(data.BlockNum).Header().NetTopology)
	if common.RoleValidator != self.currentID { //不是验证者，不处理
		log.ERROR(Module, "处理身份变更阶段 当前不是验证者 不处理", self.currentID)
		return nil
	}

	err := self.HandleTopGen(data.BlockNum) //处理拓扑生成
	if err != nil {
		log.ERROR(Module, "处理身份变更阶段 处理拓扑生成失败 err", err)
		return err
	}

	err = self.HandleNative(data.BlockNum) //处理初选列表更新
	if err != nil {
		log.ERROR(Module, "處理初選列表更新失敗 err", err)
		return err
	}
	log.INFO(Module, "处理身份变更阶段 结束(正常) 高度", data.BlockNum)
	return nil

}
func (self *ReElection) HandleNative(height uint64) error {
	/*
		if true == IsinFristPeriod(height) { //第一选举周期不更新 0-58
			log.INFO(Module, "BlockNum", height, "no need to update native list", "nil")
			return nil
		}
	*/
	if true == NeedReadTopoFromDB(height) { //299 599 899 重取缓存
		err := self.GetNativeFromDB(height + 1)
		if err == nil {
			log.INFO(Module, "从db拿Native数据 height", height+1)
			return nil
		}

		err = self.ToGenMinerTop(height + 1 - man.MinerTopologyGenerateUpTime)
		if err != nil {
			log.Error(Module, "当前db没有矿工选举信息，重新生成 当前高度", height, "计算高度", height+1-man.MinerTopologyGenerateUpTime)
			return err
		}
		err = self.ToGenValidatorTop(height + 1 - man.VerifyTopologyGenerateUpTime)
		if err != nil {
			log.Error(Module, "当前db没有验证者选举信息，重新生成 当前高度", height, "计算高度", height+1-man.VerifyTopologyGenerateUpTime)
			return err
		}
		err = self.GetNativeFromDB(height + 1)
		if err != nil {
			log.ERROR(Module, "重新计算后获取仍失败 err", err)
			return err
		}
		log.INFO(Module, "重新计算拓扑图后的错误信息 err", err)
		return err
	}

	allNative, err := self.readNativeData(height - 1) //
	if err != nil {
		log.Error(Module, "readNativeData failed height", height-1)
	}

	log.INFO(Module, "self,allNative", allNative)

	err = self.UpdateNative(height, allNative)
	log.INFO(Module, "更新初选列表结束 高度 ", height, "错误信息", err)
	return err
}
func (self *ReElection) HandleTopGen(height uint64) error {
	var err error

	if IsMinerTopGenTiming(height) { //矿工生成时间 240
		log.INFO(Module, "是矿工拓扑生成时间点 height", height)
		err = self.ToGenMinerTop(height)
		if err != nil {
			log.ERROR(Module, "矿工拓扑生成出错 err", err)
		}
	}

	if IsValidatorTopGenTiming(height) { //验证者生成时间 260
		log.INFO(Module, "是眼睁睁拓扑生成时间点 height", height)
		err = self.ToGenValidatorTop(height)
		if err != nil {
			log.ERROR(Module, "验证者拓扑生成出错 err", err)
		}
	}

	return err
}

func (self *ReElection) UpdateNative(height uint64, allNative election.AllNative) error {

	allNative, err := self.ToNativeValidatorStateUpdate(height, allNative)
	if err != nil {
		log.INFO(Module, "ToNativeMinerStateUpdate validator err", err)
		return nil
	}

	err = self.writeNativeData(height, allNative)

	log.ERROR(Module, "更新初选列表状态后-写入数据库状态 err", err, "高度", height)

	return err

}

//是不是矿工拓扑生成时间段
func IsMinerTopGenTiming(height uint64) bool {

	now := height % common.GetReElectionInterval()
	if now == MinerTopGenTiming {
		return true
	}
	return false
}

//是不是验证者拓扑生成时间段
func IsValidatorTopGenTiming(height uint64) bool {

	now := height % common.GetReElectionInterval()
	if now == ValidatorTopGenTiming {
		return true
	}
	return false
}

func NeedReadTopoFromDB(height uint64) bool {
	if (height+1)%common.GetReElectionInterval() == 0 || height == 0 {
		return true
	}
	return false
}

func IsinFristPeriod(height uint64) bool {
	if height < common.GetReElectionInterval()-1 {
		return true
	}
	return false
}
