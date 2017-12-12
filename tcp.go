package net

import (
	"container/list"
	"fmt"
	"net"
	"sync"
)

type Tcp struct {
	listener net.Listener
	addr     string

	OnConnectResult CallbackConnect
	OnDisconnect    CallbackDisconnect
	acceptChan      chan *Handle
	waitAccept      sync.WaitGroup
	handleList      list.List
}

func NewTcp() *Tcp {
	tcp := &Tcp{}
	return tcp
}

func (t *Tcp) WhenConnected(cb CallbackConnect) *Tcp {
	t.OnConnectResult = cb
	return t
}

func (t *Tcp) WhenDisconnected(cb CallbackDisconnect) *Tcp {
	t.OnDisconnect = cb
	return t
}

func (t *Tcp) Listen(port int) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}
	t.acceptChan = make(chan *Handle, 1000)

	t.startAccept(listener)

	return nil
}

func (t *Tcp) Connect(addr string) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.OnConnectResult(nil, false, err)
		return
	}

	h := createHandle(conn, &t.handleList, t.OnConnectResult, t.OnDisconnect)
	t.OnConnectResult(h, true, nil)
	h.startLoop()
}

func (t *Tcp) Close() {
	for e := t.handleList.Front(); e != nil; {
		next := e.Next()
		h := e.Value.(*Handle)
		h.Close()
		e = next
	}

	if t.listener != nil {
		t.listener.Close()
	}

	t.waitAccept.Wait()
	t.listener = nil
}

func (t *Tcp) startAccept(listener net.Listener) {
	t.waitAccept.Add(1)
	go func() {
		defer t.waitAccept.Done()

		conn, err := listener.Accept()
		if err != nil {
			return
		}
		t.startAccept(listener)

		h := createHandle(conn, &t.handleList, t.OnConnectResult, t.OnDisconnect)
		h.onConnectResult(h, true, nil)
		h.startLoop()
	}()
}
