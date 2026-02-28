package clamd

import (
	"bufio"
	"net"
	"os"
	"sync"
	"syscall"
	"time"

	"git.vsu.ru/ai/drweb-cgp/config"
	"git.vsu.ru/ai/drweb-cgp/utils"
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
	raw       []byte
}

const (
	chunkSize   = 1024
	connTimeout = 5 * time.Second
	bufSize     = 4096
)

var (
	respOK    = "OK"
	respFOUND = "FOUND"
	respERROR = "ERROR"

	cmdINSTREAM = []byte("nINSTREAM\n")
	cmdFILDES   = []byte("nFILDES\n")
	termStream  = []byte{0, 0, 0, 0}

	dialer = &net.Dialer{Timeout: connTimeout}

	readerPool = sync.Pool{
		New: func() any { return bufio.NewReaderSize(nil, bufSize) },
	}
	writerPool = sync.Pool{
		New: func() any { return bufio.NewWriterSize(nil, bufSize) },
	}
)

func Fildes(h *os.File) ([]Response, error) {
	conn, err := dialer.Dial(config.Proto(), config.Host())
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(config.Timeout()))

	raw, err := conn.(*net.UnixConn).SyscallConn()
	if err != nil {
		return nil, err
	}

	var sendErr error
	rights := syscall.UnixRights(int(h.Fd()))

	err = raw.Control(func(fd uintptr) {
		sendErr = syscall.Sendmsg(int(fd), cmdFILDES, nil, nil, 0)
		if sendErr != nil {
			return
		}
		sendErr = syscall.Sendmsg(int(fd), nil, rights, nil, 0)
	})
	if err != nil {
		return nil, err
	}
	if sendErr != nil {
		return nil, sendErr
	}

	return processResponse(conn)
}

func InStream(h *os.File) ([]Response, error) {
	conn, err := dialer.Dial(config.Proto(), config.Host())
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(config.Timeout()))

	if err := streamScan(conn, h); err != nil {
		return nil, err
	}

	return processResponse(conn)
}

func (r *Response) Raw() string {
	return utils.Bytes2string(r.raw)
}
