package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// Мок, который ведет себя как честный демон и закрывается только по команде из теста
func startMockDrWeb(t *testing.T, network string) (string, func()) {
	addr := "127.0.0.1:0"
	if network == "unix" {
		addr = filepath.Join(os.TempDir(), fmt.Sprintf("drweb_%d.sock", os.Getpid()))
		os.Remove(addr)
	}

	l, err := net.Listen(network, addr)
	if err != nil {
		t.Fatal(err)
	}

	// Канал для остановки демона
	stop := make(chan struct{})

	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				// Читаем команды от хелпера
				reader := bufio.NewReader(c)
				for {
					line, err := reader.ReadString('\n')
					if err != nil {
						return
					}

					// Имитируем логику DrWeb / ClamAV
					switch {
					case strings.Contains(line, "VERSION"):
						c.Write([]byte("EMULATOR: OK\n"))

					case strings.Contains(line, "FILDES"):
						// FILDES: файл передаётся через SCM_RIGHTS; для теста
						// достаточно ответить, что файл чистый.
						c.Write([]byte("fd[13]: OK\n"))
						return

					case strings.Contains(line, "INSTREAM"):
						// Честный протокол: читаем чанки (4 байта размера + данные)
						// из того же reader, чтобы не потерять уже забуференные байты.
						for {
							var size uint32
							if err := binary.Read(reader, binary.BigEndian, &size); err != nil || size == 0 {
								break
							}
							if _, err := io.CopyN(io.Discard, reader, int64(size)); err != nil {
								return
							}
						}
						c.Write([]byte("stream: OK\n"))
						return // Конец сессии после сканирования
					}
				}
			}(conn)
		}
	}()

	return l.Addr().String(), func() {
		l.Close()
		close(stop)
		if network == "unix" {
			os.Remove(addr)
		}
	}
}

func runIntegrationTest(t *testing.T, useUnix bool) {
	network := "tcp"
	if useUnix {
		network = "unix"
	}
	addr, cleanupMock := startMockDrWeb(t, network)
	defer cleanupMock()

	// Конфиг
	tmpDir := t.TempDir()
	confPath := filepath.Join(tmpDir, "drweb-cgp.yml")
	os.WriteFile(confPath, []byte(fmt.Sprintf("host: %s\ntimeout: 5s\n", addr)), 0644)

	// Пайпы
	inR, inW, _ := os.Pipe()
	outR, outW, _ := os.Pipe()

	oldStdin, oldStdout, oldArgs := os.Stdin, os.Stdout, os.Args
	defer func() {
		os.Stdin, os.Stdout, os.Args = oldStdin, oldStdout, oldArgs
	}()
	os.Stdin, os.Stdout = inR, outW
	os.Args = []string{"drweb-cgp", "-config", confPath}

	// Сброс флагов
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.PanicOnError)

	// Запуск хелпера
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer outW.Close() // Это сигнал для сканера в тесте, что хелпер закончил
		main()
	}()

	// Канал для сбора логов от хелпера
	resChan := make(chan string)
	go func() {
		scanner := bufio.NewScanner(outR)
		for scanner.Scan() {
			resChan <- scanner.Text()
		}
		close(resChan)
	}()

	// Формируем файл сообщения
	qid := 2026
	msgPath := filepath.Join(tmpDir, fmt.Sprintf("%d.msg", qid))
	os.WriteFile(msgPath, []byte("P 100 <s@t.com>\nS <r@t.com> SMTP [1.1.1.1]\n\nTest message"), 0644)

	// ВЗАИМОДЕЙСТВИЕ БЕЗ SLEEP:
	// 1. Отправляем команду
	fmt.Fprintf(inW, "%d FILE %s\n", qid, msgPath)

	// 2. Блокирующее ожидание результата в канале (синхронизация по событию)
	res, ok := <-resChan
	if !ok || !strings.Contains(res, fmt.Sprintf("%d OK", qid)) {
		t.Errorf("Unexpected result for %s: %s", network, res)
	}

	// 3. Завершение работы
	fmt.Fprintln(inW, "999 QUIT")
	inW.Close() // Посылаем EOF в stdin хелпера

	// Ждем честного завершения процесса
	wg.Wait()
}

