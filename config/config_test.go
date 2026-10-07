package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeTestConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "drweb-cgp.yml")
	data := "host: \"/run/.com.drweb.clamd\"\ntimeout: 5s\n"
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestNew_ConfigTest_ReturnsFlag(t *testing.T) {
	cfg := writeTestConfig(t)

	c, err := New([]string{"-config", cfg, "-configtest"})
	if err != nil {
		t.Fatalf("New(-configtest) error: %v", err)
	}
	if c == nil {
		t.Fatal("New(-configtest) returned nil config")
	}
	if !c.configTest {
		t.Errorf("expected configTest=true, got %+v", c)
	}
}

func TestNew_ConfigDump_ReturnsFlag(t *testing.T) {
	cfg := writeTestConfig(t)

	c, err := New([]string{"-config", cfg, "-configdump"})
	if err != nil {
		t.Fatalf("New(-configdump) error: %v", err)
	}
	if c == nil {
		t.Fatal("New(-configdump) returned nil config")
	}
	if !c.configDump {
		t.Errorf("expected configDump=true, got %+v", c)
	}
}

func TestNew_Version_ReturnsFlagWithoutConfigFile(t *testing.T) {
	// -version должен работать без существующего конфигурационного файла.
	c, err := New([]string{"-config", "/nonexistent/drweb-cgp.yml", "-version"})
	if err != nil {
		t.Fatalf("New(-version) error: %v", err)
	}
	if c == nil {
		t.Fatal("New(-version) returned nil config")
	}
	if !c.showVersion {
		t.Errorf("expected showVersion=true, got %+v", c)
	}
}

func TestNew_MissingConfigFile_Error(t *testing.T) {
	_, err := New([]string{"-config", "/nonexistent/drweb-cgp.yml"})
	if err == nil {
		t.Error("expected error for missing config file, got nil")
	}
}

func TestNew_DefaultTimeout_AppliedWhenMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "drweb-cgp.yml")
	// timeout не задан — должен примениться defaultTimeout.
	data := "host: \"/run/.com.drweb.clamd\"\n"
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	c, err := New([]string{"-config", path})
	if err != nil {
		t.Fatalf("New error: %v", err)
	}

	if c.Timeout != defaultTimeout {
		t.Errorf("expected default timeout %v, got %v", defaultTimeout, c.Timeout)
	}
}

func TestNew_ExplicitTimeout_Preserved(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "drweb-cgp.yml")
	data := "host: \"/run/.com.drweb.clamd\"\ntimeout: 7s\n"
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	c, err := New([]string{"-config", path})
	if err != nil {
		t.Fatalf("New error: %v", err)
	}

	if c.Timeout != 7*time.Second {
		t.Errorf("expected explicit timeout 7s, got %v", c.Timeout)
	}
}

func TestEnsurePort(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"hostname no port", "mail.example.com", "mail.example.com:3310"},
		{"IPv4 no port", "1.2.3.4", "1.2.3.4:3310"},
		{"IPv4 with port", "1.2.3.4:9999", "1.2.3.4:9999"},
		{"hostname with port", "mail.example.com:1234", "mail.example.com:1234"},
		{"bare IPv6 literal", "::1", "[::1]:3310"},
		{"full IPv6 literal", "2001:db8::1", "[2001:db8::1]:3310"},
		{"IPv6 with port", "[::1]:9999", "[::1]:9999"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ensurePort(tt.in); got != tt.want {
				t.Errorf("ensurePort(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestPostProcess_Transport(t *testing.T) {
	tests := []struct {
		name      string
		host      string
		wantProto TrType
		wantHost  string
	}{
		{"unix path", "/run/.com.drweb.clamd", TrUnix, "/run/.com.drweb.clamd"},
		{"hostname", "mail.example.com", TrTcp, "mail.example.com:3310"},
		{"ipv6 bare", "::1", TrTcp, "[::1]:3310"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Config{Host: tt.host}
			c.postProcess()

			if c.transport != tt.wantProto {
				t.Errorf("transport = %v, want %v", c.transport, tt.wantProto)
			}
			if c.Host != tt.wantHost {
				t.Errorf("host = %q, want %q", c.Host, tt.wantHost)
			}
		})
	}
}
