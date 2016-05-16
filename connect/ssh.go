package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"

	"golang.org/x/crypto/ssh"
)

// based on http://blog.ralch.com/tutorial/golang-ssh-tunneling/

type endpoint struct {
	Host string
	Port int
}

func (endpoint *endpoint) String() string {
	return fmt.Sprintf("%s:%d", endpoint.Host, endpoint.Port)
}

type tunnel struct {
	Local    *endpoint
	Server   *endpoint
	Remote   *endpoint
	Config   *ssh.ClientConfig
	listener net.Listener
}

func (t *tunnel) start(ready chan bool) (err error) {
	log.Println("Started processing tunnel traffic")
	t.listener, err = net.Listen("tcp", t.Local.String())
	if err != nil {
		return
	}

	ready <- true

	for {
		conn, err := t.listener.Accept()
		if err != nil {
			return err
		}
		go t.forward(conn)
	}
}

func (t *tunnel) stop() (err error) {
	err = t.listener.Close()
	if err != nil {
		log.Println(err)
		return
	}
	log.Println("Tunnel closed")
	return
}

func (t *tunnel) forward(localConn net.Conn) {
	log.Printf("Forwarding connection: %v\n", localConn)
	serverConn, err := ssh.Dial("tcp", t.Server.String(), t.Config)
	if err != nil {
		log.Printf("Server dial error: %s\n", err)
		return
	}

	remoteConn, err := serverConn.Dial("tcp", t.Remote.String())
	if err != nil {
		log.Printf("Remote dial error: %s\n", err)
		return
	}

	copyConn := func(writer, reader net.Conn) {
		s, err := io.Copy(writer, reader)
		if err != nil {
			log.Printf("I/O error: %s", err)
		}
		log.Printf("Copied %d bytes for connection %v", s, reader)
	}

	go copyConn(localConn, remoteConn)
	go copyConn(remoteConn, localConn)
}

func makeSSHConfig(user, privateKeyPath string) (*ssh.ClientConfig, error) {
	buff, err := ioutil.ReadFile(privateKeyPath)
	if err != nil {
		return nil, err
	}
	key, err := ssh.ParsePrivateKey(buff)
	if err != nil {
		return nil, err
	}

	config := ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(key),
		},
	}

	return &config, nil
}
