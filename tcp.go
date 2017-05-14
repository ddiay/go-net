package net

import "net"
import "sync"
import "container/list"

type Tcp struct {
	conn       net.Conn
	listener   net.Listener
	addr       string
	islistener bool

	OnConnectResult CallbackConnect
	OnDisconnect    CallbackDisconnect
	acceptChan      chan *Handle
	waitAccept      sync.WaitGroup
	handleList      list.List
}

func CreateTcp(islistener bool, addr string, onConnect CallbackConnect, onDisconnect CallbackDisconnect) *Tcp {
	tcp := &Tcp{
		addr:       addr,
		islistener: islistener,

		OnConnectResult: onConnect,
		OnDisconnect:    onDisconnect,

		acceptChan: make(chan *Handle, 1000),
	}
	return tcp
}

func (t *Tcp) Start() error {
	if t.islistener == true {
		return t.startListen()
	} else {
		t.startConnect()
	}
	return nil
}

func (t *Tcp) Stop() {
	for e := t.handleList.Front(); e != nil; {
		next := e.Next()
		h := e.Value.(*Handle)
		h.Close()
		e = next
	}

	if t.islistener {
		t.listener.Close()
	}

	t.waitAccept.Wait()
}

func (t *Tcp) startConnect() {
	conn, err := net.Dial("tcp", t.addr)
	if err != nil {
		t.OnConnectResult(nil, false, err)
		return
	}

	h := createHandle(conn, &t.handleList, t.OnConnectResult, t.OnDisconnect)
	t.OnConnectResult(h, true, nil)
	h.startLoop()
}

func (t *Tcp) startListen() error {
	listener, err := net.Listen("tcp", t.addr)
	if err != nil {
		return err
	}

	t.startAccept(listener)

	return nil
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
