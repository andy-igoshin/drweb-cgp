package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

//	"git.vsu.ru/ai/drweb-cgp/config"
)

// startMockDrWeb имитирует сервер DrWeb/ClamAV
func startMockDrWeb(t *testing.T, network string) (string, func()) {
	addr := ""
	if network == "unix" {
		addr = filepath.Join(os.TempDir(), "drweb_test.sock")
		os.Remove(addr)
	} else {
		addr = "127.0.0.1:0"
	}

	l, err := net.Listen(network, addr)
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				for {
					cmdBuf := make([]byte, 0, 128)
					b := make([]byte, 1)
					for {
						n, err := conn.Read(b)
						if err != nil || n == 0 {
							return
						}
						if b[0] == '\n' {
							break
						}
						cmdBuf = append(cmdBuf, b[0])
					}
					line := string(cmdBuf)

					if strings.Contains(line, "VERSION") {
						conn.Write([]byte("EMULATOR: OK\n"))
					} else if strings.Contains(line, "FILDES") {
						conn.Write([]byte("fildes: OK\n"))
						return
					} else if strings.Contains(line, "INSTREAM") {
						for {
							var size uint32
							if err := binary.Read(conn, binary.BigEndian, &size); err != nil {
								break
							}
							if size == 0 {
								break
							}
							io.CopyN(io.Discard, conn, int64(size))
						}
						conn.Write([]byte("stream: OK\n"))
						return
					}
				}
			}(c)
		}
	}()

	actualAddr := l.Addr().String()
	if network == "unix" {
		actualAddr = addr
	}
	return actualAddr, func() {
		l.Close()
		if network == "unix" {
			os.Remove(addr)
		}
	}
}

func createCgpQueueFile(t *testing.T, qid int, bodySize int) string {
	path := filepath.Join(os.TempDir(), fmt.Sprintf("%d.msg", qid))
	f, err := os.Create(path)
	if err != nil {
		if t != nil {
			t.Fatal(err)
		}
		return ""
	}
	defer f.Close()

	f.WriteString("P 100 <sender@test.com>\nS <rcpt@test.com>\n\n")
	f.WriteString("Subject: Integration Test\n\n")
	if bodySize > 0 {
		body := make([]byte, bodySize)
		for i := range body {
			body[i] = 'A' + byte(i%26)
		}
		f.Write(body)
	}
	return path
}

func runIntegrationTest(t *testing.T, useUnix bool) {
	network := "tcp"
	if useUnix {
		network = "unix"
	}

	addr, cleanup := startMockDrWeb(t, network)
	defer cleanup()

	oldArgs := os.Args
	os.Args = []string{"drweb-cgp", "-host", addr}
	defer func() { os.Args = oldArgs }()

	inR, inW, _ := os.Pipe()
	outR, outW, _ := os.Pipe()
	oldStdin, _ := syscall.Dup(0)
	oldStdout, _ := syscall.Dup(1)

	syscall.Dup2(int(inR.Fd()), 0)
	syscall.Dup2(int(outW.Fd()), 1)
	inR.Close()
	outW.Close()

	defer func() {
		syscall.Dup2(oldStdin, 0)
		syscall.Dup2(oldStdout, 1)
	}()

	go main()
	time.Sleep(100 * time.Millisecond)

	responses := make(chan string, 100)
	go func() {
		scanner := bufio.NewScanner(outR)
		for scanner.Scan() {
			responses <- scanner.Text()
		}
	}()

	testSizes := []int{512, 1024, 20480}
	for i, size := range testSizes {
		qid := 2000 + i
		path := createCgpQueueFile(t, qid, size)

		fmt.Fprintf(inW, "%d FILE %s\n", qid, path)

		success := false
		timeout := time.After(2 * time.Second)
	waitLoop:
		for {
			select {
			case res := <-responses:
				if strings.Contains(res, fmt.Sprintf("%d OK", qid)) {
					success = true
					break waitLoop
				}
			case <-timeout:
				t.Errorf("Timeout waiting for OK on size %d", size)
				break waitLoop
			}
		}
		if success {
			os.Remove(path)
		}
	}

	fmt.Fprint(inW, "999 QUIT\n")
	inW.Close()
	time.Sleep(100 * time.Millisecond)
}

func TestFullIntegration_Unix(t *testing.T) { runIntegrationTest(t, true) }
func TestFullIntegration_TCP(t *testing.T)  { runIntegrationTest(t, false) }

func BenchmarkHelper_20KB_Unix(b *testing.B) {
	addr, cleanup := startMockDrWeb(nil, "unix")
	defer cleanup()

	oldArgs := os.Args
	os.Args = []string{"drweb-cgp", "-host", addr}
	defer func() { os.Args = oldArgs }()

	inR, inW, _ := os.Pipe()
	outR, outW, _ := os.Pipe()
	oldStdin, _ := syscall.Dup(0)
	oldStdout, _ := syscall.Dup(1)

	syscall.Dup2(int(inR.Fd()), 0)
	syscall.Dup2(int(outW.Fd()), 1)
	inR.Close()
	outW.Close()

	defer func() {
		syscall.Dup2(oldStdin, 0)
		syscall.Dup2(oldStdout, 1)
	}()

	// 4. Запускаем хелпер
	go main()
	time.Sleep(100 * time.Millisecond)

	respChan := make(chan struct{}, 1)
	go func() {
		scanner := bufio.NewScanner(outR)
		for scanner.Scan() {
			if strings.Contains(scanner.Text(), "OK") {
				select { case respChan <- struct{}{}: default: }
			}
		}
	}()

	path := createCgpQueueFile(nil, 888, 20480)
	defer os.Remove(path)
	cmd := []byte(fmt.Sprintf("1 FILE %s\n", path))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		inW.Write(cmd)
		<-respChan
	}
}

func BenchmarkHelper_20KB_TCP(b *testing.B) {
	addr, cleanup := startMockDrWeb(nil, "tcp")
	defer cleanup()

	oldArgs := os.Args
	os.Args = []string{"drweb-cgp", "-host", addr}
	defer func() { os.Args = oldArgs }()

	inR, inW, _ := os.Pipe()
	outR, outW, _ := os.Pipe()
	oldStdin, _ := syscall.Dup(0)
	oldStdout, _ := syscall.Dup(1)
	syscall.Dup2(int(inR.Fd()), 0)
	syscall.Dup2(int(outW.Fd()), 1)
	inR.Close()
	outW.Close()
	defer func() {
		syscall.Dup2(oldStdin, 0)
		syscall.Dup2(oldStdout, 1)
	}()

	go main()
	time.Sleep(100 * time.Millisecond)

	respChan := make(chan struct{}, 1)
	go func() {
		scanner := bufio.NewScanner(outR)
		for scanner.Scan() {
			if strings.Contains(scanner.Text(), "OK") {
				select {
				case respChan <- struct{}{}:
				default:
				}
			}
		}
	}()

	path := createCgpQueueFile(nil, 777, 20480)
	defer os.Remove(path)
	cmd := []byte(fmt.Sprintf("1 FILE %s\n", path))

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		inW.Write(cmd)
		<-respChan
	}
}
