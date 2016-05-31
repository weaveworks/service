package main

import (
	log "github.com/Sirupsen/logrus"
	"io"
	"io/ioutil"
	"net"

	"golang.org/x/crypto/ssh"
)

// based on http://blog.ralch.com/tutorial/golang-ssh-tunneling/

type tunnel struct {
	Local    string
	Server   string
	Remote   string
	Config   *ssh.ClientConfig
	Listener net.Listener
}

func (t *tunnel) start() error {
	for {
		conn, err := t.Listener.Accept()
		if err != nil {
			return err
		}
		go t.forward(conn)
	}
}

func (t *tunnel) stop() error {
	return nil
}

func (t *tunnel) forward(localConn net.Conn) {
	log.Printf("Forwarding connection: %v", localConn)
	serverConn, err := ssh.Dial("tcp", t.Server, t.Config)
	if err != nil {
		log.Printf("Server dial error: %s", err)
		return
	}

	remoteConn, err := serverConn.Dial("tcp", t.Remote)
	if err != nil {
		log.Printf("Remote dial error: %s", err)
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
