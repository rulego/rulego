/*
 * Copyright 2024 The RuleGo Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package test

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// MqttBroker is a minimal in-process MQTT 3.1.1 broker for tests.
// Supports CONNECT/CONNACK, SUBSCRIBE/SUBACK, UNSUBSCRIBE/UNSUBACK,
// bidirectional QoS0 PUBLISH, PINGREQ/PINGRESP, and DISCONNECT.
// Use NewMqttBroker/Close to start and stop it, simulating broker deploy and outage.
type MqttBroker struct {
	listener net.Listener
	mu       sync.Mutex
	subs     map[string]map[*brokerConn]byte // topic filter -> subscribers and their qos
	conns    map[*brokerConn]struct{}
	closed   int32
	// downUntil, when non-zero, marks a simulated outage window: new connections
	// are rejected until this time, then service resumes.
	downUntil time.Time
}

type brokerConn struct {
	conn net.Conn
	wmu  sync.Mutex // serialize writes on the same connection
}

// NewMqttBroker starts a test broker on addr; ":0" lets the OS pick a port.
func NewMqttBroker(addr string) (*MqttBroker, error) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	b := &MqttBroker{
		listener: l,
		subs:     make(map[string]map[*brokerConn]byte),
		conns:    make(map[*brokerConn]struct{}),
	}
	go b.acceptLoop()
	return b, nil
}

// Addr returns the broker listen address with the real port.
func (b *MqttBroker) Addr() string { return b.listener.Addr().String() }

// Close stops the broker and drops all connections, simulating a broker outage.
func (b *MqttBroker) Close() {
	if !atomic.CompareAndSwapInt32(&b.closed, 0, 1) {
		return
	}
	_ = b.listener.Close()
	b.mu.Lock()
	for c := range b.conns {
		_ = c.conn.Close()
	}
	b.conns = make(map[*brokerConn]struct{})
	b.subs = make(map[string]map[*brokerConn]byte)
	b.mu.Unlock()
}

// DisconnectAll drops all client connections and clears subscriptions, keeping
// the listen port. Simulates a transient drop: clients reconnect almost
// immediately, so the connection status only briefly touches Reconnecting.
// For a stable reconnecting window, use SimulateOutage.
func (b *MqttBroker) DisconnectAll() {
	b.mu.Lock()
	for c := range b.conns {
		_ = c.conn.Close()
	}
	b.conns = make(map[*brokerConn]struct{})
	b.subs = make(map[string]map[*brokerConn]byte)
	b.mu.Unlock()
}

// SimulateOutage drops all connections and rejects new ones for duration d, then
// resumes. Clients stay in a stable Reconnecting state during the window and
// recover to Connected afterwards. Suitable for disconnect/reconnect tests.
func (b *MqttBroker) SimulateOutage(d time.Duration) {
	b.mu.Lock()
	for c := range b.conns {
		_ = c.conn.Close()
	}
	b.conns = make(map[*brokerConn]struct{})
	b.subs = make(map[string]map[*brokerConn]byte)
	b.downUntil = time.Now().Add(d)
	b.mu.Unlock()
}

// Subscriptions returns the active topic filters, for tests to assert subscriptions are established.
func (b *MqttBroker) Subscriptions() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	topics := make([]string, 0, len(b.subs))
	for t := range b.subs {
		topics = append(topics, t)
	}
	return topics
}

func (b *MqttBroker) acceptLoop() {
	for {
		conn, err := b.listener.Accept()
		if err != nil {
			return
		}
		// During an outage window, reject new connections immediately.
		b.mu.Lock()
		down := !b.downUntil.IsZero() && time.Now().Before(b.downUntil)
		b.mu.Unlock()
		if down {
			_ = conn.Close()
			continue
		}
		c := &brokerConn{conn: conn}
		b.mu.Lock()
		b.conns[c] = struct{}{}
		b.mu.Unlock()
		go b.handleConn(c)
	}
}

func (b *MqttBroker) handleConn(c *brokerConn) {
	defer func() {
		_ = c.conn.Close()
		b.removeConn(c)
	}()
	reader := bufio.NewReader(c.conn)
	for {
		pktType, body, err := readPacket(reader)
		if err != nil {
			return
		}
		switch pktType {
		case 1: // CONNECT -> CONNACK(rc=0)
			_ = b.writePacket(c, 0x20, []byte{0x00, 0x00})
		case 3: // PUBLISH (QoS0 only)
			topic, payload, ok := parsePublish(body)
			if ok {
				b.deliver(c, topic, payload)
			}
		case 8: // SUBSCRIBE -> SUBACK
			if len(body) < 2 {
				return
			}
			pid := body[:2]
			granted := b.parseSubscribe(c, body[2:])
			_ = b.writePacket(c, 0x90, append(append([]byte{}, pid...), granted...))
		case 10: // UNSUBSCRIBE -> UNSUBACK
			if len(body) < 2 {
				return
			}
			pid := body[:2]
			b.parseUnsubscribe(c, body[2:])
			_ = b.writePacket(c, 0xB0, pid)
		case 12: // PINGREQ -> PINGRESP
			_ = b.writePacket(c, 0xD0, nil)
		case 14: // DISCONNECT
			return
		}
	}
}

func (b *MqttBroker) writePacket(c *brokerConn, firstByte byte, body []byte) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	buf := make([]byte, 0, 1+len(body)+4)
	buf = append(buf, firstByte)
	buf = append(buf, encodeRemainingLength(len(body))...)
	buf = append(buf, body...)
	_, err := c.conn.Write(buf)
	return err
}

func (b *MqttBroker) parseSubscribe(c *brokerConn, body []byte) []byte {
	var granted []byte
	for len(body) >= 3 {
		topicLen := int(binary.BigEndian.Uint16(body[:2]))
		if len(body) < 2+topicLen+1 {
			break
		}
		topic := string(body[2 : 2+topicLen])
		qos := body[2+topicLen]
		body = body[2+topicLen+1:]
		b.mu.Lock()
		subs := b.subs[topic]
		if subs == nil {
			subs = make(map[*brokerConn]byte)
			b.subs[topic] = subs
		}
		subs[c] = qos
		b.mu.Unlock()
		granted = append(granted, qos)
	}
	return granted
}

func (b *MqttBroker) parseUnsubscribe(c *brokerConn, body []byte) {
	for len(body) >= 2 {
		topicLen := int(binary.BigEndian.Uint16(body[:2]))
		if len(body) < 2+topicLen {
			break
		}
		topic := string(body[2 : 2+topicLen])
		body = body[2+topicLen:]
		b.mu.Lock()
		if subs := b.subs[topic]; subs != nil {
			delete(subs, c)
			if len(subs) == 0 {
				delete(b.subs, topic)
			}
		}
		b.mu.Unlock()
	}
}

func (b *MqttBroker) removeConn(c *brokerConn) {
	b.mu.Lock()
	delete(b.conns, c)
	for topic, subs := range b.subs {
		delete(subs, c)
		if len(subs) == 0 {
			delete(b.subs, topic)
		}
	}
	b.mu.Unlock()
}

func (b *MqttBroker) deliver(_ *brokerConn, topic string, payload []byte) {
	b.mu.Lock()
	var targets []*brokerConn
	for filter, subs := range b.subs {
		if topicMatch(filter, topic) {
			for c := range subs {
				targets = append(targets, c)
			}
		}
	}
	b.mu.Unlock()

	body := make([]byte, 0, 2+len(topic)+len(payload))
	var tlen [2]byte
	binary.BigEndian.PutUint16(tlen[:], uint16(len(topic)))
	body = append(body, tlen[:]...)
	body = append(body, topic...)
	body = append(body, payload...)
	for _, c := range targets {
		_ = b.writePacket(c, 0x30, body)
	}
}

// topicMatch matches MQTT topic wildcards (+ single level, # multi level).
func topicMatch(filter, topic string) bool {
	if filter == topic {
		return true
	}
	fs := strings.Split(filter, "/")
	ts := strings.Split(topic, "/")
	for i, f := range fs {
		if f == "#" {
			return true
		}
		if i >= len(ts) {
			return false
		}
		if f != "+" && f != ts[i] {
			return false
		}
	}
	return len(fs) == len(ts)
}

func readPacket(r *bufio.Reader) (byte, []byte, error) {
	b1, err := r.ReadByte()
	if err != nil {
		return 0, nil, err
	}
	mult := 1
	remLen := 0
	for {
		b, err := r.ReadByte()
		if err != nil {
			return 0, nil, err
		}
		remLen += int(b&0x7F) * mult
		if b&0x80 == 0 {
			break
		}
		mult *= 128
		if mult > 128*128*128 {
			return 0, nil, fmt.Errorf("malformed remaining length")
		}
	}
	body := make([]byte, remLen)
	if _, err := io.ReadFull(r, body); err != nil {
		return 0, nil, err
	}
	return b1 >> 4, body, nil
}

func encodeRemainingLength(n int) []byte {
	var out []byte
	for {
		d := byte(n % 128)
		n /= 128
		if n > 0 {
			d |= 0x80
		}
		out = append(out, d)
		if n == 0 {
			return out
		}
	}
}

func parsePublish(body []byte) (string, []byte, bool) {
	if len(body) < 2 {
		return "", nil, false
	}
	topicLen := int(binary.BigEndian.Uint16(body[:2]))
	if len(body) < 2+topicLen {
		return "", nil, false
	}
	topic := string(body[2 : 2+topicLen])
	payload := body[2+topicLen:]
	return topic, payload, true
}
