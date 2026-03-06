package clamd

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"os"
	"sync"

	"git.vsu.ru/ai/drweb-cgp/utils"
)

const (
	respOK    = "OK"
	respFOUND = "FOUND"
	respERROR = "ERROR"
)

var (
	cmdINSTREAM = []byte("nINSTREAM\n")
	cmdFILDES   = []byte("nFILDES\n")
	termStream  = []byte{0, 0, 0, 0}
)

var dataPool = sync.Pool{
    New: func() any {
        b := make([]byte, bufSize)
        return &b
    },
}

func processResponse(conn net.Conn) (r []Response, err error) {
	br := readerPool.Get().(*bufio.Reader)
	br.Reset(conn)
	defer putReader(br)

	for {
		line, err := br.ReadSlice('\n')
		if err != nil {
			if err == io.EOF {
				return r, nil
			}
			return nil, err
		}

		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		raw := make([]byte, len(line))
		copy(raw, line)
		res := Response{raw: raw}
		rs := bytes.Fields(res.raw)
		lastIdx := len(rs) - 1

		if lastIdx < 0 {
			continue
		}

		switch utils.Bytes2string(rs[lastIdx]) {
		case respOK:
			res.Filename = utils.Bytes2string(rs[0])
			res.Status = Ok

		case respFOUND:
			res.Filename = utils.Bytes2string(rs[0])
			if len(rs) > 1 {
				res.Signature = utils.Bytes2string(rs[1])
			}
			res.Status = Found

		case respERROR:
			res.Filename = utils.Bytes2string(rs[0])
			res.Status = Error

		default:
			res.Status = Unknown
		}

		r = append(r, res)
	}
}

func putReader(br *bufio.Reader) {
	br.Reset(nil)
	readerPool.Put(br)
}

func putWriter(bw *bufio.Writer) {
	bw.Reset(nil)
	writerPool.Put(bw)
}

func streamScan(conn net.Conn, h *os.File) (err error) {
	w := writerPool.Get().(*bufio.Writer)
	w.Reset(conn)
	defer putWriter(w)

	bufPtr := dataPool.Get().(*[]byte)
	buf := *bufPtr
	defer dataPool.Put(bufPtr)

	if _, err = w.Write(cmdINSTREAM); err != nil {
		return
	}

	for {
		// Читаем 4092 байта, оставляя 4 под header
		var n int
		n, err = h.Read(buf[4:])
		if n > 0 {
			binary.BigEndian.PutUint32(buf[:4], uint32(n))
			// Весь блок (4+n) уходит в bufio. При n=4092 это 4096.
			if _, err = w.Write(buf[:4+n]); err != nil {
				return
			}
		}
		if err != nil {
			if err == io.EOF {
				err = nil
				break
			}
			return
		}
	}

	if _, err = w.Write(termStream); err != nil {
		return
	}

	return w.Flush()
}
