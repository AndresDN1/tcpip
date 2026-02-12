package arp

import (
	"encoding/binary"
	"tcp-ip/internal/ethernet"
	"tcp-ip/internal/ip"
	"tcp-ip/internal/nic"
	"time"
)

func (arp *ARPModule) sendARP(ip ip.IPAddress, mac nic.MACAddress, op uint16) error {
	packet := &ARPPacket{
		HardwareType:          arp.hrd,
		ProtocolType:          arp.proto,
		HardwareLength:        arp.hrdLen,
		ProtocolLength:        arp.protoLen,
		Operation:             op,
		SenderHardwareAddress: arp.hrdAddr,
		SenderProtocolAddress: binary.BigEndian.Uint32(arp.protoAddr[:]),
		TargetHardwareAddress: mac,
		TargetProtocolAddress: binary.BigEndian.Uint32(ip[:]),
	}
	data := packet.Serialize()

	err := arp.sender.SendToMAC(data[:], mac, ethernet.ARPEtherType)
	return err
}

func (arp *ARPModule) SendGARP() (<-chan struct{}, error) {
	arp.mutex.Lock()
	if ch, exists := arp.pending[arp.protoAddr]; exists {
		arp.mutex.Unlock()
		return ch, nil
	}
	ch := make(chan struct{})
	arp.pending[arp.protoAddr] = ch
	arp.mutex.Unlock()

	err := arp.sendARP(arp.protoAddr, ethernet.BroadcastAddress, OpRequest)
	if err != nil {
		arp.mutex.Lock()
		defer arp.mutex.Unlock()
		close(ch)
		delete(arp.pending, arp.protoAddr)
		return nil, err
	}

	return ch, nil
}

func (arp *ARPModule) sendResponse(ip ip.IPAddress, mac nic.MACAddress) error {
	return arp.sendARP(ip, mac, OpResponse)
}

func (arp *ARPModule) sendRequest(ip ip.IPAddress, mac nic.MACAddress) (<-chan struct{}, error) {
	arp.mutex.Lock()
	if ch, exists := arp.pending[ip]; exists {
		arp.mutex.Unlock()
		return ch, nil
	}

	ch := make(chan struct{})
	arp.pending[ip] = ch
	arp.table[ip] = newARPEntry(nic.MACAddress{}, StatePending)
	arp.table[ip].lastAttempted = time.Now()
	arp.mutex.Unlock()

	err := arp.sendARP(ip, mac, OpRequest)
	if err != nil {
		arp.mutex.Lock()
		defer arp.mutex.Unlock()
		close(arp.pending[ip])
		delete(arp.pending, ip)
		arp.table[ip].state = StateFailed
		return nil, err
	}

	return ch, nil
}
