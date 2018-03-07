package net

import (
	"container/list"
	"encoding/binary"
	"errors"
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
	onRead            CallbackRead
	waitGroup         sync.WaitGroup
	handleListElement *list.Element
	handleList        *list.List
}

func onReadHead(h *Handle, data []byte, size int) {
	packetSize := int(binary.LittleEndian.Uint32(data[:4]))
	h.ReadSize(packetSize, onReadBody)
}

func onReadBody(h *Handle, data []byte, size int) {
	h.onRead(h, data, size)
	h.ReadSize(4, onReadBody)
}

func createHandle(conn net.Conn, handleList *list.List, onConnectResult CallbackConnect, onDisconnect CallbackDisconnect, onRead CallbackRead) *Handle {
	h := &Handle{
		conn:            conn,
		onConnectResult: onConnectResult,
		onDisconnect:    onDisconnect,
		onRead:          onRead,
		netPacketChan:   make(chan *NetPacket, 5),
		sendPacketChan:  make(chan *NetPacket, 5),
	}

	h.handleList = handleList
	h.handleListElement = handleList.PushBack(h)

	return h
}

func (h *Handle) Read(onRead CallbackRead) {
	buf := make([]byte, 4096)
	h.ReadToBuffer(buf, 0, onRead)
}

func (h *Handle) ReadSize(size int, onRead CallbackRead) {
	if size > 0 {
		buf := make([]byte, size)
		h.ReadToBuffer(buf, size, onRead)
	} else {
		h.Read(onRead)
	}
}

func (h *Handle) ReadToBuffer(buffer []byte, size int, onRead CallbackRead) {
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

func (h *Handle) Send(data []byte) {
	if data == nil {
		return
	}
	headSize := len(data)
	packetSize := 4 + headSize
	packet := make([]byte, packetSize)
	binary.LittleEndian.PutUint32(packet, uint32(headSize))
	copy(packet[4:], data)
	netPacket := &NetPacket{
		PacketSize: packetSize,
		Buffer:     packet,
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
	h.ReadSize(4, onReadHead)
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
						disconnectErr = errors.New("numReceived value <= 0")
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
