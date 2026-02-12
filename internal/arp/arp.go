package arp

import (
	"fmt"
	"sync"
	"tcp-ip/internal/ethernet"
	"tcp-ip/internal/ip"
	"tcp-ip/internal/nic"
	"time"
)

var (
	ErrIPConflict         = fmt.Errorf("the IP is already occupied")
	ErrMaxDefensesReached = fmt.Errorf("the IP conflict has reached the max allowed defenses")
)

const (
	HeaderSize            = 28
	HrdEthernet    uint16 = 1
	ProtoIPv4      uint16 = 0x800
	HrdLenEthernet uint8  = 6
	ProtoLenIpv4   uint8  = 4

	retryAttempts  = 3
	retryInterval  = time.Millisecond * 250
	probeAttempts  = 3
	probeWait      = time.Millisecond * 100
	maxDefenses    = 3
	defendInterval = time.Second * 10

	gcTick       = time.Minute
	timeToStale  = time.Second * 30
	timeToDelete = time.Minute * 3
)

const (
	OpResponse uint16 = iota + 1
	OpRequest
)

type EntryState int

const (
	StateReachable EntryState = iota
	StatePending
	StateStale
	StateFailed
)

type sender interface {
	SendToMAC(message []byte, dst nic.MACAddress, etherType uint16) error
}

type ARPModule struct {
	hrd      uint16
	hrdLen   uint8
	proto    uint16
	protoLen uint8

	hrdAddr   nic.MACAddress
	protoAddr ip.IPAddress
	sender    sender

	lastDefense   time.Time
	defendAttempt int

	table map[ip.IPAddress]*arpEntry
	mutex *sync.RWMutex
}

func NewARPModule(hrd uint16, hrdLen uint8, proto uint16, protoLen uint8, hrdAddr nic.MACAddress, protoAddr ip.IPAddress, sender sender) *ARPModule {
	return &ARPModule{
		hrd:       hrd,
		hrdLen:    hrdLen,
		proto:     proto,
		protoLen:  protoLen,
		hrdAddr:   hrdAddr,
		protoAddr: protoAddr,
		sender:    sender,
		table:     make(map[ip.IPAddress]*arpEntry),
		mutex:     new(sync.RWMutex),
	}
}

// TODO: on receiving response, update as well, but send to channel only if pending
// What's the deal with the garps
func (arp *ARPModule) RunGC() {
	ticker := time.NewTicker(gcTick)
	for range ticker.C {
		arp.mutex.Lock()
		for ip, entry := range arp.table {

			if time.Since(entry.lastUsed) > timeToDelete ||
				(entry.state == StateFailed && time.Since(entry.lastUpdated) > timeToDelete) {
				delete(arp.table, ip)
				if ch, ok := arp.pending[ip]; ok {
					close(ch)
					delete(arp.pending, ip)
				}
			}

			if entry.state == StateReachable && time.Since(entry.lastUpdated) > timeToStale {
				entry.state = StateStale
			}

		}
	}
}

func (arp *ARPModule) AwaitResponse(ip ip.IPAddress, ch <-chan struct{}) (nic.MACAddress, error) {
	for range retryAttempts {
		select {
		case <-ch:
			if ip == arp.protoAddr {
				return nic.MACAddress{}, ErrIPConflict
			}

			arp.mutex.RLock()
			defer arp.mutex.RUnlock()
			entry, ok := arp.table[ip]

			if ok && entry.state == StateReachable {
				return entry.mac, nil
			}

			return nic.MACAddress{}, fmt.Errorf("no reply: host unreachable")

		case <-time.After(retryInterval):
			continue
		}
	}

	if ip == arp.protoAddr {
		return nic.MACAddress{}, nil
	}

	arp.mutex.Lock()
	defer arp.mutex.Unlock()
	if arp.table[ip].state == StatePending {
		arp.updateEntry(StateFailed, ip, nic.MACAddress{})
	}
	return nic.MACAddress{}, fmt.Errorf("no reply: host unreachable")
}

func (arp *ARPModule) Resolve(ip ip.IPAddress) (nic.MACAddress, error) {
	arp.mutex.Lock()
	entry, ok := arp.table[ip]

	if !ok {
		arp.mutex.Unlock()
		ch, err := arp.sendRequest(ip, ethernet.BroadcastAddress)
		if err != nil {
			return nic.MACAddress{}, fmt.Errorf("error sending ARP request: %w", err)
		}
		arp.mutex.Lock()
		arp.table[ip].lastUsed = time.Now()
		arp.mutex.Unlock()
		return arp.AwaitResponse(ip, ch)
	}

	entry.lastUsed = time.Now()
	state := entry.state
	latestAttempted := entry.lastAttempted
	mac := entry.mac
	arp.mutex.Unlock()

	switch state {
	case StateReachable, StateStale:
		return mac, nil

	case StatePending:
		arp.mutex.RLock()
		ch := arp.pending[ip]
		arp.mutex.RUnlock()
		return arp.AwaitResponse(ip, ch)

	case StateFailed:
		if time.Since(latestAttempted) < retryInterval {
			return nic.MACAddress{}, fmt.Errorf("negative cache: recently failed")
		}
		ch, err := arp.sendRequest(ip, ethernet.BroadcastAddress)
		if err != nil {
			return nic.MACAddress{}, fmt.Errorf("error sending ARP request: %w", err)
		}
		return arp.AwaitResponse(ip, ch)

	default:
		return nic.MACAddress{}, fmt.Errorf("invalid state")
	}
}
