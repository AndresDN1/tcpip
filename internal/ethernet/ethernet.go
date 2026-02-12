package ethernet

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"tcp-ip/internal/nic"
)

type Frame struct {
	DstMAC    [6]byte
	SrcMAC    [6]byte
	EtherType uint16
	Data      []byte
	FCS       uint32
}

var (
	BroadcastAddress        = nic.MACAddress{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}
	MTU                     = 5018
	ARPEtherType     uint16 = 0x0806
	IPv4EtherType    uint16 = 0x0800
	TestEtherType    uint16 = 0x0000
	MaxFramePayload         = MTU - frameOverhead
	MinFrame                = 46
	frameOverhead           = 18
)

func CRC(frame []byte) uint32 {
	polynomial := uint32(0xEDB88320)
	result := uint32(0xFFFFFFFF)
	for _, b := range frame {
		result ^= uint32(b)
		for range 8 {
			lsb := result & 1
			if lsb == 1 {
				result = (result >> 1) ^ polynomial
			} else {
				result >>= 1
			}
		}
	}
	return ^result
}

func (frame *Frame) Serialize() []byte {
	buf := new(bytes.Buffer)
	buf.Write(frame.DstMAC[:])
	buf.Write(frame.SrcMAC[:])
	_ = binary.Write(buf, binary.BigEndian, frame.EtherType)
	buf.Write(frame.Data)
	frame.FCS = CRC(buf.Bytes())
	_ = binary.Write(buf, binary.BigEndian, frame.FCS)
	return buf.Bytes()
}

func Deserialize(data []byte) (*Frame, error) {
	if len(data) > MTU || len(data) < MinFrame {
		return nil, fmt.Errorf("invalid frame: invalid frame size")
	}
	frame := &Frame{}
	copy(frame.DstMAC[:], data[:6])
	copy(frame.SrcMAC[:], data[6:12])
	frame.EtherType = binary.BigEndian.Uint16(data[12:14])
	frame.Data = data[14 : len(data)-4]
	frame.FCS = binary.BigEndian.Uint32(data[len(data)-4:])
	calculatedFCS := CRC(data[:len(data)-4])
	if calculatedFCS != frame.FCS {
		return nil, fmt.Errorf("invalid frame: fcs doesn't match")
	}
	return frame, nil
}

func NewFrame(src, dst [6]byte, etherType uint16, data []byte) (*Frame, error) {
	if len(data) > MaxFramePayload {
		return nil, fmt.Errorf("data length exceeds the MTU")
	}
	if len(data) < MinFrame {
		newSlice := make([]byte, MinFrame)
		copy(newSlice, data)
		data = newSlice
	}
	return &Frame{DstMAC: dst, SrcMAC: src, EtherType: etherType, Data: data}, nil
}
