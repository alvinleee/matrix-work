// Copyright 2017 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package ethash

import (
	crand "crypto/rand"
	"math"
	"math/big"
	"math/rand"
	"runtime"
	"sort"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

type diffiList []*big.Int

func (v diffiList) Len() int {
	return len(v)
}

func (v diffiList) Swap(i, j int) {
	v[i], v[j] = v[j], v[i]
}

func (v diffiList) Less(i, j int) bool {
	if v[i].Cmp(v[j]) == 1 {

		return true
	}
	return false
}

type minerDifficultyList struct {
	lock      sync.RWMutex
	diffiList []*big.Int
	targets   []*big.Int
}

func GetdifficultyListAndTargetList(difficultyList []*big.Int) minerDifficultyList {
	difficultyListAndTargetList := minerDifficultyList{
		diffiList: make([]*big.Int, len(difficultyList)),
		targets:   make([]*big.Int, len(difficultyList)),
		lock:      sync.RWMutex{},
	}
	copy(difficultyListAndTargetList.diffiList, difficultyList)
	var targets = make([]*big.Int, len(difficultyList))
	for i := 0; i < len(difficultyList); i++ {
		targets[i] = new(big.Int).Div(maxUint256, difficultyList[i])

	}
	copy(difficultyListAndTargetList.targets, targets)

	return difficultyListAndTargetList
}

// Seal implements consensus.Engine, attempting to find a nonce that satisfies
// the block's difficulty requirements.

func (ethash *Ethash) Seal(chain consensus.ChainReader, header *types.Header, stop <-chan uint64, Result_Send chan<- *consensus.Result, difficultyList []*big.Int, isBroadcastNode bool) error {
	log.Warn("defer", "开始挖矿 高度", header.Number.Uint64())
	defer log.Warn("defer", "开始挖矿-借宿 高度", header.Number.Uint64())

	sort.Sort(diffiList(difficultyList))
	difficultyListAndTargetList := GetdifficultyListAndTargetList(difficultyList)

	// Create a runner and the multiple search threads it directs
	abort := make(chan struct{})
	found := make(chan consensus.FoundMsg)
	ethash.lock.Lock()
	curHeader := types.CopyHeader(header)
	threads := ethash.threads
	if ethash.rand == nil {
		seed, err := crand.Int(crand.Reader, big.NewInt(math.MaxInt64))
		if err != nil {
			ethash.lock.Unlock()
			return err
		}
		ethash.rand = rand.New(rand.NewSource(seed.Int64()))
	}
	ethash.lock.Unlock()

	threads = runtime.NumCPU()
	if isBroadcastNode {
		threads = 1
	}

	var pend sync.WaitGroup
	for i := 0; i < threads; i++ {
		pend.Add(1)
		go func(id int, nonce uint64) {
			defer pend.Done()
			ethash.mine(curHeader, id, nonce, abort, found, &difficultyListAndTargetList)

		}(i, uint64(ethash.rand.Int63()))
	}
	// Wait until sealing is terminated or a nonce is found

	for {
		select {
		case num := <-stop:
			log.INFO("SEALER", "Sealer Recv stop mine,num", num, "curHeader.Number.U", curHeader.Number.Uint64())
			// Outside abort, stop all miner threads
			if num == curHeader.Number.Uint64() {
				log.Warn("SEALER", "是需要停的高度 高度", num)
				close(abort)
				return nil
			}

		case result := <-found:
			log.INFO("SEALER", "recv found msg from mine difficulty", result.Difficulty)

			Result_Send <- &consensus.Result{result.Difficulty, result.Header}
			// One of the threads found a block, abort all others
			//close(abort)
		case <-ethash.update:
			// Thread count was changed on user request, restart
			close(abort)
			pend.Wait()
			return ethash.Seal(chain, curHeader, stop, Result_Send, difficultyList, isBroadcastNode)
		}
	}
	// Wait for all miners to terminate and return the block
	pend.Wait()
	return nil
}

func compareDifflist(result []byte, diffList []*big.Int, targets []*big.Int) (int, bool) {
	for i := 0; i < len(diffList); i++ {
		if new(big.Int).SetBytes(result).Cmp(targets[i]) <= 0 {
			return i, true
		}
	}

	return -1, false
}

// mine is the actual proof-of-work miner that searches for a nonce starting from
// seed that results in correct final block difficulty.
func (ethash *Ethash) mine(header *types.Header, id int, seed uint64, abort chan struct{}, found chan consensus.FoundMsg, diffiList *minerDifficultyList) {
	// Extract some data from the header

	var (
		curHeader     = types.CopyHeader(header)
		hash          = curHeader.HashNoNonce().Bytes()
		number        = curHeader.Number.Uint64()
		dataset       = ethash.dataset(number)
		NowDifficulty *big.Int
	)
	// Start generating random nonces until we abort or find a good one
	var (
		attempts = int64(0)
		nonce    = seed
	)
	logger := log.New("miner", id)
	logger.Trace("Started ethash search for new nonces", "seed", seed)
	//log.INFO("SEALER", "Started ethash search for new nonces seed", seed)
	for {
		select {
		case <-abort:
			// Mining terminated, update stats and abort
			logger.Trace("Ethash nonce search aborted", "attempts", nonce-seed)
			log.INFO("SEALER", "curHeader.number", curHeader.Number.Uint64())
			ethash.hashrate.Mark(attempts)
			return

		default:
			// We don't have to update hash rate on every nonce, so update after after 2^X nonces
			attempts++
			if (attempts % (1 << 15)) == 0 {
				ethash.hashrate.Mark(attempts)
				attempts = 0
			}
			// Compute the PoW value of this nonce
			digest, result := hashimotoFull(nil, hash, nonce)

			//compare difficuty list
			if result == nil {
				continue
			}
			//todo 锁的位置加的有问题
			diffiList.lock.Lock()
			num, ret := compareDifflist(result, diffiList.diffiList, diffiList.targets)

			if ret == true {
				NowDifficulty = diffiList.diffiList[num]
				diffiList.targets = diffiList.targets[:num]
				diffiList.diffiList = diffiList.diffiList[:num]
				diffiList.lock.Unlock()
				// Correct nonce found, create a new header with it
				FoundHeader := types.CopyHeader(curHeader)
				FoundHeader.Nonce = types.EncodeNonce(nonce)
				FoundHeader.MixDigest = common.BytesToHash(digest)
				log.INFO("SEALER", "Send found message to update! NowDifficulty", NowDifficulty, "id", id)

				select {
				case found <- consensus.FoundMsg{Header: FoundHeader, Difficulty: NowDifficulty}:
					logger.Trace("Ethash nonce found and reported", "attempts", nonce-seed, "nonce", nonce)
				case <-abort:
					logger.Trace("Ethash nonce found but discarded", "attempts", nonce-seed, "nonce", nonce)
				}

				//log.INFO("-----","NowDifficulty",NowDifficulty,"curheader.Difficulty",curHeader.Difficulty,"curheader.Number",curHeader.Number.Uint64())
				if NowDifficulty.Uint64() == curHeader.Difficulty.Uint64() {
					log.INFO("NowDifficulty == curHeader.Difficulty", "quit minning", "", "id", id, "高度", curHeader.Number.Uint64())
					return
				}

			} else {
				diffiList.lock.Unlock()
			}

			nonce++
		}
	}
	// Datasets are unmapped in a finalizer. Ensure that the dataset stays live
	// during sealing so it's not unmapped while being read.
	runtime.KeepAlive(dataset)
}
