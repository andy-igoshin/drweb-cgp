package cgp

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/cespare/xxhash/v2"
)

type Message struct {
	File *os.File
	QID  int

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
	bSubjHdr  = []byte("Subject: ")
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

	// bSeenHdr = []byte("X-Rspamd-Seen: ")
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

	// 4. Пропускаем Конверт (Envelope)
	pos, err := skipCGPHeaders(rd)
	if err != nil {
		return nil, err
	}

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

	_, err = msg.File.Seek(pos, io.SeekStart)

	return msg, nil
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

func putReader(rd *bufio.Reader) {
	rd.Reset(nil)
	readerPool.Put(rd)
}
