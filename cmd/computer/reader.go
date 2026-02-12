package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"tcp-ip/internal/arp"
	"tcp-ip/internal/ethernet"
	"tcp-ip/internal/nic"
	"time"
)

func (computer *Computer) isForMe(data []byte) bool {
	if len(data) < ethernet.MinFrame {
		fmt.Println("Too small frame received, dropping frame")
		return false
	}
	dstMAC := nic.MACAddress(data[0:6])
	if dstMAC != computer.nic.MAC && dstMAC != ethernet.BroadcastAddress {
		fmt.Println("Wrong destination received, dropping frame")
		return false
	}
	if len(data) > ethernet.MTU || len(data) < ethernet.MinFrame {
		fmt.Println("Invalid frame size received, dropping frame")
		return false
	}
	if binary.BigEndian.Uint32(data[len(data)-4:]) != ethernet.CRC(data[:len(data)-4]) {
		fmt.Println("FSC doesn't match, dropping frame")
		return false
	}
	return true
}

func (computer *Computer) getMessage(conn net.Conn, waitTime int, maxMessageLength int) ([]byte, error) {
	err := conn.SetReadDeadline(time.Now().Add(time.Duration(waitTime) * time.Minute))
	if err != nil {
		return nil, err
	}

	var length uint16
	err = binary.Read(conn, binary.BigEndian, &length)
	if err != nil {
		return nil, err
	}

	if length > uint16(maxMessageLength) || length <= 0 {
		return nil, fmt.Errorf("invalid message length received")
	}

	buf := make([]byte, length)
	_, err = io.ReadFull(conn, buf)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

func (computer *Computer) handleReceiving(wg *sync.WaitGroup) {
	defer func() {
		wg.Done()
		_ = computer.routerConn.Close()
	}()

	for {
		data, err := computer.getMessage(computer.routerConn, 10, ethernet.MTU)
		if errors.Is(err, io.EOF) {
			fmt.Println("The server closed the connection")
			return
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error receiving message:", err.Error())
			return
		}

		if !computer.isForMe(data) {
			continue
		}

		slotIndex, err := computer.nic.LoadFrame(data)
		if err != nil {
			fmt.Println("Could not write to memory, dropping frame:", err.Error())
			continue
		}

		data, err = computer.readMemory(slotIndex)
		if err != nil {
			fmt.Println("Could not read from memory, dropping frame:", err.Error())
			continue
		}

		frame, err := ethernet.Deserialize(data)
		if err != nil {
			fmt.Println("Could not parse frame, dropping frame:", err.Error())
			continue
		}

		err = computer.dispatch(frame)
		if err != nil && errors.Is(err, arp.ErrMaxDefensesReached) {
			fmt.Println("Critical error: ", err.Error())
			fmt.Println("Shutting down system")
			return
		} else if err != nil {
			fmt.Println("Could not dispatch frame:", err.Error())
		}
	}
}
