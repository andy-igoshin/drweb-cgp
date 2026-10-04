package cgp

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// MessageMock помогает собирать тестовые файлы сообщений CGP в байтовый массив
type MessageMock struct {
	Envelope []string
	Headers  []string
	Body     string
}

func (m *MessageMock) Render() []byte {
	var sb strings.Builder
	// 1. Конверт
	for _, line := range m.Envelope {
		sb.WriteString(line)
		sb.WriteByte('\n')
	}
	sb.WriteByte('\n') // Пустая строка - признак конца конверта

	// 2. RFC Заголовки
	for _, line := range m.Headers {
		sb.WriteString(line)
		sb.WriteByte('\n')
	}
	sb.WriteByte('\n') // Пустая строка - признак конца заголовков

	// 3. Тело
	sb.WriteString(m.Body)

	return []byte(sb.String())
}

// createTestFile создает структуру папок Submitted и пишет туда файл сообщения
func createTestFile(t *testing.T, qid int, content []byte) string {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, submitDir)
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	filename := filepath.Join(subDir, strconv.Itoa(qid)+".msg")
	if err := os.WriteFile(filename, content, 0644); err != nil {
		t.Fatal(err)
	}
	return filename
}

func TestNewMessage_Parsing(t *testing.T) {
	rawRecv := "Received: from mail.domain.name ([1.2.3.4] verified)"
	mock := &MessageMock{
		Envelope: []string{
			"P <sender@domain.name>",
			"R <rcpt1@domain.name>",
			"R <rcpt2@domain.name>",
			"S SMTP [1.2.3.4]",
		},
		Headers: []string{
			rawRecv,
			"Subject: Test",
			"From: sender@domain.name",
		},
		Body: "Hello world!",
	}

	content := mock.Render()
	qid := 10001
	fname := createTestFile(t, qid, content)

	msg, err := NewMessage(1, fname)
	if err != nil {
		t.Fatalf("NewMessage failed: %v", err)
	}
	defer msg.Close()

	// Проверка извлечения данных
	if msg.QID != qid {
		t.Errorf("QID mismatch: got %d, want %d", msg.QID, qid)
	}
	if msg.From != "sender@domain.name" {
		t.Errorf("From mismatch: got %s", msg.From)
	}
	if len(msg.Rcpts) != 2 {
		t.Errorf("Rcpts count mismatch: got %d", len(msg.Rcpts))
	}

	// Проверка смещений (HdrPos должен указывать на 'R' в 'Received')
	expectedHdrPos := int64(strings.Index(string(content), "Received:"))
	if msg.HdrPos != expectedHdrPos {
		t.Errorf("HdrPos: got %d, want %d", msg.HdrPos, expectedHdrPos)
	}

	// Проверка смещения тела (BodyPos после пустой строки после заголовков)
	expectedBodyPos := int64(strings.Index(string(content), "Hello world!"))
	if msg.BodyPos != expectedBodyPos {
		t.Errorf("BodyPos: got %d, want %d", msg.BodyPos, expectedBodyPos)
	}
}

func TestNewMessage_Malformed(t *testing.T) {
	t.Run("Unexpected EOF", func(t *testing.T) {
		// Обрываем файл прямо в середине конверта
		content := []byte("P <sender@domain.name>\nS SMTP [1.2.3.4]")
		fname := createTestFile(t, 30003, content)

		_, err := NewMessage(1, fname)
		if err == nil || !strings.Contains(err.Error(), "unexpected end of envelope") {
			t.Errorf("Expected EOF error, got: %v", err)
		}
	})
}
