package arp

import (
	"encoding/binary"
	"fmt"
	"os"
	"tcp-ip/internal/ip"
	"time"
)

func (arp *ARPModule) defendIP() error {
	arp.mutex.Lock()
	if time.Since(arp.lastDefense) > defendInterval {
		arp.defendAttempt = 0
	}

	arp.defendAttempt++
	arp.lastDefense = time.Now()
	if arp.defendAttempt > maxDefenses {
		return ErrMaxDefensesReached
	}
	arp.mutex.Unlock()

	_, err := arp.SendGARP()
	if err != nil {
		return fmt.Errorf("could not send defense GARP: %w", err)
	}
	arp.mutex.Lock()
	close(arp.pending[arp.protoAddr])
	delete(arp.pending, arp.protoAddr)
	arp.mutex.Unlock()
	return nil
}

func (arp *ARPModule) handleConflict(ip ip.IPAddress, entry *arpEntry, packet *ARPPacket) {
	arp.mutex.RLock()
	state := entry.state
	currentMac := entry.mac
	arp.mutex.RUnlock()

	switch state {
	case StateReachable, StateStale:
		_, _ = fmt.Fprintf(os.Stdout, "Conflict detected, sending verification unicast to: %x\n", currentMac)
		ch, err := arp.sendRequest(ip, entry.mac)
		if err != nil {
			fmt.Println("Could not send verification message to :", err.Error())
			return
		}

		_, err = arp.AwaitResponse(ip, ch)
		if err != nil {
			arp.mutex.RLock()
			if entry.state == StateFailed {
				_, _ = fmt.Fprintf(os.Stdout, "%x did not respond, updating to %x", entry.mac, packet.SenderHardwareAddress)
				arp.mutex.RUnlock()
				arp.updateEntry(entry, ip, packet.SenderHardwareAddress)
			} else {
				arp.mutex.RUnlock()
				fmt.Println("Did not get response, a different goroutine already updated the state")
			}
			return
		}
		fmt.Println("Response obtained, not modifying entry")

	case StatePending:
		arp.mutex.RLock()
		ch, ok := arp.pending[ip]
		arp.mutex.RUnlock()
		if !ok {
			fmt.Println("pending channel for pending entry does not exist")
			return
		}

		fmt.Println("Pending request already exists, joining queque")
		_, err := arp.AwaitResponse(ip, ch)
		if err != nil {
			arp.mutex.RLock()
			if entry.state == StateFailed {
				_, _ = fmt.Fprintf(os.Stdout, "%x did not respond, updating to %x", entry.mac, packet.SenderHardwareAddress)
				arp.mutex.RUnlock()
				arp.updateEntry(entry, ip, packet.SenderHardwareAddress)
			} else {
				arp.mutex.RUnlock()
				fmt.Println("Did not get response, a different goroutine already updated the state")
			}
			return
		}
		fmt.Println("from ending: Response obtained, not updating")

	case StateFailed:
		arp.updateEntry(entry, ip, packet.SenderHardwareAddress)

	default:
		arp.updateEntry(entry, ip, packet.SenderHardwareAddress)
	}
}

func (arp *ARPModule) handleResponse(packet *ARPPacket) error {
	var senderIP, targetIP ip.IPAddress
	binary.BigEndian.PutUint32(senderIP[:], packet.SenderProtocolAddress)
	if senderIP == arp.protoAddr {
		err := arp.defendIP()
		if err != nil {
			return fmt.Errorf("error defending IP: %w", err)
		}
		return nil
	}

	binary.BigEndian.PutUint32(targetIP[:], packet.TargetProtocolAddress)
	if targetIP != arp.protoAddr {
		return nil
	}

	arp.mutex.RLock()
	entry, ok := arp.table[senderIP]
	if !ok || entry.state != StatePending {
		arp.mutex.RUnlock()
		return nil
	}

	if _, ok = arp.pending[senderIP]; !ok {
		arp.mutex.RUnlock()
		return fmt.Errorf("pending channel for pending entry does not exist")
	}
	arp.mutex.RUnlock()

	arp.updateEntry(entry, senderIP, packet.SenderHardwareAddress)

	arp.mutex.Lock()
	close(arp.pending[senderIP])
	delete(arp.pending, senderIP)
	arp.mutex.Unlock()
	return nil
}

func (arp *ARPModule) handleRequest(packet *ARPPacket) error {
	var senderIP, targetIP ip.IPAddress
	binary.BigEndian.PutUint32(senderIP[:], packet.SenderProtocolAddress)
	if senderIP == arp.protoAddr {
		err := arp.defendIP()
		if err != nil {
			return fmt.Errorf("error defending IP: %w", err)
		}
		fmt.Println("Another node is trying to occupy the IP, defending it")
		return nil
	}

	binary.BigEndian.PutUint32(targetIP[:], packet.TargetProtocolAddress)
	arp.mutex.Lock()
	entry, ok := arp.table[senderIP]
	if ok && entry.mac != packet.SenderHardwareAddress {
		arp.mutex.Unlock()
		fmt.Println("i'm conflicted")
		go arp.handleConflict(senderIP, entry, packet)
	} else if !ok {
		arp.table[senderIP] = newARPEntry(packet.SenderHardwareAddress, StateReachable)
		arp.table[senderIP].lastUpdated = time.Now()
		arp.mutex.Unlock()
		_, _ = fmt.Fprintf(os.Stdout, "Added to the table %v:%x\n", senderIP, packet.SenderHardwareAddress)
	} else {
		arp.table[senderIP].lastUpdated = time.Now()
		arp.mutex.Unlock()
	}

	if targetIP != arp.protoAddr {
		return nil
	}

	err := arp.sendResponse(senderIP, packet.SenderHardwareAddress)
	return err
}

func (arp *ARPModule) Receive(data []byte) error {
	packet := Deserialize([28]byte(data))

	switch packet.Operation {
	case OpRequest:
		err := arp.handleRequest(packet)
		return err
	case OpResponse:
		return arp.handleResponse(packet)
	default:
		fmt.Println("Unrecognized OPCode")
		return nil
	}
}
