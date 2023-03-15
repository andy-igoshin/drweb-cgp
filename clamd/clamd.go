package clamd

import (
	"encoding/binary"
	"fmt"
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
	Msg       string
	Raw       string
}

const (
	chunkSize   = 1024
	cmdTimeout  = 1 * time.Minute
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

	conn.SetDeadline(time.Now().Add(cmdTimeout))

	err = fildesScan(tc, conn, h)
	if err != nil {
		tc.EndRequest(id)
		return
	}

	tc.W.Flush()
	tc.EndRequest(id)

	tc.StartResponse(id)
	defer tc.EndResponse(id)

	conn.SetReadDeadline(time.Now().Add(config.Timeout()))

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

	conn.SetDeadline(time.Now().Add(cmdTimeout))

	err = streamScan(tc, h)
	if err != nil {
		tc.EndRequest(id)
		return
	}

	tc.W.Flush()
	tc.EndRequest(id)

	tc.StartResponse(id)
	defer tc.EndResponse(id)

	conn.SetReadDeadline(time.Now().Add(config.Timeout()))

	r, err = processResponse(tc)

	return
}

func fildesScan(tc *textproto.Conn, conn net.Conn, h *os.File) (err error) {

	var sf *os.File

	_, err = tc.W.Write([]byte("nFILDES\n"))
	if err != nil {
		return
	}
	tc.W.Flush()

	s := conn.(*net.UnixConn)
	if sf, err = s.File(); err != nil {
		return
	}
	defer sf.Close()

	sock := int(sf.Fd())
	fds := []int{int(h.Fd())}
	rights := syscall.UnixRights(fds...)
	err = syscall.Sendmsg(sock, nil, rights, nil, 0)
	if err != nil {
		return
	}

	return
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
			res.Msg = strings.Join(rs[0:len(rs)-1], " ")

		default:
			res.Status = Unknown
			err = fmt.Errorf("Invalid server response: %s", res.Raw)
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
