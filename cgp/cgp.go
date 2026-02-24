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
	"unicode/utf8"
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

func getHeader(m *bufio.Reader) (string, error) {
	var b strings.Builder
	b.Grow(1024)

	for {
		line, err := m.ReadSlice('\n')
		if err == nil {
			// Успешно прочитали порцию до \n
			b.Write(line)
		} else if err == bufio.ErrBufferFull {
			// Строка больше 4КБ. Пишем то, что влезло в буфер, и продолжаем
			b.Write(line)
			continue
		} else {
			// EOF или критическая ошибка
			if len(line) > 0 {
				b.Write(line)
			}
			if err == io.EOF {
				break
			}
			return "", err
		}

		// Заглядываем вперед на предмет folding (пробел или таб)
		next, err := m.Peek(1)
		if err != nil {
			break // EOF
		}

		if next[0] != ' ' && next[0] != '\t' {
			// Следующая строка — новый заголовок
			break
		}
	}

	return b.String(), nil
}

func hashReceived(s string) string {
	h := sha256.New224()
	for _, r := range s {
		if !unicode.IsSpace(r) {
			if r < utf8.RuneSelf {
				h.Write([]byte{byte(r)})
			} else {
				var buf [utf8.UTFMax]byte
				n := utf8.EncodeRune(buf[:], r)
				h.Write(buf[:n])
			}
		}
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func isSeen(m *bufio.Reader) (seen bool, err error) {
    var found bool
    var seenSum string

    for {
        hdr, err := getHeader(m)
        if err != nil {
            if err == io.EOF {
                break
            }
            return false, fmt.Errorf("1: %v", err)
        }

        if hdr == "\n" || hdr == "\r\n" {
			// конец заголовка
            break
        }

        if !found {
            if val, ok := strings.CutPrefix(hdr, seenHdr); ok {
                seenSum = strings.TrimSpace(val)
                found = true
            }
        } else if strings.HasPrefix(hdr, recvHdr) {
            if hashReceived(hdr) == seenSum {
                seen = true
            }
            break
        }
    }
    return seen, nil
}

func makeSeen(filename string) (seenhdr string, err error) {
	h, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer h.Close()

	m := bufio.NewReader(h)

	if _, err := skipCGPHeaders(m); err != nil {
		return "", err
	}

	for {
		hdr, err := getHeader(m)
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", fmt.Errorf("read message header error: %v", err)
		}

		if hdr == "\n" || hdr == "\r\n" {
			// конец заголовка
			break
		}

		if strings.HasPrefix(hdr, recvHdr) {
			seenhdr = fmt.Sprintf("%s %s", seenHdr, hashReceived(hdr))
			return seenhdr, nil
		}
	}

	return "", fmt.Errorf("no Received header found in message")
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