func TestFullIntegration_Unix(t *testing.T) { runIntegrationTest(t, true) }
func TestFullIntegration_TCP(t *testing.T)  { runIntegrationTest(t, false) }

// TestScan_StreamContainsHeadersAndBody проверяет, что по INSTREAM (TCP)
// в антивирус уходит RFC822-заголовки + тело сообщения, но НЕ CGP-конверт.
func TestScan_StreamContainsHeadersAndBody(t *testing.T) {
	// Мок DrWeb, который захватывает переданные по INSTREAM байты.
	var captured bytes.Buffer

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				r := bufio.NewReader(c)
				for {
					line, err := r.ReadString('\n')
					if err != nil {
						return
					}
					if strings.Contains(line, "INSTREAM") {
						// Читаем чанки (4 байта размера + данные) до нулевого терминатора.
						for {
							var size uint32
							if err := binary.Read(r, binary.BigEndian, &size); err != nil || size == 0 {
								break
							}
							if _, err := io.CopyN(&captured, r, int64(size)); err != nil {
								return
							}
						}
						c.Write([]byte("stream: OK\n"))
						return
					}
				}
			}(conn)
		}
	}()

	// Конфиг с mock-адресом.
	tmpDir := t.TempDir()
	confPath := filepath.Join(tmpDir, "drweb-cgp.yml")
	os.WriteFile(confPath, []byte(fmt.Sprintf("host: %s\ntimeout: 5s\n", l.Addr().String())), 0644)

	// Файл сообщения: CGP-конверт + пустая строка + RFC822-заголовки + пустая строка + тело.
	qid := 2027
	msgPath := filepath.Join(tmpDir, fmt.Sprintf("%d.msg", qid))
	msgContent := "P 100 <s@t.com>\n" +
		"S <r@t.com> SMTP [1.1.1.1]\n" +
		"\n" +
		"Received: from mail.example.com ([1.2.3.4]) by mx.example.com\n" +
		"Subject: Virus scan test\n" +
		"\n" +
		"Test body content"
	os.WriteFile(msgPath, []byte(msgContent), 0644)

	// Пайпы для stdin/stdout хелпера.
	inR, inW, _ := os.Pipe()
	outR, outW, _ := os.Pipe()

	oldStdin, oldStdout, oldArgs := os.Stdin, os.Stdout, os.Args
	defer func() {
		os.Stdin, os.Stdout, os.Args = oldStdin, oldStdout, oldArgs
	}()
	os.Stdin, os.Stdout = inR, outW
	os.Args = []string{"drweb-cgp", "-config", confPath}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.PanicOnError)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer outW.Close()
		main()
	}()

	resChan := make(chan string)
	go func() {
		scanner := bufio.NewScanner(outR)
		for scanner.Scan() {
			resChan <- scanner.Text()
		}
		close(resChan)
	}()

	// Отправляем FILE и ждём подтверждения OK (значит сканирование завершено).
	fmt.Fprintf(inW, "%d FILE %s\n", qid, msgPath)
	res, ok := <-resChan
	if !ok || !strings.Contains(res, fmt.Sprintf("%d OK", qid)) {
		t.Fatalf("Unexpected result: %s", res)
	}

	fmt.Fprintln(inW, "999 QUIT")
	inW.Close()
	wg.Wait()

	got := captured.String()
	if !strings.HasPrefix(got, "Received: from mail.example.com") {
		t.Errorf("scan stream must start with RFC822 headers, got prefix: %q", got)
	}
	if !strings.Contains(got, "Subject: Virus scan test") {
		t.Errorf("scan stream must contain headers, got: %q", got)
	}
	if !strings.Contains(got, "Test body content") {
		t.Errorf("scan stream must contain body, got: %q", got)
	}
	if strings.Contains(got, "P 100 <s@t.com>") {
		t.Errorf("scan stream must NOT contain CGP envelope, got: %q", got)
	}
}
