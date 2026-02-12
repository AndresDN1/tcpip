package arp

import (
	"fmt"
	"os"
	"tcp-ip/internal/ip"
	"tcp-ip/internal/nic"
	"time"
)

type arpEntry struct {
	mac       nic.MACAddress
	state     EntryState
	pendingCh chan struct{}

	lastUsed      time.Time
	lastUpdated   time.Time
	lastAttempted time.Time
}

func newARPEntry(mac nic.MACAddress, state EntryState) *arpEntry {
	return &arpEntry{
		mac:   mac,
		state: state,
	}
}

// check if the target is the same as current to return imediately if necessary
func (arp *ARPModule) updateEntry(state EntryState, ip ip.IPAddress, newMAC nic.MACAddress) {
	entry := arp.table[ip]
	if entry.state == StatePending {
		close(entry.pendingCh)
		entry.pendingCh = nil
	}

	switch state {
	case StateReachable:
		entry.lastUpdated = time.Now()
		entry.mac = newMAC
		entry.state = StateReachable

	case StatePending:
		entry.state = StatePending
		entry.pendingCh = make(chan struct{})

	case StateFailed:
		entry.state = StateFailed
	}

	_, _ = fmt.Fprintf(os.Stdout, "Updated entry %v to %x\n with the state %v", ip, newMAC, state)
}
