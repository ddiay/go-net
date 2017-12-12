package net

import (
	"container/list"
	"fmt"
	"net"
	"sync"
)

type CallbackConnect func(*Handle, bool, error)
type CallbackDisconnect func(*Handle, error)
type CallbackRead func(*Handle, []byte, int)

type NetPacket struct {
	PacketSize int
	Buffer     []byte
	OnRead     CallbackRead
}

type Handle struct {
	conn              net.Conn
	netPacketChan     chan *NetPacket
	sendPacketChan    chan *NetPacket
	onConnectResult   CallbackConnect
	onDisconnect      CallbackDisconnect
	waitGroup         sync.WaitGroup
	handleListElement *list.Element
	handleList        *list.List
}

func createHandle(conn net.Conn, handleList *list.List, onConnectResult CallbackConnect, onDisconnect CallbackDisconnect) *Handle {
	h := &Handle{
		conn:            conn,
		onConnectResult: onConnectResult,
		onDisconnect:    onDisconnect,
		netPacketChan:   make(chan *NetPacket, 5),
		sendPacketChan:  make(chan *NetPacket, 5),
	}

	h.handleList = handleList
	h.handleListElement = handleList.PushBack(h)

	return h
}

func (h *Handle) TryRead(buffer []byte, size int, onRead CallbackRead) {
	if buffer == nil || len(buffer) < size {
		return
	}
	netPacket := &NetPacket{
		PacketSize: size,
		Buffer:     buffer,
		OnRead:     onRead,
	}
	h.netPacketChan <- netPacket
}

func (h *Handle) TrySend(buffer []byte, size int) {
	if buffer == nil || len(buffer) < size {
		return
	}
	netPacket := &NetPacket{
		PacketSize: size,
		Buffer:     buffer,
	}
	h.sendPacketChan <- netPacket
}

func (h *Handle) Close() {
	h.conn.Close()
	h.waitGroup.Wait()
	h.removeFromList()
}

func (h *Handle) Addr() net.Addr {
	return h.conn.RemoteAddr()
}

func (h *Handle) removeFromList() {
	if h.handleListElement.Next() != nil {
		h.handleList.Remove(h.handleListElement)
	}
}

func (h *Handle) startLoop() {
	h.waitGroup.Add(1)
	go func() {
		defer h.waitGroup.Done()
		h.readLoop()
	}()

	h.waitGroup.Add(1)
	go func() {
		defer h.waitGroup.Done()
		h.writeLoop()
	}()
}

func (h *Handle) readLoop() {
	var disconnectErr error
	defer func() {
		h.removeFromList()
		h.onDisconnect(h, disconnectErr)
	}()

	for {
		select {
		case netPacket, ok := <-h.netPacketChan:
			if !ok {
				return
			}
			var pos, needReadSize int
			if netPacket.PacketSize <= 0 {
				needReadSize = len(netPacket.Buffer)
			} else {
				needReadSize = netPacket.PacketSize
			}
			if needReadSize > 0 {
				for {
					numReceived, err := h.conn.Read(netPacket.Buffer[pos:needReadSize])
					if err != nil {
						disconnectErr = err
						return
					}
					if numReceived <= 0 {
						fmt.Println("numReceived value <= 0")
						return
					}
					pos += numReceived
					needReadSize -= numReceived
					if netPacket.PacketSize <= 0 || pos == netPacket.PacketSize {
						netPacket.OnRead(h, netPacket.Buffer, pos)
						break
					}
				}
			}
		}
	}
}

func (h *Handle) writeLoop() {
	var disconnectErr error
	defer func() {
		h.removeFromList()
		h.onDisconnect(h, disconnectErr)
	}()

	for {
		select {
		case sendPacket, ok := <-h.sendPacketChan:
			if !ok {
				return
			}
			if sendPacket.PacketSize > 0 {
				_, err := h.conn.Write(sendPacket.Buffer[:sendPacket.PacketSize])
				if err != nil {
					disconnectErr = err
					return
				}
			}
		}
	}
}
