package cgp

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"unicode/utf8"
)

func getHeader(m *bufio.Reader, buf *bytes.Buffer) error {
	for {
		line, err := m.ReadSlice('\n')
		if err != nil && err != bufio.ErrBufferFull {
			if len(line) > 0 {
				buf.Write(line)
			}
			return err
		}

		buf.Write(line)

		if err == bufio.ErrBufferFull {
			if buf.Len() > 64*1024 {
				return fmt.Errorf("header too long")
			}
			continue
		}

		if isHeaderEnd(line) {
			return nil
		}

		next, err := m.Peek(1)
		if err != nil {
			return err
		}

		if next[0] != ' ' && next[0] != '\t' {
			return nil
		}
	}
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
