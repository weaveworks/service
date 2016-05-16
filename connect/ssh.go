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

type Endpoint struct {
	Host string
	Port int
}

func (endpoint *Endpoint) String() string {
	return fmt.Sprintf("%s:%d", endpoint.Host, endpoint.Port)
}

type SSHtunnel struct {
	Local    *Endpoint
	Server   *Endpoint
	Remote   *Endpoint
	Config   *ssh.ClientConfig
	listener net.Listener
}

func (tunnel *SSHtunnel) Start(ready chan bool) (err error) {
	log.Println("Started processing tunnel traffic")
	tunnel.listener, err = net.Listen("tcp", tunnel.Local.String())
	if err != nil {
		return
	}

	ready <- true

	for {
		conn, err := tunnel.listener.Accept()
		if err != nil {
			return err
		}
		go tunnel.forward(conn)
	}
}

func (tunnel *SSHtunnel) Stop() (err error) {
	err = tunnel.listener.Close()
	if err != nil {
		log.Println(err)
		return
	}
	log.Println("Tunnel closed")
	return
}

func (tunnel *SSHtunnel) forward(localConn net.Conn) {
	log.Printf("Forwarding connection: %v\n", localConn)
	serverConn, err := ssh.Dial("tcp", tunnel.Server.String(), tunnel.Config)
	if err != nil {
		log.Printf("Server dial error: %s\n", err)
		return
	}

	remoteConn, err := serverConn.Dial("tcp", tunnel.Remote.String())
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

func makeSshConfig(user, privateKeyPath string) (*ssh.ClientConfig, error) {
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
