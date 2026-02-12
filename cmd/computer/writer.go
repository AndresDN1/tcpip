package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"sync"
	"tcp-ip/internal/arp"
	"tcp-ip/internal/ethernet"
	"tcp-ip/internal/ip"
	"tcp-ip/internal/nic"
	"tcp-ip/pkg/utils"
	"time"
)

func (computer *Computer) SendToMAC(message []byte, dstMAC nic.MACAddress, etherType uint16) error {
	conn := computer.routerConn
	frame, err := ethernet.NewFrame(computer.nic.MAC, dstMAC, etherType, message)
	if err != nil {
		return err
	}
	data := frame.Serialize()
	length := uint16(len(data))

	err = conn.SetWriteDeadline(time.Now().Add(time.Duration(10) * time.Minute))
	if err != nil {
		return err
	}
	err = binary.Write(conn, binary.BigEndian, length)
	if err != nil {
		return err
	}

	err = conn.SetWriteDeadline(time.Now().Add(time.Duration(10) * time.Minute))
	if err != nil {
		return err
	}
	_, err = conn.Write(data)
	return err
}

func (computer *Computer) sendToIP(message []byte, dstIP ip.IPAddress) error {
	dstMAC, err := computer.arp.Resolve(dstIP)
	if err != nil {
		return fmt.Errorf("could not resolve IP address: %w", err)
	}

	err = computer.SendToMAC(message, dstMAC, ethernet.IPv4EtherType)
	if err != nil {
		return fmt.Errorf("could not send message to MAC: %w", err)
	}
	return nil
}

func (computer *Computer) handleSending(wg *sync.WaitGroup) {
	defer func() {
		wg.Done()
		_ = computer.routerConn.Close()
	}()

	ch, err := computer.arp.SendGARP()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Could not send GARP:", err.Error())
		return
	}

	_, err = computer.arp.AwaitResponse(computer.ip, ch)

	if errors.Is(err, arp.ErrIPConflict) {
		fmt.Fprintln(os.Stderr, "Critical error:", err.Error())
		return
	} else if err != nil {
		fmt.Fprintln(os.Stderr, "Error awaiting response to GARP:", err.Error())
		return
	}

	for {
		dstIPStr, err := utils.PromptString(computer.reader, "Enter destination IP address:")
		if err != nil {
			fmt.Fprintln(os.Stderr, "Could not read input:", err.Error())
			continue
		}

		dstIP, err := ip.ParseIP(dstIPStr)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Could not parse IP address:", err.Error())
			continue
		}

		payload, err := utils.PromptBytes(computer.reader, "Enter payload:")
		if err != nil {
			fmt.Fprintln(os.Stderr, "Could not read input:", err.Error())
			continue
		}

		err = computer.sendToIP(payload, dstIP)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Could not send message to IP: ", err.Error())
		}
	}
}
