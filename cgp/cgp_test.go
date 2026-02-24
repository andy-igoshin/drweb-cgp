package cgp

import (
	"bufio"
	"strings"
	"testing"
)

func TestSkipCGPHeaders(t *testing.T) {
	// Имитируем файл CGP: служебные заголовки, пустая строка, письмо
	raw := "P 123\nS sender@test.com\n\nReturn-Path: <sender@test.com>\n"
	reader := bufio.NewReader(strings.NewReader(raw))

	pos, err := skipCGPHeaders(reader)
	if err != nil {
		t.Fatalf("skipCGPHeaders failed: %v", err)
	}

	// Ожидаем, что pos укажет на начало "Return-Path"
	// "P 123\n" (6) + "S sender@test.com\n" (18) + "\n" (1) = 25
	expectedPos := int64(strings.Index(raw, "Return-Path"))
	if pos != expectedPos {
		t.Errorf("expected pos %d, got %d", expectedPos, pos)
	}
}

func TestGetHeader_LongAndFolded(t *testing.T) {
	// Создаем заголовок длиннее 4КБ с фолдингом
	longVal := strings.Repeat("a", 5000)
	rawHeader := "X-Long-Header: " + longVal[:100] + "\n " + longVal[100:] + "\nNext-Header: val\n"
	reader := bufio.NewReader(strings.NewReader(rawHeader))

	hdr, err := getHeader(reader)
	if err != nil {
		t.Fatalf("getHeader failed on long header: %v", err)
	}

	if !strings.HasPrefix(hdr, "X-Long-Header:") {
		t.Error("Header name mismatch")
	}

	// Проверяем, что заголовок считан полностью (длина должна быть > 5000)
	if len(hdr) < 5000 {
		t.Errorf("Header too short, probably truncated: %d", len(hdr))
	}

	// Проверяем, что следующий вызов даст следующий заголовок
	next, _ := getHeader(reader)
	if !strings.HasPrefix(next, "Next-Header") {
		t.Errorf("Expected Next-Header, got: %s", next)
	}
}

func TestHashReceived(t *testing.T) {
	// Проверяем, что разные варианты фолдинга дают один хеш
	h1 := "Received: from test.com\n by mx.test.com\n with ESMTP"
	h2 := "Received:from test.com by mx.test.com with ESMTP" // без пробелов и переносов

	hash1 := hashReceived(h1)
	hash2 := hashReceived(h2)

	if hash1 != hash2 {
		t.Errorf("Hashes don't match!\n1: %s\n2: %s", hash1, hash2)
	}
}

func TestIsSeen_BoundaryConditions(t *testing.T) {
	headerToHash := "Received: from a\n"
	hash := hashReceived(headerToHash)

	// В CGP ваш заголовок будет ВЫШЕ того, который вы хешировали
	content := "X-DrWeb-Seen: " + hash + "\n" +
		headerToHash +
		"\r\n" +
		"Body starts here"

	reader := bufio.NewReader(strings.NewReader(content))
	seen, err := isSeen(reader)
	if err != nil {
		t.Fatalf("isSeen error: %v", err)
	}
	if !seen {
		t.Error("Expected seen=true, but got false. Check headers order.")
	}
}

func BenchmarkIsSeen(b *testing.B) {
	// Имитируем типичное письмо с длинным Received и нашим маркером
	longReceived := "Received: from mxfront15j.mail.yandex.net (mxfront15j.mail.yandex.net [1.2.3.4])\n" +
		" by mx.yourserver.com (CommuniGate Pro SMTP 6.3.1)\n" +
		" with ESMTPS id 1234567 for user@example.com;\n" +
		" Tue, 24 Feb 2026 10:00:00 +0300\n"

	hash := hashReceived(longReceived)
	content := "X-DrWeb-Seen: " + hash + "\n" + longReceived + "\r\nBody"

	b.ResetTimer()
	b.ReportAllocs() // Включает отчет об аллокациях

	for i := 0; i < b.N; i++ {
		reader := bufio.NewReader(strings.NewReader(content))
		_, _ = isSeen(reader)
	}
}
