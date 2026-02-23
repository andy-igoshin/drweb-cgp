package cgp

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"
	"unicode"
)

const recvHdr = "Received:"
const seenHdr = "X-DrWeb-Seen:"
const badFiles = "BadFiles"

var protocol int

func AddHeader(seq int, headers []string) {

	hdrs := replaceSpecChars(strings.Join(headers, "\n"))

	if protocol >= 4 {
		Putline("%d ADDHEADER \"%s\" OK\n", seq, hdrs)
	} else {
		Putline("%d ADDHEADER \"%s\"\n", seq, hdrs)
	}
}

func Discard(seq int) {
	Putline("%d DISCARD\n", seq)
}

func Failure(seq, qid int, err error) {
	Putline("* %d [%d]: %s\n", seq, qid, err)
	Putline("%d FAILURE\n", seq)
}

func Intf(seq int, ver string) {
	protocol, _ = strconv.Atoi(ver)
	Putline("%d INTF %d\n", seq, protocol)
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

	var line []byte
	var pos int64

	m := bufio.NewReader(h)

	for {
		line, err = m.ReadSlice('\n')
		if err != nil {
			if err == io.EOF {
				err = fmt.Errorf("unexpected end of message")
			}
			return
		}

		pos += int64(len(line))

		if string(line) == "\n" {
			break
		}
	}

	seen, err = isSeen(m)
	if err != nil {
		return
	}

	if seen {
		return
	}

	_, err = h.Seek(pos, io.SeekStart)
	if err != nil {
		return
	}

	return
}

func MirrorTo(seq int, qid int, filename string, to []string) {

	if protocol >= 4 {

		if len(to) > 0 {

			mirrorTo := []string{}
			for _, m := range to {
				mirrorTo = append(mirrorTo, fmt.Sprintf("MIRRORTO \"%s\"", m))
			}

			seenHdr, err := makeSeen(filename)
			if err != nil {
				Failure(seq, qid, err)
			} else {
				Putline("%d ADDHEADER \"%s\" %s DISCARD\n", seq, seenHdr, strings.Join(mirrorTo, " "))
			}

		} else {
			Discard(seq)
		}

	} else {
		Discard(seq)
	}
}

func Ok(seq int) {
	Putline("%d OK\n", seq)
}

func OkSeen(seq, qid int) {
	Putline("* %d [%d]: Already seen by DrWeb\n", seq, qid)
	Putline("%d OK\n", seq)
}

func Putline(format string, a ...interface{}) {
	s := fmt.Sprintf(format, a...)
	syscall.Write(int(os.Stdout.Fd()), []byte(s))
}

func Reject(seq int) {
	Putline("%d REJECT Try again later\n", seq)
}

func SaveBadFile(seq, qid int, filename string) {
	err := os.Link(filename, fmt.Sprintf("%s/%d.msg.debug", badFiles, qid))
	if err != nil {
		Putline("* %d [%d]: debug: %s\n", seq, qid, err)
	}
}

func getHeader(m *bufio.Reader) (hdr string, err error) {

	var c byte
	var line []byte

	var b strings.Builder
	b.Grow(384)

	for {
		c, err = m.ReadByte()
		if err == io.EOF {
			if c == 0 {
				break
			} else {
				err = nil
			}
		}
		if err != nil {
			return
		}

		if b.Len() == 0 {

			if c == ' ' || c == '\t' {
				err = m.UnreadByte()
				if err != nil {
					return
				}
				err = fmt.Errorf("bad header")
				return

			} else if c == '\n' {

				b.WriteByte(c)
				break

			} else {

				b.WriteByte(c)

				line, err = m.ReadSlice('\n')
				if err == io.EOF {
					err = nil
				}
				if err != nil {
					return
				}

				b.Write(line)
			}

		} else {

			if c == ' ' || c == '\t' {

				b.WriteByte(c)

				line, err = m.ReadSlice('\n')
				if err == io.EOF {
					err = nil
				}
				if err != nil {
					return
				}

				b.Write(line)

			} else {
				err = m.UnreadByte()
				if err != nil {
					return
				}
				break
			}
		}
	}

	hdr = b.String()

	return
}

func isSeen(m *bufio.Reader) (seen bool, err error) {

	var found bool
	var hdr string
	var seenSum string

	for {

		hdr, err = getHeader(m)
		if err == io.EOF {
			err = nil
			if len(hdr) == 0 {
				break
			}
		}
		if err != nil {
			return
		}

		if hdr == "\n" {
			// конец заголовка
			break
		}

		if !found {

			if seenSum, found = strings.CutPrefix(hdr, seenHdr); found {
				seenSum = strings.TrimSpace(seenSum)
			}

		} else if strings.HasPrefix(hdr, recvHdr) {

			sum := fmt.Sprintf("%x", sha256.Sum224([]byte(nospace(hdr))))
			if seenSum == sum {
				seen = true
			}

			break
		}
	}

	return
}

func makeSeen(filename string) (seenhdr string, err error) {

	var h *os.File
	var hdr string
	var line []byte

	h, err = os.Open(filename)
	if err != nil {
		return
	}
	defer h.Close()

	m := bufio.NewReader(h)

	for {
		line, err = m.ReadSlice('\n')
		if err != nil {
			if err == io.EOF {
				err = fmt.Errorf("unexpected end of message")
			}
			return
		}

		if string(line) == "\n" {
			break
		}
	}

	for {

		hdr, err = getHeader(m)
		if err == io.EOF {
			err = nil
			if len(hdr) == 0 {
				break
			}
		}
		if err != nil {
			return
		}

		if hdr == "\n" {
			break
		}

		if strings.HasPrefix(hdr, recvHdr) {
			seenhdr = fmt.Sprintf("%s %x", seenHdr, sha256.Sum224([]byte(nospace(hdr))))
			break
		}
	}

	return
}

func nospace(s string) string {

	var b strings.Builder
	b.Grow(len(s))

	for _, c := range s {
		if !unicode.IsSpace(c) {
			b.WriteRune(c)
		}
	}

	return b.String()
}

func replaceSpecChars(msg string) string {

	var sb strings.Builder
	sb.Grow(len(msg) + 128)

	for _, symbol := range msg {

		switch symbol {
		case rune('\\'):
			// replace \ -> \\   (CGP backslash)
			sb.WriteString("\\\\")

		case 0x0000:
			fallthrough
		case rune('\r'):
			continue

		case rune('\n'):
			// replace \n -> \\e (CGP End-of-Line)
			sb.WriteString("\\e")

		case rune('\t'):
			// replace \t -> \\t (CGP Tab)
			sb.WriteString("\\t")

		case rune('"'):
			// replace \" -> \\" (CGP quote)
			sb.WriteString("\\\"")

		default:
			sb.WriteRune(symbol)
		}
	}

	return sb.String()
}
