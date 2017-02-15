package main

import (
	"net"
	"sync"

	"github.com/armon/go-proxyproto"
)

// proxyListenerRouter is a net.Listener which can route net.Conns to different
// 'sub' listeners based on port.
type proxyListenerRouter struct {
	mtx       sync.Mutex
	listeners map[int]*proxyListener
	original  net.Listener
}

type proxyListener struct {
	parent *proxyListenerRouter
	c      chan net.Conn
}

func newProxyListenerRouter(original net.Listener) *proxyListenerRouter {
	return &proxyListenerRouter{
		listeners: map[int]*proxyListener{},
		original:  original,
	}
}

func (p *proxyListenerRouter) listenerForPort(port int) net.Listener {
	listener := &proxyListener{
		parent: p,
		c:      make(chan net.Conn),
	}
	p.mtx.Lock()
	defer p.mtx.Unlock()
	p.listeners[port] = listener
	return listener
}

// Accept waits for and returns the next connection to the listener.
func (p *proxyListenerRouter) Accept() (net.Conn, error) {
	for {
		conn, err := p.original.Accept()
		if err != nil {
			return nil, err
		}

		// Check this is a proxy proto connection
		if proxyConn, ok := conn.(*proxyproto.Conn); ok {
			localAddr := proxyConn.DstAddr().(*net.TCPAddr)
			p.mtx.Lock()
			listener, ok := p.listeners[localAddr.Port]
			p.mtx.Unlock()
			if ok {
				listener.c <- conn
				continue
			}
		}

		return conn, nil
	}
}

// Close closes the listener.
// Any blocked Accept operations will be unblocked and return errors.
func (p *proxyListenerRouter) Close() error {
	return p.original.Close()
}

// Addr returns the listener's network address.
func (p *proxyListenerRouter) Addr() net.Addr {
	return p.original.Addr()
}

// Accept waits for and returns the next connection to the listener.
func (p *proxyListener) Accept() (net.Conn, error) {
	return <-p.c, nil
}

// Close closes the listener.
// Any blocked Accept operations will be unblocked and return errors.
func (p *proxyListener) Close() error {
	return p.parent.Close()
}

// Addr returns the listener's network address.
func (p proxyListener) Addr() net.Addr {
	return p.parent.Addr()
}
