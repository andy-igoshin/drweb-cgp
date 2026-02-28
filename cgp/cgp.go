package cgp

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"unicode"
	"unicode/utf8"
)

const badFiles = "BadFiles"

var (
	protocol int
	stdoutFd int
	bRecvHdr = []byte("Received: ")
	bSeenHdr = []byte("X-DrWeb-Seen: ")
)

var bufferPool = sync.Pool{
	New: func() any { return bytes.NewBuffer(make([]byte, 0, 4096)) },
}

func AddHeader(seq int, headers []string) {
	buf := bufferPool.Get().(*bytes.Buffer)
	defer putBuffer(buf)

	buf.WriteString(strconv.Itoa(seq))
	buf.WriteString(" ADDHEADER \"")

	for i, h := range headers {
		replaceSpecCharsBuf(buf, h)
		if i < len(headers)-1 {
			buf.WriteString("\\e")
		}
	}

	if protocol >= 4 {
		buf.WriteString("\" OK")
	} else {
		buf.WriteString("\"")
	}

	res := buf.Bytes()
	length := len(res)

	if length > 4096 || (length == 4096 && res[length-1] != '\n') {
		Failure(seq, 0, fmt.Errorf("AddHeader: result exceeds 4k limit"))
		return
	}

	if length > 0 && res[length-1] != '\n' {
		buf.WriteByte('\n')
		res = buf.Bytes()
	}

	syscall.Write(stdoutFd, res)
}

func Discard(seq int) {
	var buf [64]byte
	b := strconv.AppendInt(buf[:0], int64(seq), 10)
	b = append(b, " DISCARD\n"...)
	syscall.Write(stdoutFd, b)
}

func Failure(seq, qid int, err error) {
	var buf [512]byte
	b := buf[:0]

	b = append(b, "* "...)
	b = strconv.AppendInt(b, int64(seq), 10)
	b = append(b, " ["...)
	b = strconv.AppendInt(b, int64(qid), 10)
	b = append(b, "]: "...)
	if err != nil {
		b = append(b, err.Error()...)
	} else {
		b = append(b, "unknown error"...)
	}

	length := len(b)
	if length > 0 && b[length-1] != '\n' {
		b = append(b, '\n')
	}
	syscall.Write(stdoutFd, b)

	b = buf[:0]
	b = strconv.AppendInt(b, int64(seq), 10)
	b = append(b, " FAILURE\n"...)
	syscall.Write(stdoutFd, b)
}

// InitStdoutFd инициализирует системный дескриптор для ответов серверу.
// Вызывать в самом начале main().
func InitStdoutFd() error {
	rawConn, err := os.Stdout.SyscallConn()
	if err != nil {
		return err
	}

	err = rawConn.Control(func(fd uintptr) {
		stdoutFd = int(fd)
	})

	if err != nil {
		return err
	}

	return nil
}

func Intf(seq int, ver string) {
	protocol, _ = strconv.Atoi(ver)
	var buf [64]byte
	b := strconv.AppendInt(buf[:0], int64(seq), 10)
	b = append(b, " INTF "...)
	b = append(b, strconv.FormatInt(int64(protocol), 10)...)
	b = append(b, '\n')
	syscall.Write(stdoutFd, b)
}

func Message(filename string) (qid int, h *os.File, seen bool, err error) {

	start := strings.LastIndexByte(filename, '/') + 1
	end := strings.LastIndexByte(filename, '.')
	if end <= start {
		err = fmt.Errorf("invalid filename: %s", filename)
		return
	}
	qid, err = strconv.Atoi(filename[start:end])
	if err != nil {
		return
	}

	h, err = os.Open(filename)
	if err != nil {
		return
	}

	m := bufio.NewReader(h)
	var pos int64
	pos, err = skipCGPHeaders(m)
	if err != nil {
		return
	}

	seen, err = isSeen(m)
	if err != nil || seen {
		return
	}

	_, err = h.Seek(pos, io.SeekStart)

	return
}

func MirrorTo(seq int, qid int, filename string, to []string) {
	if protocol < 4 || len(to) == 0 {
		Discard(seq)
		return
	}

	seenhdr, err := makeSeen(filename)
	if err != nil {
		Failure(seq, qid, err)
		return
	}

	buf := bufferPool.Get().(*bytes.Buffer)
	defer putBuffer(buf)

	buf.WriteString(strconv.Itoa(seq))
	buf.WriteString(" ADDHEADER \"")

	replaceSpecCharsBuf(buf, seenhdr)
	buf.WriteString("\"")

	for _, m := range to {
		buf.WriteString(" MIRRORTO \"")
		replaceSpecCharsBuf(buf, m)
		buf.WriteString("\"")
	}

	buf.WriteString(" DISCARD")

	res := buf.Bytes()
	length := len(res)

	if length > 4096 || (length == 4096 && res[length-1] != '\n') {
		Failure(seq, qid, fmt.Errorf("MirrorTo: result exceeds 4k limit"))
		return
	}

	if length > 0 && res[length-1] != '\n' {
		buf.WriteByte('\n')
		res = buf.Bytes()
	}

	syscall.Write(stdoutFd, res)
}

func Ok(seq int) {
	var buf [64]byte
	b := strconv.AppendInt(buf[:0], int64(seq), 10)
	b = append(b, " OK\n"...)
	syscall.Write(stdoutFd, b)
}

