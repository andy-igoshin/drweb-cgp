package cgp

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

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

func Message(filename string) (qid int, h *os.File, err error) {

	qid, err = strconv.Atoi((filename)[strings.LastIndexByte(filename, '/')+1 : strings.LastIndexByte(filename, '.')])
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
			return
		}

		pos += int64(len(line))

		if string(line) == "\n" {
			break
		}
	}

	_, err = h.Seek(pos, os.SEEK_SET)
	if err != nil {
		return
	}

	return
}

func MirrorTo(seq int, to []string, headers []string, mirrorDiscard bool) {

	hdrs := replaceSpecChars(strings.Join(headers, "\n"))

	if protocol >= 4 {

		if len(to) > 0 {

			mirrorTo := []string{}
			for _, m := range to {
				mirrorTo = append(mirrorTo, fmt.Sprintf("MIRRORTO \"%s\"", m))
			}

			if mirrorDiscard {
				Putline("%d %s DISCARD\n", seq, strings.Join(mirrorTo, " "))
			} else {
				Putline("%d %s ADDHEADER \"%s\" OK\n", seq, strings.Join(mirrorTo, " "), hdrs)
			}

		} else {
			Putline("%d ADDHEADER \"%s\" OK\n", seq, hdrs)
		}

	} else {
		Putline("%d ADDHEADER \"%s\"\n", seq, hdrs)
	}
}

func Ok(seq int) {
	Putline("%d OK\n", seq)
}

func Putline(format string, a ...interface{}) {
	s := fmt.Sprintf(format, a...)
	syscall.Write(int(os.Stdout.Fd()), []byte(s))
}

func Reject(seq int) {
	Putline("%d REJECT Try again later\n", seq)
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
