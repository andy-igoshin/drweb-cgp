package clamd

import (
	"bytes"
	"io"
	"net"
	"os"
	"testing"
)

func TestProcessResponse_OriginalLogic(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  Status
		filename  string
		signature string
	}{
		{
			name:      "DrWeb Fildes OK",
			input:     "fd[13]: OK\n",
			expected:  Ok,
			signature: "",
		},
		{
			name:      "DrWeb Fildes Virus",
			input:     "fd[13]: JS.Muldrop.1159 FOUND\n",
			expected:  Found,
			signature: "JS.Muldrop.1159",
		},
		{
			name:      "Standard Path OK",
			input:     "/tmp/test.eml: OK\n",
			expected:  Ok,
			signature: "",
		},
		{
			name:      "Standard Path Virus",
			input:     "/tmp/virus.eml: Eicar-Test FOUND\n",
			expected:  Found,
			signature: "Eicar-Test",
		},
		{
			name:      "Error Response",
			input:     "file.eml: some_error ERROR\n",
			expected:  Error,
			signature: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, c := net.Pipe()
			go func() {
				s.Write([]byte(tt.input))
				s.Close()
			}()

			res, err := processResponse(c)
			c.Close()

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(res) == 0 {
				t.Fatal("no response parsed")
			}

			r := res[0]
			if r.Status != tt.expected {
				t.Errorf("status mismatch: got %v, want %v", r.Status, tt.expected)
			}
			if tt.expected == Found && r.Signature != tt.signature {
				t.Errorf("signature mismatch: got %q, want %q", r.Signature, tt.signature)
			}
		})
	}
}

func TestStreamScan_Protocol(t *testing.T) {
	f, _ := os.CreateTemp("", "clamd_proto")
	defer os.Remove(f.Name())

	content := bytes.Repeat([]byte("B"), 1536)
	f.Write(content)
	f.Seek(0, 0)
	defer f.Close()

	server, client := net.Pipe()
	var captured bytes.Buffer
	done := make(chan struct{})

	go func() {
		io.Copy(&captured, server)
		close(done)
	}()

	err := streamScan(client, f)
	client.Close()
	<-done

	if err != nil {
		t.Fatalf("streamScan failed: %v", err)
	}

	data := captured.Bytes()

	if !bytes.HasPrefix(data, cmdINSTREAM) {
		t.Error("cmdINSTREAM prefix missing")
	}

	if !bytes.HasSuffix(data, termStream) {
		t.Error("terminal null-chunk missing")
	}
}

func TestPoolsEfficiency(t *testing.T) {
	msg := "test_file: OK\n"

	for i := 0; i < 10; i++ {
		s, c := net.Pipe()
		go func() {
			s.Write([]byte(msg))
			s.Close()
		}()

		res, err := processResponse(c)
		c.Close()

		if err != nil || len(res) == 0 || res[0].Raw() != "test_file: OK" {
			t.Fatalf("Pool corruption or parse error at iteration %d", i)
		}
	}
}
