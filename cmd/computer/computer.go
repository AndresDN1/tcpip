package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"tcp-ip/internal/arp"
	"tcp-ip/internal/ethernet"
	"tcp-ip/internal/ip"
	"tcp-ip/internal/nic"
	"tcp-ip/pkg/utils"
	"time"
)

const (
	slotSize        = 2048
	descriptorSlots = 64
	reconnectDelay  = 3
)

type Computer struct {
	memory     []byte
	ring       []nic.Descriptor
	routerConn net.Conn
	ip         ip.IPAddress
	nic        *nic.NIC
	reader     *bufio.Reader
	arp        *arp.ARPModule
}

func (computer *Computer) readMemory(slotIndex int) ([]byte, error) {
	slot := &computer.ring[slotIndex]
	if slot.Owner != nic.CPUOwned {
		return nil, fmt.Errorf("slot currently owned by the NIC")
	}
	data := make([]byte, slot.Length)
	copy(data, computer.memory[slotIndex*slotSize:])
	slot.Owner = nic.NICOwned
	return data, nil
}

func (computer *Computer) connectToRouter() error {
	address, err := utils.PromptString(computer.reader, "Enter router address to connect to:")
	if err != nil {
		return err
	}
	conn, err := net.Dial("tcp", address)
	computer.routerConn = conn
	return err
}

func parseIpArgs() (ip.IPAddress, error) {
	flag.Parse()
	args := flag.Args()
	if len(args) < 1 {
		return ip.IPAddress{}, fmt.Errorf("IP address argument expected")
	}
	if len(args) > 1 {
		return ip.IPAddress{}, fmt.Errorf("unexpected extra arguments")
	}
	return ip.ParseIP(args[0])
}

func main() {
	ip, err := parseIpArgs()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Invalid arguments:", err.Error())
		return
	}
	reader := bufio.NewReader(io.LimitReader(os.Stdin, int64(ethernet.MaxFramePayload)))
	computer := &Computer{reader: reader, ip: ip, memory: make([]byte, slotSize*descriptorSlots), ring: make([]nic.Descriptor, descriptorSlots)}
	computer.nic = nic.NewNIC(computer.memory, computer.ring, slotSize)

	for {
		err := computer.connectToRouter()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Could not connect to router:", err.Error())
			continue
		}

		computer.arp = arp.NewARPModule(arp.HrdEthernet, arp.HrdLenEthernet, arp.ProtoIPv4, arp.ProtoLenIpv4, computer.nic.MAC, computer.ip, computer)
		wg := new(sync.WaitGroup)
		wg.Add(2)
		go computer.handleSending(wg)
		go computer.handleReceiving(wg)
		wg.Wait()

		reconnect, err := utils.PromptString(computer.reader, "Enter 1 to reconnect")
		if err != nil {
			fmt.Fprintln(os.Stderr, "Could not read input:", err.Error())
		}
		if reconnect == "1" {
			fmt.Println("Reconnecting in 3", reconnectDelay, "seconds...")
			time.Sleep(time.Duration(reconnectDelay) * time.Second)
			continue
		}
		break
	}
}
