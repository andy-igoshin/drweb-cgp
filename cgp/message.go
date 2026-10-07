package cgp

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/cespare/xxhash/v2"
	"github.com/emersion/go-msgauth/dkim"

	"git.vsu.ru/ai/drweb-cgp/utils"
)

// Sentinel-ошибки для различимых исходов проверки DKIM.
var (
	ErrDKIMNoSignature = errors.New("dkim: no signature found")
	ErrDKIMInvalid     = errors.New("dkim: invalid signature")
)

type Message struct {
	File  *os.File
	QID   int
	From  string
	Rcpts []string
	Auth  string

	HdrPos  int64 // Смещение до RFC заголовков (после конверта)
	BodyPos int64 // Смещение до RFC тела (после заголовков)

	seen        bool
	extReceived []byte
}

var (
	readerPool = sync.Pool{
		New: func() any { return bufio.NewReaderSize(nil, 4096) },
	}
	writerPool = sync.Pool{
		New: func() any { return bufio.NewWriterSize(nil, 4096) },
	}
)

var (
	bRecvHdr  = []byte("Received: ")
	bRecvFrom = []byte("Received: from ")
	bSeenHdr  = []byte("X-DrWeb-Seen: ")
)

var (
	// Тип источника: 1 - Внешний (SMTP/HTTP...), 2 - Внутренний (RULE/LIST...)
	sourceModules = map[string]int{
		"SMTP": 1, "HTTP": 1, "HTTPU": 1, "AIRSYNC": 1, "IMAP": 1, "RPOP": 1, "XIMSS": 1,
		"ALARM": 2, "DSN": 2, "GROUP": 2, "ICAL": 2, "LIST": 2, "LSTM": 2, "LSTO": 2,
		"MDN": 2, "PBX": 2, "PIPE": 2, "RULE": 2, "WEBUSER": 2,
	}
)

func (m *Message) Close() error {
	return m.File.Close()
}

func (m *Message) IsSeen() bool {
	return m.seen
}

func (m *Message) MakeSeen() string {
	// Если внешних Received не найдено (внутреннее письмо),
	// возвращаем пустую строку, чтобы заголовок не добавлялся.
	if len(m.extReceived) == 0 {
		return ""
	}

	// bSeenHdr = []byte("X-DrWeb-Seen: ")
	// Собираем строку: имя заголовка + хеш от сохраненного оригинала
	return string(bSeenHdr) + hashReceived(m.extReceived)
}

func NewMessage(seq int, filename string) (*Message, error) {
	// 1. Извлечение QID из имени файла (например, 12345.msg -> 12345)
	start := strings.LastIndexByte(filename, '/') + 1
	end := strings.LastIndexByte(filename, '.')
	if end <= start {
		return nil, fmt.Errorf("invalid filename: %s", filename)
	}

	qid, err := strconv.Atoi(filename[start:end])
	if err != nil {
		return nil, fmt.Errorf("parse QID failed: %w", err)
	}

	// 2. Открытие файла оригинала
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	msg := &Message{
		File: f,
		QID:  qid,
	}

	// 3. Подготовка ридера из пула
	rd := getReader(f)
	defer putReader(rd)

	var pos int64

	// 4. Читаем Конверт (Envelope)
	for {
		line, err := rd.ReadSlice('\n')
		if err != nil {
			if err == io.EOF {
				err = fmt.Errorf("unexpected end of envelope")
			}
			f.Close()
			return nil, err
		}
		pos += int64(len(line))

		if isHeaderEnd(line) {
			break
		}

		// Парсинг параметров конверта
		switch line[0] {
		case 'P': // Return-Path
			if v, ok := extractAngle(line); ok {
				msg.From = v
			}
		case 'R': // Envelope-To
			if v, ok := extractAngle(line); ok {
				msg.Rcpts = append(msg.Rcpts, v)
			}
		case 'S': // Source/Interface info
			parseSource(seq, line, msg)
		}
	}
	msg.HdrPos = pos

	// 5. Разбор Заголовков (Headers)
	buf := bufferPool.Get().(*bytes.Buffer)
	defer putBuffer(buf)

	var (
		seenSum          string
		seenFound        bool
		seenProcessed    bool
		extReceivedSaved bool
	)

	for {
		buf.Reset()
		err := getHeader(rd, buf)
		hdr := buf.Bytes()

		if len(hdr) > 0 {
			pos += int64(len(hdr))

			if isHeaderEnd(hdr) {
				break
			}

			// А. Проверка существующей метки Seen (Прошлое)
			// Доверяем порядку: Метка -> (опционально не-Received) -> Первый Received
			if !seenProcessed {
				if !seenFound {
					if bytes.HasPrefix(hdr, bSeenHdr) {
						seenSum = string(bytes.TrimSpace(hdr[len(bSeenHdr):]))
						seenFound = true
					}
				} else if bytes.HasPrefix(hdr, bRecvHdr) {
					// Валидируем по первому же встречному Received
					if hashReceived(hdr) == seenSum {
						msg.seen = true
					}
					seenProcessed = true
				}
			}

			// Б. Идентификация точки входа (Будущее)
			// Нас интересует только первый внешний Received.
			if !extReceivedSaved && isExternalReceived(hdr) {
				msg.extReceived = bytes.Clone(hdr)
				extReceivedSaved = true
			}
		}

		if err != nil {
			if err == io.EOF {
				break
			}
			f.Close()
			return nil, err
		}
	}

	msg.BodyPos = pos

	// Позиционируемся на начало RFC822-заголовков (после CGP-конверта),
	// чтобы антивирус проверял всё сообщение целиком (заголовки + тело).
	_, err = msg.File.Seek(msg.HdrPos, io.SeekStart)

	return msg, nil
}

