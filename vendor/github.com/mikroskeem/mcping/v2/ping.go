//Package mcping facilitates the pinging of Minecraft servers using the 1.7+ protocol.
package mcping

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"net"
	"strconv"
	"time"
)

const (
	//DefaultTimeout stores default ping timeout
	DefaultTimeout = time.Second
)

var (
	dialer = net.Dialer{}
)

type pingOptions struct {
	ctx        context.Context
	addr       string
	serverAddr string

	readTimeout  uint
	writeTimeout uint
}

type PingOpt func(*pingOptions)

// WithServerAddress sets a custom server address for handshake packet
func WithServerAddress(serverAddr string) PingOpt {
	return func(po *pingOptions) {
		po.serverAddr = serverAddr
	}
}

func WithReadTimeout(timeout uint) PingOpt {
	return func(po *pingOptions) {
		po.readTimeout = timeout
	}
}

func WithWriteTimeout(timeout uint) PingOpt {
	return func(po *pingOptions) {
		po.writeTimeout = timeout
	}
}

func WithReadWriteTimeout(rtimeout, wtimeout uint) PingOpt {
	return func(po *pingOptions) {
		po.readTimeout = rtimeout
		po.writeTimeout = wtimeout
	}
}

// DoPing pings with specified options
func Ping(ctx context.Context, addr string, opts ...PingOpt) (PingResponse, error) {
	pingOpts := pingOptions{ctx, addr, addr, 1500, 1500}
	for _, opt := range opts {
		opt(&pingOpts)
	}
	return ping(pingOpts)
}

func ping(options pingOptions) (resp PingResponse, err error) {
	var host string
	var port uint16

	addr := options.addr
	spoofedServerAddr := options.serverAddr

	if spoofedServerAddr == "" {
		spoofedServerAddr = addr
	}
	if hostname, rawPort, err := net.SplitHostPort(spoofedServerAddr); err != nil {
		return resp, err
	} else {
		host = hostname
		if intport, err := strconv.Atoi(rawPort); err == nil {
			port = uint16(intport)
		} else {
			return resp, err
		}
	}

	// Start timer
	timer := pingTimer{}
	timer.Start()

	// Connect
	conn, err := dialer.DialContext(options.ctx, "tcp", addr)
	if err != nil {
		return resp, err
	}
	defer conn.Close()

	connReader := bufio.NewReader(conn)

	var dataBuf bytes.Buffer
	var finBuf bytes.Buffer

	_, _ = dataBuf.Write([]byte("\x00")) // Packet ID
	_, _ = dataBuf.Write([]byte("\x6D")) // 1.9 protocol

	// Write host string length + host
	hostLength := uint8(len(host))
	_, _ = dataBuf.Write([]uint8{hostLength})
	_, _ = dataBuf.Write([]byte(host))

	// Write port
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, port)
	_, _ = dataBuf.Write(b)

	// Next state ping
	_, _ = dataBuf.Write([]byte("\x01"))

	// Prepend packet length with data
	packetLength := []byte{uint8(dataBuf.Len())}
	_, _ = finBuf.Write(append(packetLength, dataBuf.Bytes()...))

	if err = conn.SetWriteDeadline(time.Now().Add(time.Duration(options.writeTimeout) * time.Millisecond)); err != nil {
		return
	}

	// Sending handshake
	if _, err = conn.Write(finBuf.Bytes()); err != nil {
		return
	}
	// Status ping
	if _, err = conn.Write([]byte("\x01\x00")); err != nil {
		return
	}

	if err = conn.SetReadDeadline(time.Now().Add(time.Duration(options.readTimeout) * time.Millisecond)); err != nil {
		return
	}

	// Get situationally useless full byte length
	if _, err = binary.ReadUvarint(connReader); err != nil {
		return
	}

	// Packet type 0 means we're good to receive ping
	if packetType, err := connReader.ReadByte(); err != nil {
		return resp, err
	} else if !bytes.Equal([]byte{packetType}, []byte("\x00")) {
		return resp, ErrPacketType
	}

	// Get data length via Varint
	length, err := binary.ReadUvarint(connReader)
	if err != nil {
		return resp, err
	}
	if length < 10 {
		return resp, ErrSmallPacket
	} else if length > 700000 {
		return resp, ErrBigPacket
	}

	// Recieve json buffer
	bytesRecieved := uint64(0)
	recBytes := make([]byte, length)
	for bytesRecieved < length {
		n, err := connReader.Read(recBytes[bytesRecieved:length])
		if err != nil {
			return resp, err
		}
		bytesRecieved = bytesRecieved + uint64(n)
	}

	// Stop Timer, collect latency
	latency := timer.End()

	// Decode buffer
	if err = json.Unmarshal(recBytes, &resp); err != nil {
		return
	}
	resp.Latency = uint(latency)

	return
}
