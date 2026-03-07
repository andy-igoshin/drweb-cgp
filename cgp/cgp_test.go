package cgp

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func readOutput(r *os.File) string {
	var buf [8192]byte
	r.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	n, _ := r.Read(buf[:])
	return string(buf[:n])
}

func setupTestFd(t *testing.T) (*os.File, *os.File) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdoutFd = int(w.Fd())
	return r, w
}

func TestAddHeader(t *testing.T) {
	r, w := setupTestFd(t)
	defer r.Close()
	defer w.Close()

	protocol = 4
	AddHeader(101, []string{"X-Test: True", "X-Admin: Yes"})

	got := readOutput(r)
	want := "101 ADDHEADER \"X-Test: True\\eX-Admin: Yes\" OK\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestAddHeaderLimit(t *testing.T) {
	r, w := setupTestFd(t)
	defer r.Close()
	defer w.Close()

	// Создаем заголовок, который гарантированно выйдет за 4096 байт
	bigHeader := strings.Repeat("A", 4100)
	AddHeader(102, []string{bigHeader})

	got := readOutput(r)
	if !strings.Contains(got, "FAILURE") {
		t.Errorf("Expected FAILURE on 4k limit, got %q", got)
	}
}

func TestDiscard(t *testing.T) {
	r, w := setupTestFd(t)
	defer r.Close()
	defer w.Close()

	Discard(202)
	got := readOutput(r)
	if got != "202 DISCARD\n" {
		t.Errorf("got %q", got)
	}
}

func TestFailure(t *testing.T) {
	r, w := setupTestFd(t)
	defer r.Close()
	defer w.Close()

	// Тест с ошибкой, содержащей \n (проверка логики подчистки)
	Failure(303, 999, fmt.Errorf("Critical error\n"))
	got := readOutput(r)

	if strings.Count(got, "\n") != 2 {
		t.Errorf("Expected exactly 2 lines (log + failure), got %d", strings.Count(got, "\n"))
	}
	if !strings.Contains(got, "* 303 [999]: Critical error\n") {
		t.Errorf("Log line format mismatch: %q", got)
	}
}

func TestMessageAndIsSeenIntegration(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "cgp_msg")
	defer os.RemoveAll(tmpDir)

	qid := 54321
	fname := filepath.Join(tmpDir, strconv.Itoa(qid)+".msg")

	recvLine := "Received: from remote.host by local.host\n"
	hash := hashReceived([]byte(recvLine))

	// Контент: CGP заголовки + Сообщение
	content := "P 100 <test@test.com>\n\n" +
		"X-DrWeb-Seen: " + hash + "\n" +
		recvLine + "\n" +
		"Body of message"

	os.WriteFile(fname, []byte(content), 0644)

	msg, err := NewMessage(2, fname)
	if err != nil {
		t.Fatal(err)
	}
	defer msg.Close()

	if msg.QID != qid || !msg.IsSeen() {
		t.Errorf("Message parse failed: qid=%d, seen=%v", msg.QID, msg.IsSeen())
	}
}

