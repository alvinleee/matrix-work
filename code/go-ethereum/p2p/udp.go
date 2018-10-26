package p2p

import (
	"net"

	"github.com/ethereum/go-ethereum/ca"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/mc"
	"github.com/ethereum/go-ethereum/p2p/discover"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
)

func UdpStart() {
	addr, err := net.ResolveUDPAddr("udp", ":30000")
	if err != nil {
		log.Error("Can't resolve address: ", "p2p udp", err)
		return
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Error("Error listening:", "p2p udp", err)
		return
	}
	defer conn.Close()

	buf := make([]byte, params.MaxUdpBuf)

	for {
		var mxtxs []*types.Transaction_Mx
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Error("UDP read error", "err", err)
			return
		}

		err = rlp.DecodeBytes(buf[:n], &mxtxs)
		if err != nil {
			log.Error("rlp decode error", "err", err)
			continue
		}
		mc.PublishEvent(mc.SendUdpTx, mxtxs)
	}
}

func UdpSend(data interface{}) {
	bytes, err := rlp.EncodeToBytes(data)
	if err != nil {
		log.Error("error", "p2p udp", err)
		return
	}

	ids := make([]discover.NodeID, 0)
	if ca.InDuration() {
		ids = ca.GetRolesByGroupOnlyNextElect(common.RoleValidator | common.RoleBackupValidator)
	} else {
		ids = ca.GetRolesByGroup(common.RoleValidator | common.RoleBackupValidator)
	}
	if len(ids) <= 2 {
		for _, id := range ids {
			send(id, bytes)
		}
		return
	}

	is := Random(len(ids), 2)
	for _, i := range is {
		send(ids[i], bytes)
	}
}

func send(id discover.NodeID, data []byte) {
	node := ServerP2p.ntab.Resolve(id)
	if node == nil {
		log.Error("buckets nodes", "p2p", id)
		return
	}

	addr, err := net.ResolveUDPAddr("udp", node.IP.String()+":30000")
	if err != nil {
		log.Error("Can't resolve address: ", "p2p udp", err)
		return
	}
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		log.Error("Can't dial: ", "p2p udp", err)
		return
	}
	defer conn.Close()

	_, err = conn.Write(data)
	if err != nil {
		log.Error("failed:", "p2p udp", err)
		return
	}
}