func (m *Message) VerifyDKIM() (string, error) {
	// 1. Возвращаемся к началу RFC-заголовков
	if _, err := m.File.Seek(m.HdrPos, 0); err != nil {
		return "", fmt.Errorf("seek failed: %w", err)
	}

	// 2. Настраиваем опции с кэширующим резолвером
	options := &dkim.VerifyOptions{
		LookupTXT: func(domain string) ([]string, error) {
			key := getDKIMKey(domain)
			if key == "" {
				return nil, fmt.Errorf("dkim key not found: %s", domain)
			}
			return []string{key}, nil
		},
	}

	// 3. Вызываем проверку
	verifications, err := dkim.VerifyWithOptions(m.File, options)
	if err != nil {
		return "", fmt.Errorf("dkim verify: %w", err)
	}

	// 4. Ищем успешный результат
	found := false
	for _, v := range verifications {
		if v.Err == nil {
			return strings.ToLower(v.Domain), nil
		}
		found = true
	}

	if found {
		return "", ErrDKIMInvalid
	}
	return "", ErrDKIMNoSignature
}

func getReader(r io.Reader) *bufio.Reader {
	rd := readerPool.Get().(*bufio.Reader)
	rd.Reset(r)
	return rd
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

func isExternalReceived(hdr []byte) bool {
	// 1. Проверяем полный префикс.
	if !bytes.HasPrefix(hdr, bRecvFrom) {
		return false
	}

	// 2. Смещаемся за "Received: from "
	fromVal := hdr[len(bRecvFrom):]
	if len(fromVal) == 0 {
		return false
	}

	// 3. Быстрый чек на IP в скобках [IP] (SMTP/Web)
	if fromVal[0] == '[' {
		return true
	}

	// 4. Ищем границу идентификатора (до первого пробела)
	firstSpace := bytes.IndexByte(fromVal, ' ')
	if firstSpace == -1 {
		firstSpace = len(fromVal)
	}
	firstWord := fromVal[:firstSpace]

	// 5. Проверка на Email (GROUP/RULE).
	// Если во входном идентификаторе нет '@', это сетевой хост.
	return len(firstWord) > 0 && !bytes.Contains(firstWord, []byte("@"))
}

func isHeaderEnd(hdr []byte) bool {
	return len(hdr) == 1 && hdr[0] == '\n' ||
		(len(hdr) == 2 && hdr[0] == '\r' && hdr[1] == '\n')
}

/*
		внешние источники: SMTP|HTTPU?|AIRSYNC|IMAP|RPOP|XIMSS
		S <test@domain.name> AIRSYNC [83.139.170.75]
		S <test@domain.name> HTTP [46.72.226.199]
		S SMTP [130.193.65.125]
		S <test@domain.name> SMTP [2001:67c:418:2020::21]
		S RPOP [81.19.77.161]

		внутренние источники: ALARM|DSN|GROUP|ICAL|LIST|LSTM|LSTO|MDN|PBX|PIPE|RULE|WEBUSER
		S DSN [0.0.0.0]
	 	S GROUP [0.0.0.0]
	 	S LIST [0.0.0.0]
		S LSTM [0.0.0.0]
		S <test@domain.name> MDN [0.0.0.0]
		S PIPE [0.0.0.0]
		S <postmaster@domain.name> RULE [0.0.0.0]
*/
func parseSource(seq int, line []byte, msg *Message) {
	// Строка вида: S [<auth> ]MODULE [IP]
	data := bytes.TrimSpace(line[1:])
	if len(data) == 0 {
		return
	}

	var auth string
	// 1. Извлекаем <auth>, если он присутствует
	if data[0] == '<' {
		endAuth := bytes.IndexByte(data, '>')
		if endAuth != -1 {
			auth = string(data[1:endAuth])
			data = bytes.TrimSpace(data[endAuth+1:])
		}
	}

	// 2. Ищем IP в квадратных скобках в конце строки
	openBracket := bytes.LastIndexByte(data, '[')
	closeBracket := bytes.LastIndexByte(data, ']')

	if openBracket == -1 || closeBracket == -1 || closeBracket <= openBracket {
		Putline("* ", seq, " [", msg.QID, "]: Unknown S-line format: ", utils.Bytes2string(line))
		return
	}

	moduleBytes := bytes.TrimSpace(data[:openBracket])
	ipBytes := data[openBracket+1 : closeBracket]

	mType, known := sourceModules[utils.Bytes2string(moduleBytes)]

	// 3. Заполнение структуры
	if known && mType == 2 && bytes.Equal(ipBytes, []byte("0.0.0.0")) {
		// Внутренний источник (trusted)
		if auth != "" {
			msg.Auth = auth
		}
	} else if known && mType == 1 {
		// Внешний источник (SMTP, и т.д.)
		msg.Auth = auth
	} else {
		Putline("* ", seq, " [", msg.QID, "]: Unknown or mismatched module in S-line: ", string(line))
	}
}

func putReader(rd *bufio.Reader) {
	rd.Reset(nil)
	readerPool.Put(rd)
}
