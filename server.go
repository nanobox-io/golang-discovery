// Copyright (c) 2015 Pagoda Box Inc
//
// This Source Code Form is subject to the terms of the Mozilla Public License, v.
// 2.0. If a copy of the MPL was not distributed with this file, You can obtain one
// at http://mozilla.org/MPL/2.0/.
//

package discovery

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

const (
	address = "224.003.003.003"
)

type (
	Generator interface {
		New() io.Closer
	}

	Discover interface {
		Loop() error
		Close() error
		Add(string, string, Generator)
		Remove(string)
	}

	pong chan bool
	ping chan ping

	idip struct {
		id string
		ip string
	}

	generator struct {
		value     string
		generator Generator
	}

	discoverer struct {
		conn      *net.UDPConn
		done      chan interface{}
		broadcast map[string]generator
		alive     map[string]ping
		aliveMsg  chan idip
		timeout   time.Duration
		token     string
		address   *net.UDPAddr
	}
)

func NewDiscovery(port int, network, token string, timeout time.Duration) (Discover, error) {
	discover := &discoverer{
		done:      make(chan interface{}),
		aliveMsg:  make(chan string),
		broadcast: map[string]string{},
		alive:     map[string]ping{},
		generator: gen,
		timeout:   timeout,
		token:     "token",
	}
	var err error

	d.address = &net.UDPAddr{
		IP:   net.ParseIP(address),
		Port: port,
	}

	discover.conn, err = net.ListenMulticastUDP("udp4", network, d.address)
	if err != nil {
		return err
	}

	return discoverer
}

func (d *discoverer) Add(key, value string, gen Generator) {
	// really should validate the ip address (value)....
	d.broadcast[key] = generator{
		value:     value,
		generator: gen,
	}
}

func (d *discoverer) Remove(key string) {
	delete(d.broadcast, key)
}

func (d *discoverer) Close() error {
	close(d.done)
	d.conn.Close()
	// is there someway to shut all of the generated go procs off?
	return nil
}

func (d *discoverer) Loop(every time.Duration) error {
	ticker := time.NewTicker(every)
	defer ticker.Stop()

	go d.receiveBroadcasts()
	for {
		select {
		case <-ticker.C:
			if err := d.doBroadcast(); err != nil {
				return err
			}
		case <-d.done:
			return
		case msg := <-d.aliveMsg:
			remote, ok := d.alive[msg.id]
			select {
			case remote <- true:
			default:
				remote := make(ping, 1)
				d.alive[msg.id] = remote
				go remote.wait(d.timeout, d.generator.New(msg.id, msg.address))
			}
		}
	}
}

func (d *discoverer) receiveBroadcasts() {
	conn := d.conn
	buf := make([]byte, 65536)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			return
		}

		// 44 is ','
		elems := bytes.Split(buf[:n], byte(44))
		if elems[0] != d.token {
			continue
		}

		// pull the token off the front
		elems = elems[1:]

		for elem := range elems {
			// 64 is '@'
			parts := bytes.SplitN(elem, byte(64), 2)
			if len(parts) != 2 {
				continue
			}

			packet := idip{
				id: parts[0],
				ip: parts[1],
			}

			select {
			case d.aliveMsg <- packet:
			case <-d.done:
				return
			}
		}
	}
}

func (d *discoverer) makePacket() []byte {
	buffer := bytes.NewBuffer(string(d.token))
	for key, value := range d.broadcast {
		buffer.Write(fmt.Sprintf(",%v@%v", key, value))
	}

	return buffer.Bytes()
}

func (d *discoverer) doBroadcast() error {
	b := d.makePacket()
	_, error := d.conn.WriteToUDP(b, d.address)
	return error
}

func (p ping) wait(duration time.Duration, done io.Closer) {
	defer done.Close()
	for {
		select {
		case <-p:
		case <-time.After(duration):
			return
		}
	}
}