func OkSeen(seq, qid int) {
	var buf [128]byte
	b := buf[:0]
	b = append(b, "* "...)
	b = strconv.AppendInt(b, int64(seq), 10)
	b = append(b, " ["...)
	b = strconv.AppendInt(b, int64(qid), 10)
	b = append(b, "]: Already seen by DrWeb\n"...)
	syscall.Write(stdoutFd, b)
	Ok(seq)
}

func Putline(a ...any) {
	var stackBuf [128]byte
	res := stackBuf[:0]

	for _, arg := range a {
		switch v := arg.(type) {
		case string:
			res = append(res, v...)
		case int:
			res = strconv.AppendInt(res, int64(v), 10)
		case int64:
			res = strconv.AppendInt(res, v, 10)
		case error:
			if v != nil {
				res = append(res, v.Error()...)
			}
		case []byte:
			res = append(res, v...)
		default:
			res = append(res, fmt.Sprint(v)...)
		}
	}

	length := len(res)

	if length > 4096 || (length == 4096 && length > 0 && res[length-1] != '\n') {
		os.Stderr.WriteString("Putline: result exceeds 4k limit\n")
		return
	}

	if length > 0 && res[length-1] != '\n' {
		res = append(res, '\n')
	}

	syscall.Write(stdoutFd, res)
}

func Reject(seq int) {
	var buf [64]byte
	b := strconv.AppendInt(buf[:0], int64(seq), 10)
	b = append(b, " REJECT Try again later\n"...)
	syscall.Write(stdoutFd, b)
}

func SaveBadFile(seq, qid int, filename string) {
	path := badFiles + "/" + strconv.Itoa(qid) + ".msg.debug"
	err := os.Link(filename, path)
	if err != nil {
		Putline("* ", seq, " [", qid, "]: debug: ", err)
	}
}

func getHeader(m *bufio.Reader, buf *bytes.Buffer) error {
	buf.Reset()
	for {
		line, err := m.ReadSlice('\n')
		if err != nil {
			if err == bufio.ErrBufferFull {
				buf.Write(line)
				// Защита от бесконечных заголовков
				if buf.Len() > 64*1024 {
					return fmt.Errorf("header too long")
				}
				continue
			}
			if len(line) > 0 {
				buf.Write(line)
			}
			if err == io.EOF {
				break
			}
			return err
		}
		buf.Write(line)

		next, err := m.Peek(1)
		if err != nil {
			break // EOF
		}
		if next[0] != ' ' && next[0] != '\t' {
			break
		}
	}
	return nil
}

func hashReceived(b []byte) string {
	h := sha256.New224()
	for i := 0; i < len(b); {
		r, size := utf8.DecodeRune(b[i:])
		if !unicode.IsSpace(r) {
			h.Write(b[i : i+size])
		}
		i += size
	}
	return hex.EncodeToString(h.Sum(nil))
}

func isSeen(m *bufio.Reader) (seen bool, err error) {
	buf := bufferPool.Get().(*bytes.Buffer)
	defer putBuffer(buf)

	var found bool
	var seenSum []byte

	for {
		err = getHeader(m, buf)
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return
		}

		hdr := buf.Bytes()

		if len(hdr) == 0 || (len(hdr) == 1 && hdr[0] == '\n') || (len(hdr) == 2 && hdr[0] == '\r') {
			break
		}

		if !found {
			if bytes.HasPrefix(hdr, bSeenHdr) {
				val := bytes.TrimSpace(hdr[len(bSeenHdr):])
				seenSum = make([]byte, len(val))
				copy(seenSum, val)
				found = true
			}
		} else if bytes.HasPrefix(hdr, bRecvHdr) {
			if hashReceived(hdr) == string(seenSum) {
				seen = true
				break
			}
		}
	}
	return
}

func makeSeen(filename string) (string, error) {
	h, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer h.Close()

	m := bufio.NewReader(h)
	if _, err := skipCGPHeaders(m); err != nil {
		return "", err
	}

	buf := bufferPool.Get().(*bytes.Buffer)
	defer putBuffer(buf)

	for {
		if err := getHeader(m, buf); err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}

		hdr := buf.Bytes()
		if len(hdr) <= 2 && len(bytes.TrimSpace(hdr)) == 0 {
			break
		}

		if bytes.HasPrefix(hdr, bRecvHdr) {
			return string(bSeenHdr) + hashReceived(hdr), nil
		}
	}

	return "", fmt.Errorf("no Received header found")
}

func putBuffer(buf *bytes.Buffer) {
	if buf.Cap() > 4096 {
		return
	}
	buf.Reset()
	bufferPool.Put(buf)
}

func replaceSpecCharsBuf(buf *bytes.Buffer, s string) {
	for _, r := range s {
		switch r {
		case '\\':
			buf.WriteString("\\\\")
		case '\n':
			buf.WriteString("\\e")
		case '\t':
			buf.WriteString("\\t")
		case '"':
			buf.WriteString("\\\"")
		case '\r', 0x00:
			continue
		default:
			if r < utf8.RuneSelf {
				buf.WriteByte(byte(r))
			} else {
				buf.WriteRune(r)
			}
		}
	}
}

func skipCGPHeaders(m *bufio.Reader) (int64, error) {
	var pos int64
	for {
		line, err := m.ReadSlice('\n')
		pos += int64(len(line))

		if err == nil {
			if (len(line) == 1 && line[0] == '\n') ||
				(len(line) == 2 && line[0] == '\r' && line[1] == '\n') {
				return pos, nil
			}
			continue
		}

		if err == bufio.ErrBufferFull {
			continue
		}

		if err == io.EOF {
			return pos, fmt.Errorf("unexpected end of CGP headers")
		}
		return pos, err
	}
}
