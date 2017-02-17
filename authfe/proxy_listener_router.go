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

	conns  chan net.Conn
	errs   chan error
	closed chan struct{}
}

type proxyListener struct {
	parent *proxyListenerRouter
	c      chan net.Conn
}

func newProxyListenerRouter(original net.Listener) *proxyListenerRouter {
	p := &proxyListenerRouter{
		listeners: map[int]*proxyListener{},
		original:  original,
		conns:     make(chan net.Conn),
		errs:      make(chan error),
		closed:    make(chan struct{}),
	}
	go p.loop()
	return p
}

func (p *proxyListenerRouter) loop() {
	for {
		select {
		case <-p.closed:
			return
		default:
		}

		conn, err := p.original.Accept()
		if err != nil {
			p.errs <- err
			continue
		}

		// Check this is a proxy proto connection
		proxyConn, ok := conn.(*proxyproto.Conn)
		if !ok {
			p.conns <- conn
			continue
		}

		go func(proxyConn *proxyproto.Conn) {
			localAddr := proxyConn.DstAddr().(*net.TCPAddr)

			// Can happen if proxyProto fails to read the header
			if localAddr == nil {
				p.conns <- proxyConn
				return
			}

			p.mtx.Lock()
			listener, ok := p.listeners[localAddr.Port]
			p.mtx.Unlock()
			if ok {
				listener.c <- proxyConn
			} else {
				p.conns <- proxyConn
			}
		}(proxyConn)
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
	select {
	case conn := <-p.conns:
		return conn, nil
	case err := <-p.errs:
		return nil, err
	}
}

// Close closes the listener.
// Any blocked Accept operations will be unblocked and return errors.
func (p *proxyListenerRouter) Close() error {
	close(p.closed)
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
