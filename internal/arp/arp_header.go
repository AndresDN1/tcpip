package arp

import (
	"bytes"
	"encoding/binary"
	"tcp-ip/internal/nic"
)

type ARPPacket struct {
	HardwareType          uint16
	ProtocolType          uint16
	HardwareLength        uint8
	ProtocolLength        uint8
	Operation             uint16
	SenderHardwareAddress nic.MACAddress
	SenderProtocolAddress uint32
	TargetHardwareAddress nic.MACAddress
	TargetProtocolAddress uint32
}

func (header *ARPPacket) Serialize() [28]byte {
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.BigEndian, header.HardwareType)
	_ = binary.Write(buf, binary.BigEndian, header.ProtocolType)
	_ = binary.Write(buf, binary.BigEndian, header.HardwareLength)
	_ = binary.Write(buf, binary.BigEndian, header.ProtocolLength)
	_ = binary.Write(buf, binary.BigEndian, header.Operation)
	buf.Write(header.SenderHardwareAddress[:])
	_ = binary.Write(buf, binary.BigEndian, header.SenderProtocolAddress)
	buf.Write(header.TargetHardwareAddress[:])
	_ = binary.Write(buf, binary.BigEndian, header.TargetProtocolAddress)
	return [28]byte(buf.Bytes())
}

func Deserialize(data [28]byte) *ARPPacket {
	header := &ARPPacket{}
	header.HardwareType = binary.BigEndian.Uint16(data[:2])
	header.ProtocolType = binary.BigEndian.Uint16(data[2:4])
	header.HardwareLength = data[4]
	header.ProtocolLength = data[5]
	header.Operation = binary.BigEndian.Uint16(data[6:8])
	copy(header.SenderHardwareAddress[:], data[8:14])
	header.SenderProtocolAddress = binary.BigEndian.Uint32(data[14:18])
	copy(header.TargetHardwareAddress[:], data[18:24])
	header.TargetProtocolAddress = binary.BigEndian.Uint32(data[24:28])
	return header
}
