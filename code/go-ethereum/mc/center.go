package mc

import (
	"errors"

	"github.com/ethereum/go-ethereum/event"
)

type Center struct {
	FeedMap map[EventCode]*event.Feed
}

var (
	local = newCenter()

	SubErrorNoThisEvent  = errors.New("SubscribeEvent Failed No This Event")
	PostErrorNoThisEvent = errors.New("PostEvent Failed No This Event")
)

func newCenter() *Center {
	msgCenter := &Center{FeedMap: make(map[EventCode]*event.Feed)}
	msgCenter.init()
	return msgCenter
}

func (c *Center) init() {
	for i := 0; i < int(LastEventCode); i++ {
		c.FeedMap[EventCode(i)] = new(event.Feed)
	}
}

func SubscribeEvent(aim EventCode, ch interface{}) (event.Subscription, error) {
	feed, ok := local.FeedMap[aim]
	if !ok {
		return nil, SubErrorNoThisEvent
	}
	return feed.Subscribe(ch), nil
}

func PublishEvent(aim EventCode, data interface{}) error {
	feed, ok := local.FeedMap[aim]
	if !ok {
		return PostErrorNoThisEvent
	}
	go feed.Send(data)
	return nil
}
