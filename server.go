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
	"time"
)

const (
	address = "224.003.003.003"
)

type (
	Generator interface {
		New(string) io.Closer
	}

	Discover interface {
		Loop(time.Duration) error
		Close() error
		Add(string, string)
		Remove(string)
		Handle(string, Generator)
		Unhandle(string)
	}

	ping chan bool

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
		broadcast map[string]string
		handle    map[string]Generator
		alive     map[string]ping
		aliveMsg  chan idip
		timeout   time.Duration
		token     []byte
		address   *net.UDPAddr
	}
)

func NewDiscovery(network, token string, timeout time.Duration) (Discover, error) {
	discover := &discoverer{
		done:      make(chan interface{}),
		aliveMsg:  make(chan idip),
		broadcast: map[string]string{},
		handle:    map[string]Generator{},
		alive:     map[string]ping{},
		timeout:   timeout,
		token:     []byte(token),
	}
	var err error

	discover.address = &net.UDPAddr{
		IP:   net.ParseIP(address),
		Port: 5432,
	}

	iface, err := net.InterfaceByName(network)
	if err != nil {
		return nil, err
	}
	discover.conn, err = net.ListenMulticastUDP("udp4", iface, discover.address)
	if err != nil {
		return nil, err
	}

	return discover, nil
}

func (d *discoverer) Add(key, value string) {
	// really should validate the ip address (value)....
	d.broadcast[key] = value
}

func (d *discoverer) Remove(key string) {
	delete(d.broadcast, key)
}

func (d *discoverer) Handle(key string, gen Generator) {
	d.handle[key] = gen
}

func (d *discoverer) Unhandle(key string) {
	// I should really close already created handlers
	delete(d.handle, key)
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

	go d.receiveBroadcasts(d.conn)
	for {
		select {
		case <-ticker.C:
			if err := d.doBroadcast(); err != nil {
				return err
			}
		case <-d.done:
			return nil
		case msg := <-d.aliveMsg:

			select {
			case d.alive[msg.id] <- true:
			default:
				gen, ok := d.handle[msg.id]
				if !ok {
					continue
				}
				remote := make(ping, 1)
				d.alive[msg.id] = remote
				go remote.wait(d.timeout, gen.New(msg.ip))
			}
		}
	}
}

func (d *discoverer) receiveBroadcasts(conn *net.UDPConn) {
	buf := make([]byte, 65536)
	for {

		n, err := conn.Read(buf)
		if err != nil {
			return
		}

		// 44 is ','
		elems := bytes.Split(buf[:n], []byte{44})
		if !bytes.Equal(elems[0], d.token) {
			continue
		}

		// pull the token off the front
		elems = elems[1:]

		for _, elem := range elems {
			// 64 is '@'
			parts := bytes.SplitN(elem, []byte{64}, 2)
			if len(parts) != 2 {
				continue
			}

			packet := idip{
				id: string(parts[0]),
				ip: string(parts[1]),
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
	buffer := bytes.NewBuffer(d.token)
	for key, value := range d.broadcast {
		buffer.Write([]byte(fmt.Sprintf(",%v@%v", key, value)))
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
