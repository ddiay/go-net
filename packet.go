package net

import (
	"encoding/binary"
	"fmt"
)

const (
	PrefixLengthSize = 4
)

type Packet struct {
}

type CallbackOnReadPrefixLengthPacket func(h *Handle, buffer []byte)

var _callbackOnReadPrefixLengthPacket CallbackOnReadPrefixLengthPacket

func SetOnReadPrefixPacketLengthCallback(onReadPrefixLengthPacket CallbackOnReadPrefixLengthPacket) {
	_callbackOnReadPrefixLengthPacket = onReadPrefixLengthPacket
}

func SendPrefixLengthPacket(h *Handle, buffer []byte, size int) {
	packetSize := PrefixLengthSize + size
	packet := make([]byte, packetSize)
	binary.LittleEndian.PutUint32(packet, uint32(size))
	copy(packet[4:], buffer)
	h.TrySend(packet, packetSize)
}

func ReadPrefixLengthPacket(h *Handle) {
	headBuffer := make([]byte, PrefixLengthSize)
	h.TryRead(headBuffer, PrefixLengthSize, onReadPrefixLength)
}

func onReadPrefixLength(h *Handle, buffer []byte, size int) {
	packetSize := int(binary.LittleEndian.Uint32(buffer[:PrefixLengthSize]))
	fmt.Printf("packetSize:%d\n", packetSize)
	packetBuffer := buffer
	if packetSize > len(buffer) {
		packetBuffer = make([]byte, packetSize)
	}
	h.TryRead(packetBuffer, packetSize, onReadPrefixLengthPacket)
}

func onReadPrefixLengthPacket(h *Handle, buffer []byte, size int) {
	_callbackOnReadPrefixLengthPacket(h, buffer[:size])

	headBuffer := buffer
	if size < PrefixLengthSize {
		headBuffer = make([]byte, PrefixLengthSize)
	}
	h.TryRead(headBuffer, PrefixLengthSize, onReadPrefixLength)
}
