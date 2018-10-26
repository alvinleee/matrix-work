package verifier

import (
	"github.com/ethereum/go-ethereum/log"
	"testing"
	"time"
)

func TestTimer(t *testing.T) {
	log.InitLog(3)

	recvCh := make(chan struct{})
	go TimerRunning(t, recvCh)

	//time.Sleep(7 * time.Second)
	recvCh <- struct{}{}
	time.Sleep(11111 * time.Second)
}

func TimerRunning(t *testing.T, recv chan struct{}) {
	timer := time.NewTimer(10 * time.Second)
	log.Info("开始定时器")
	for {
		select {
		case <-recv:
			log.Info("收到停止消息")
			time.Sleep(12 * time.Second)
			log.Info("停止定时器")
			result := timer.Reset(10 * time.Second)
			log.Info("重置定时器", "结果", result)
		case <-timer.C:
			log.Info("超时了!!!")
		}
	}
}
