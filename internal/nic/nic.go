package nic

import (
	"encoding/binary"
	"fmt"
	"os"
	"strings"
)

var (
	MACStringLength = 17
	CPUOwned        = 1
	NICOwned        = 0
)

type MACAddress [6]byte

type Descriptor struct {
	Length int
	Owner  int
}

type NIC struct {
	MAC       MACAddress
	memory    []byte
	ring      []Descriptor
	slotIndex int
	SlotSize  int
}

func (nic *NIC) LoadFrame(data []byte) (int, error) {
	index := nic.slotIndex
	loop := 0
	for nic.ring[index].Owner != NICOwned {
		index = (index + 1) % len(nic.ring)
		loop += 1
		if loop == len(nic.ring) {
			return 0, fmt.Errorf("full kernel ring, could not load frame")
		}
	}

	ring := &nic.ring[index]
	ring.Owner = NICOwned
	ring.Length = len(data)
	copy(nic.memory[nic.SlotSize*index:], data)
	ring.Owner = CPUOwned
	return index, nil
}

func NewNIC(memory []byte, ring []Descriptor, slotSize int) *NIC {
	var MAC MACAddress
	MAC[0] = 0x02
	PID := uint32(os.Getpid())
	binary.BigEndian.PutUint32(MAC[2:], PID)
	return &NIC{memory: memory, ring: ring, MAC: MAC, SlotSize: slotSize}
}

func ParseMAC(MAC string) (MACAddress, error) {
	var result MACAddress
	if len(MAC) != MACStringLength {
		return result, fmt.Errorf("invalid MAC address")
	}
	MAC = strings.ToLower(MAC)
	i := 0
	for i < len(result) {
		hn := MAC[i*3]
		if hn >= '0' && hn <= '9' {
			hn -= '0'
		} else if hn >= 'a' && hn <= 'f' {
			hn = hn - 'a' + 10
		} else {
			return result, fmt.Errorf("invalid MAC address")
		}

		ln := MAC[i*3+1]
		if ln >= '0' && ln <= '9' {
			ln -= '0'
		} else if ln >= 'a' && ln <= 'f' {
			ln = ln - 'a' + 10
		} else {
			return result, fmt.Errorf("invalid MAC address")
		}

		if i < 5 {
			sep := MAC[i*3+2]
			if sep != '-' && sep != ':' {
				return result, fmt.Errorf("invalid MAC address")
			}
		}

		result[i] = (hn << 4) | ln
		i += 1
	}
	return result, nil
}
