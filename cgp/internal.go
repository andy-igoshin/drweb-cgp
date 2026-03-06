package cgp

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"unicode/utf8"

	"github.com/cespare/xxhash/v2"
)

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
	h := xxhash.New()

	start := 0
	for i := 0; i < len(b); i++ {
		// Нормализация: игнорируем пробелы, табы и переносы строк (folding)
		if b[i] <= 32 {
			if i > start {
				h.Write(b[start:i])
			}
			start = i + 1
		}
	}

	// Дописываем остаток, если он есть
	if start < len(b) {
		h.Write(b[start:])
	}

	// Результат — 64-битное число, приводим к hex (16 символов)
	return strconv.FormatUint(h.Sum64(), 16)
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

		if len(hdr) == 0 || (len(hdr) == 1 && hdr[0] == '\n') ||
			(len(hdr) == 2 && hdr[0] == '\r' && hdr[1] == '\n') {
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
