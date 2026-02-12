package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"sync"
	"tcp-ip/internal/ethernet"
	"tcp-ip/internal/nic"
	"time"
)

const (
	slotSize        = 2048
	descriptorSlots = 1024
)

type Router struct {
	MACTable map[[6]byte]net.Conn
	memory   []byte
	ring     []nic.Descriptor
	NIC      *nic.NIC
	mutex    sync.Mutex
	host     string
}

func (router *Router) getMessage(conn net.Conn, waitTime int) ([]byte, error) {
	err := conn.SetReadDeadline(time.Now().Add(time.Duration(waitTime) * time.Minute))
	if err != nil {
		return nil, err
	}

	var length uint16
	err = binary.Read(conn, binary.BigEndian, &length)
	if err != nil {
		return nil, err
	}

	if int(length) > ethernet.MTU || length <= 0 {
		return nil, fmt.Errorf("invalid message length received")
	}

	buf := make([]byte, length)
	_, err = io.ReadFull(conn, buf)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

func (router *Router) forwardMessage(data []byte, conn net.Conn) error {
	length := uint16(len(data))
	err := conn.SetWriteDeadline(time.Now().Add(time.Duration(10) * time.Minute))
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

func (router *Router) switchConnection(conn net.Conn) {
	var srcMAC nic.MACAddress
	defer func() {
		fmt.Println("Clossing connection to: ", conn.RemoteAddr().String())
		delete(router.MACTable, srcMAC)
		_ = conn.Close()
	}()

	for {
		data, err := router.getMessage(conn, 10)
		if errors.Is(err, io.EOF) {
			fmt.Println("The client closed the connection")
			return
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error receiving message: ", err.Error())
			return
		}

		srcMAC = nic.MACAddress(data[6:12])
		router.mutex.Lock()
		router.MACTable[nic.MACAddress(srcMAC)] = conn
		fmt.Fprintf(os.Stderr, "MAC table updated: added %x\n", srcMAC)
		router.mutex.Unlock()

		dstMAC := nic.MACAddress(data[:6])
		dstConn, ok := router.MACTable[dstMAC]
		if !ok && dstMAC != ethernet.BroadcastAddress {
			fmt.Fprintln(os.Stderr, "Unknown MAC address received, dropping frame")
			continue
		}

		if dstMAC == ethernet.BroadcastAddress {
			_, _ = fmt.Fprintf(os.Stdout, "Broadcasting message from %x\n", srcMAC)
			for MAC, conn := range router.MACTable {
				if MAC == srcMAC {
					continue
				}
				err = router.forwardMessage(data, conn)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error forwading message from %x to %x: %s\n", srcMAC, dstMAC, err.Error())
				}
			}
			continue
		}

		err = router.forwardMessage(data, dstConn)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error forwading message from %x to %x: %s\n", srcMAC, dstMAC, err.Error())
			continue
		}
		_, _ = fmt.Fprintf(os.Stdout, "Forwaded message from %x to %x\n", srcMAC, dstMAC)
	}
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))
	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Could not create listener:", err.Error())
		return
	}

	router := &Router{
		MACTable: make(map[[6]byte]net.Conn),
		memory:   make([]byte, slotSize*descriptorSlots),
		ring:     make([]nic.Descriptor, descriptorSlots),
		host:     listener.Addr().String(),
	}
	router.NIC = nic.NewNIC(router.memory, router.ring, slotSize)
	fmt.Println("Server started at:", router.host)

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Could not accept connection:", err.Error())
		}
		fmt.Println("Connection received from:", conn.RemoteAddr().String())
		go router.switchConnection(conn)
	}
}
