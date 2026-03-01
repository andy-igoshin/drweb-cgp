package clamd

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"os"

	"git.vsu.ru/ai/drweb-cgp/utils"
)

const (
	chunkSize = 1024

	respOK    = "OK"
	respFOUND = "FOUND"
	respERROR = "ERROR"
)

var (
	cmdINSTREAM = []byte("nINSTREAM\n")
	cmdFILDES   = []byte("nFILDES\n")
	termStream  = []byte{0, 0, 0, 0}
)

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

func streamScan(conn net.Conn, h *os.File) error {
	w := writerPool.Get().(*bufio.Writer)
	w.Reset(conn)
	defer putWriter(w)

	if _, err := w.Write(cmdINSTREAM); err != nil {
		return err
	}

	var buf [chunkSize]byte
	var hdr [4]byte

	for {
		n, err := h.Read(buf[:])
		if n > 0 {
			binary.BigEndian.PutUint32(hdr[:], uint32(n))
			w.Write(hdr[:])
			w.Write(buf[:n])
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
	}

	w.Write(termStream)
	return w.Flush()
}
