package main

import (
	"bufio"
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
