package main

import (
	"fmt"
	"os"
	"tcp-ip/internal/ethernet"
)

// look around for the delay and race
// logging and don't add to table when defending
// probs
// gc
// check locks and paralelization
// TODO: refactors:
// handle the GC
// some stuff going on when you try to talk to yourself
// go over everything carefully again
// refactor errors and logins throughout
// smart switch logic
func (computer *Computer) dispatch(frame *ethernet.Frame) error {
	switch frame.EtherType {
	case ethernet.IPv4EtherType:
		_, _ = fmt.Fprintf(os.Stdout, "Frame received\nDestination: %x\nSource: %x\nEtherType: %d\nPayload: %s\nCRC: %d\n",
			frame.DstMAC, frame.SrcMAC, frame.EtherType, frame.Data, frame.FCS)
		return nil

	case ethernet.ARPEtherType:
		return computer.arp.Receive(frame.Data)

	default:
		return fmt.Errorf("unrecognized ethertype")

	}
}
