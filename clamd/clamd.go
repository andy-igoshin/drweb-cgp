package clamd

import (
	"encoding/binary"
	"io"
	"net"
	"net/textproto"
	"os"
	"strings"
	"syscall"
	"time"

	"git.vsu.ru/ai/drweb-cgp/config"
)

type Status int

const (
	Unknown Status = iota
	Ok
	Found
	Error
)

type Response struct {
	Filename  string
	Signature string
	Status    Status
	Raw       string
}

const (
	chunkSize   = 1024
	connTimeout = 5 * time.Second
)

var dialer *net.Dialer

func init() {
	dialer = &net.Dialer{
		Timeout: connTimeout,
	}
}

func Fildes(h *os.File) (r []Response, err error) {

	var conn net.Conn

	conn, err = dialer.Dial(config.Transport(), config.Host())
	if err != nil {
		return
	}

	tc := textproto.NewConn(conn)
	defer tc.Close()

	id := tc.Next()
	tc.StartRequest(id)

	conn.SetDeadline(time.Now().Add(config.Timeout()))

	err = fildesScan(tc, conn, h)
	if err != nil {
		tc.EndRequest(id)
		return
	}

	tc.W.Flush()
	tc.EndRequest(id)

	tc.StartResponse(id)
	defer tc.EndResponse(id)

	r, err = processResponse(tc)

	return
}

func InStream(h *os.File) (r []Response, err error) {

	var conn net.Conn

	conn, err = dialer.Dial(config.Transport(), config.Host())
	if err != nil {
		return
	}

	tc := textproto.NewConn(conn)
	defer tc.Close()

	id := tc.Next()
	tc.StartRequest(id)

	conn.SetDeadline(time.Now().Add(config.Timeout()))

	err = streamScan(tc, h)
	if err != nil {
		tc.EndRequest(id)
		return
	}

	tc.W.Flush()
	tc.EndRequest(id)

	tc.StartResponse(id)
	defer tc.EndResponse(id)

	r, err = processResponse(tc)

	return
}

func fildesScan(tc *textproto.Conn, conn net.Conn, h *os.File) error {

	if _, err := tc.W.Write([]byte("nFILDES\n")); err != nil {
		return err
	}
	tc.W.Flush()

	rc, err := conn.(*net.UnixConn).SyscallConn()
	if err != nil {
		return err
	}

	rights := syscall.UnixRights(int(h.Fd()))

	var sendErr error
	err = rc.Control(func(fd uintptr) {
		sendErr = syscall.Sendmsg(int(fd), nil, rights, nil, 0)
	})
	if err != nil {
		return err
	}
	return sendErr
}

func processResponse(tc *textproto.Conn) (r []Response, err error) {

	var line []byte

	for {
		res := Response{}

		if line, err = tc.R.ReadSlice('\n'); err != nil {
			if err == io.EOF {
				err = nil
			}
			break
		}

		res.Raw = strings.TrimRight(string(line), "\n")
		rs := strings.Split(res.Raw, " ")

		switch rs[len(rs)-1] {
		case "OK":
			res.Filename = rs[0]
			res.Status = Ok

		case "FOUND":
			res.Filename = rs[0]
			res.Signature = rs[1]
			res.Status = Found

		case "ERROR":
			res.Filename = rs[0]
			res.Status = Error

		default:
			res.Status = Unknown
		}

		r = append(r, res)
	}

	return
}

func streamScan(tc *textproto.Conn, h *os.File) (err error) {

	var n int
	var eof bool

	_, err = tc.W.Write([]byte("nINSTREAM\n"))
	if err != nil {
		return
	}

	b := make([]byte, 4)

	for !eof {

		buf := make([]byte, chunkSize)

		if n, err = h.Read(buf); err != nil {
			if err != io.EOF {
				return
			}

			err = nil
			eof = true
		}

		if n > 0 {
			binary.BigEndian.PutUint32(b, uint32(n))
			if _, err = tc.W.Write(b); err != nil {
				return
			}

			if _, err = tc.W.Write(buf[0:n]); err != nil {
				return
			}
			tc.W.Flush()
		}
	}

	if _, err = tc.W.Write([]byte{0, 0, 0, 0}); err != nil {
		return
	}
	tc.W.Flush()

	return
}