func TestMirrorTo_Integrated(t *testing.T) {
	// Подготовка Pipe для захвата syscall.Write
	r, w, _ := os.Pipe()
	oldStdout := stdoutFd
	stdoutFd = int(w.Fd())
	defer func() { stdoutFd = oldStdout }()

	tmpDir, _ := os.MkdirTemp("", "cgp_mirror_integrated")
	defer os.RemoveAll(tmpDir)

	t.Run("Success_Mirroring", func(t *testing.T) {
		fname := filepath.Join(tmpDir, "505.msg")
		content := "P 123 <addr>\n\nReceived: from relay1\n\nBody"
		os.WriteFile(fname, []byte(content), 0644)

		msg, err := NewMessage(505, fname)
		if err != nil {
			t.Errorf("NewMessage error: %v", err)
		}

		var headers []string
		to := []string{"user@remote.com"}
		if seenHdr := msg.MakeSeen(); seenHdr != "" {
			headers = append(headers, seenHdr)
		} else {
			t.Errorf("warning: MirrorTo skipped (no Seen tag)")
			to = nil
		}

		MirrorTo(505, msg, to, headers)

		got := readOutput(r)
		if strings.Count(got, "\n") != 1 {
			t.Errorf("Expected exactly one newline for atomic response, got %d", strings.Count(got, "\n"))
		}
		if !strings.Contains(got, "MIRRORTO") || !strings.Contains(got, "DISCARD") {
			t.Errorf("Chain missing components: %q", got)
		}
		if !strings.Contains(got, "X-DrWeb-Seen:") {
			t.Error("X-DrWeb-Seen header was not generated")
		}
	})

	t.Run("No_Received_Header", func(t *testing.T) {
		fname := filepath.Join(tmpDir, "506.msg")
		// Файл без Received — makeSeen должен споткнуться
		content := "P 124 <addr>\n\nSubject: No Received\n\nBody"
		os.WriteFile(fname, []byte(content), 0644)

		msg, err := NewMessage(506, fname)
		if err != nil {
			t.Errorf("NewMessage error: %v", err)
		}

		var headers []string
		to := []string{"user@remote.com"}
		if seenHdr := msg.MakeSeen(); seenHdr != "" {
			headers = append(headers, seenHdr)
		} else {
			t.Log("warning: MirrorTo skipped (no Seen tag)")
			to = nil
		}

		MirrorTo(506, msg, to, headers)

		got := readOutput(r)
		// Ожидаем вызов Failure (две строки: * seq [qid]... и seq FAILURE)
		if !strings.Contains(got, "DISCARD") {
			t.Errorf("Expected DISCARD due to missing Received header, got: %q", got)
		}
	})
}

func TestOk(t *testing.T) {
	r, w := setupTestFd(t)
	defer r.Close()
	defer w.Close()

	Ok(606)
	if readOutput(r) != "606 OK\n" {
		t.Error("Ok output mismatch")
	}
}

func TestPutline(t *testing.T) {
	r, w := setupTestFd(t)
	defer r.Close()
	defer w.Close()

	Putline("Tag: ", "Info", " Code: ", 200)
	got := readOutput(r)
	if got != "Tag: Info Code: 200\n" {
		t.Errorf("got %q", got)
	}
}

func TestSaveBadFile(t *testing.T) {
	os.Mkdir(badFiles, 0755)
	defer os.RemoveAll(badFiles)

	src := "source.tmp"
	os.WriteFile(src, []byte("data"), 0644)
	defer os.Remove(src)

	SaveBadFile(1, 777, src)
	dst := filepath.Join(badFiles, "777.msg.debug")
	if _, err := os.Stat(dst); os.IsNotExist(err) {
		t.Error("Debug file not linked")
	}
}

// --- Тесты приватных функций и механизмов ---

func TestGetHeaderFolding(t *testing.T) {
	data := "Subject: Long\n  folded subject\nNext: Header\n"
	r := bufio.NewReader(strings.NewReader(data))
	buf := new(bytes.Buffer)

	err := getHeader(r, buf)
	if err != nil {
		t.Fatal(err)
	}

	res := buf.String()
	if !strings.Contains(res, "folded subject") {
		t.Errorf("Folding not handled: %q", res)
	}
}

func TestHashReceived(t *testing.T) {
	h1 := hashReceived([]byte("Received: from a by b\n"))
	h2 := hashReceived([]byte("Received: from  a  by  b\r\n"))
	if h1 != h2 {
		t.Error("Hash should ignore whitespace differences")
	}
}

func TestPutBufferHygiene(t *testing.T) {
	buf := bufferPool.Get().(*bytes.Buffer)
	// Искусственно раздуваем буфер
	buf.Write(make([]byte, 5000))
	putBuffer(buf)

	// Берем новый
	newBuf := bufferPool.Get().(*bytes.Buffer)
	if newBuf.Cap() > 4096 {
		t.Errorf("Pool hygiene failed: buffer with cap %d returned", newBuf.Cap())
	}
}

func TestReplaceSpecCharsBuf(t *testing.T) {
	buf := new(bytes.Buffer)
	replaceSpecCharsBuf(buf, "a\\b\"c\n\t")
	want := "a\\\\b\\\"c\\e\\t"
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
}

func TestSkipCGPHeadersEdge(t *testing.T) {
	// Кейс: файл обрывается до пустой строки
	r := bufio.NewReader(strings.NewReader("P 1\nS 2"))
	_, err := skipCGPHeaders(r)
	if err == nil {
		t.Error("Expected error on unexpected EOF")
	}
}
