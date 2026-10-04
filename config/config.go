package config

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Данные заполняются при сборке через -ldflags
var (
	Version   = "dev"
	Commit    = "none"
	BuildTime = "unknown"
)

type TrType int

const (
	TrUnknown TrType = iota
	TrUnix
	TrTcp
)

type Config struct {
	Host          string        `yaml:"host"`
	Timeout       time.Duration `yaml:"timeout"`
	MirrorTo      []string      `yaml:"mirror-to"`
	Notifications struct {
		From          string `yaml:"from"`
		Authenticated bool   `yaml:"authenticated"`
		External      struct {
			Enable  bool `yaml:"enable"`
			Domains struct {
				Allow []string `yaml:"allow"`
				Deny  []string `yaml:"deny"`
			} `yaml:"domains"`
		} `yaml:"external"`
	} `yaml:"notifications"`

	allowMap map[string]struct{}
	denyMap  map[string]struct{}

	debug       bool
	transport   TrType
	showVersion bool
}

var configInstance *Config

func Debug() bool {
	return configInstance.debug
}

func GetVersion() string {
	return fmt.Sprintf("drweb-cgp\n  version:  %s\n  commit:   %s\n  built:    %s\n",
		Version, Commit, BuildTime)
}

func Host() string {
	return configInstance.Host
}

func IsAuthNotifyEnabled() bool {
	return configInstance.Notifications.Authenticated
}

func IsDomainAllowed(domain string) bool {
	domain = strings.ToLower(strings.TrimSpace(domain))

	if _, denied := configInstance.denyMap[domain]; denied {
		return false
	}

	if len(configInstance.allowMap) == 0 {
		return true
	}

	_, allowed := configInstance.allowMap[domain]
	return allowed
}

func IsExternalNotifyEnabled() bool {
	return configInstance.Notifications.External.Enable
}

func MirrorTo() []string {
	return configInstance.MirrorTo
}

func New(args []string) (*Config, error) {
	fs := flag.NewFlagSet("drweb-cgp", flag.ContinueOnError)

	var dir string
	if exePath, err := os.Executable(); err == nil {
		dir = filepath.Dir(exePath)
	} else {
		dir = "."
	}

	var configPath string
	var configDump bool
	var configTest bool
	var debug bool
	var version bool

	fs.StringVar(&configPath, "config", filepath.Join(dir, "drweb-cgp.yml"), "Path to configuration file")
	fs.BoolVar(&configDump, "configdump", false, "Dump configuration and exit")
	fs.BoolVar(&configTest, "configtest", false, "Test configuration syntax and exit")
	fs.BoolVar(&debug, "debug", false, "Enable debug logging")
	fs.BoolVar(&version, "version", false, "Print version and exit")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	if version {
		return &Config{showVersion: true}, nil
	}

	c, err := loadFile(configPath)
	if err != nil {
		return nil, err
	}

	c.debug = debug
	c.showVersion = version

	if configTest {
		fmt.Println("config: Syntax OK")
		os.Exit(0)
	}

	if configDump {
		dumpConfig(c)
		os.Exit(0)
	}

	return c, nil
}

func NotifyFrom() string {
	return configInstance.Notifications.From
}

func Proto() string {
	if configInstance.transport == TrUnix {
		return "unix"
	}
	return "tcp"
}

func SetGlobal(c *Config) {
	configInstance = c
}

func ShowVersion() bool {
	return configInstance.showVersion
}

func Timeout() time.Duration {
	return configInstance.Timeout
}

func Transport() TrType {
	return configInstance.transport
}
