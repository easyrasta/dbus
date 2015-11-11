//+build !windows
package dbus

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strconv"
)

type oobTReader struct {
	conn *net.TCPConn
	oob  []byte
	buf  [4096]byte
}

func (o *oobTReader) Read(b []byte) (n int, err error) {
	n, err = o.conn.Read(b)
	return n, err
}

type TCPTransport struct {
	*net.TCPConn
	hasUnixFDs bool
}

func newTCPTransport(keys string) (transport, error) {
	t := new(TCPTransport)
	host := getKey(keys, "host")
	port := getKey(keys, "port")
	switch {
	case host != "" && port != "":
		hostParsed, _, err := net.ParseCIDR(host)
		if err != nil {
			return nil, err
		}
		portParsed, err := strconv.Atoi(port)
		if err != nil {
			return nil, err
		}
		t.TCPConn, err = net.DialTCP("tcp", nil, &net.TCPAddr{IP: hostParsed,Port: portParsed, Zone: ""})
		if err != nil {
			return nil, err
		}
		return t, nil
	default:
		return nil, errors.New("dbus: invalid address (host or port non set.)")
	}
}

func init() {
	transports["tcp"] = newTCPTransport
}

func (t *TCPTransport) SendNullByte() error {
	_, err := t.Write([]byte{0})
	return err
}

func (t *TCPTransport) EnableUnixFDs() {
	t.hasUnixFDs = false
}

func (t *TCPTransport) ReadMessage() (*Message, error) {
	var (
		blen, hlen uint32
		csheader   [16]byte
		headers    []header
		order      binary.ByteOrder
	)
	// To be sure that all bytes of out-of-band data are read, we use a special
	// reader that uses ReadUnix on the underlying connection instead of Read
	// and gathers the out-of-band data in a buffer.
	rd := &oobTReader{conn: t.TCPConn}
	// read the first 16 bytes (the part of the header that has a constant size),
	// from which we can figure out the length of the rest of the message
	if _, err := io.ReadFull(rd, csheader[:]); err != nil {
		return nil, err
	}
	switch csheader[0] {
	case 'l':
		order = binary.LittleEndian
	case 'B':
		order = binary.BigEndian
	default:
		return nil, InvalidMessageError("invalid byte order")
	}
	// csheader[4:8] -> length of message body, csheader[12:16] -> length of
	// header fields (without alignment)
	binary.Read(bytes.NewBuffer(csheader[4:8]), order, &blen)
	binary.Read(bytes.NewBuffer(csheader[12:]), order, &hlen)
	if hlen%8 != 0 {
		hlen += 8 - (hlen % 8)
	}

	// decode headers and look for unix fds
	headerdata := make([]byte, hlen+4)
	copy(headerdata, csheader[12:])
	if _, err := io.ReadFull(t, headerdata[4:]); err != nil {
		return nil, err
	}
	dec := newDecoder(bytes.NewBuffer(headerdata), order)
	dec.pos = 12
	vs, err := dec.Decode(Signature{"a(yv)"})
	if err != nil {
		return nil, err
	}
	all := make([]byte, 16+hlen+blen)
	copy(all, csheader[:])
	copy(all[16:], headerdata[4:])
	if _, err := io.ReadFull(rd, all[16+hlen:]); err != nil {
		return nil, err
	}
	return DecodeMessage(bytes.NewBuffer(all))
}

func (t *TCPTransport) SendMessage(msg *Message) error {
	if err := msg.EncodeTo(t, binary.LittleEndian); err != nil {
		return err
	}
	return nil
}

func (t *TCPTransport) SupportsUnixFDs() bool {
	return false
}
