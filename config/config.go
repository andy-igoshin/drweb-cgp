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
	configDump  bool
	configTest  bool
}

var configInstance *Config

func ConfigDump() bool {
	if configInstance == nil {
		return false
	}
	return configInstance.configDump
}

func ConfigTest() bool {
	if configInstance == nil {
		return false
	}
	return configInstance.configTest
}

func Debug() bool {
	if configInstance == nil {
		return false
	}
	return configInstance.debug
}

func GetVersion() string {
	return fmt.Sprintf("drweb-cgp\n  version:  %s\n  commit:   %s\n  built:    %s\n",
		Version, Commit, BuildTime)
}

func Host() string {
	if configInstance == nil {
		return ""
	}
	return configInstance.Host
}

func IsAuthNotifyEnabled() bool {
	if configInstance == nil {
		return false
	}
	return configInstance.Notifications.Authenticated
}

func IsDomainAllowed(domain string) bool {
	if configInstance == nil {
		return false
	}

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
	if configInstance == nil {
		return false
	}
	return configInstance.Notifications.External.Enable
}

func MirrorTo() []string {
	if configInstance == nil {
		return []string{}
	}
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

	var configpath string
	var configdump bool
	var configtest bool
	var debug bool
	var version bool

	fs.StringVar(&configpath, "config", filepath.Join(dir, "drweb-cgp.yml"), "Path to configuration file")
	fs.BoolVar(&configdump, "configdump", false, "Dump configuration and exit")
	fs.BoolVar(&configtest, "configtest", false, "Test configuration syntax and exit")
	fs.BoolVar(&debug, "debug", false, "Enable debug logging")
	fs.BoolVar(&version, "version", false, "Print version and exit")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	if version {
		return &Config{showVersion: true}, nil
	}

	c, err := loadFile(configpath)
	if err != nil {
		return nil, err
	}

	c.debug = debug
	c.showVersion = version

	if configdump {
		dumpConfig(c)
		c.configDump = true
		return c, nil
	}

	if configtest {
		fmt.Println("config: Syntax OK")
		c.configTest = true
		return c, nil
	}

	return c, nil
}

func NotifyFrom() string {
	if configInstance == nil {
		return ""
	}
	return configInstance.Notifications.From
}

func Proto() string {
	if configInstance == nil {
		return ""
	}

	if configInstance.transport == TrUnix {
		return "unix"
	}
	return "tcp"
}

func SetGlobal(c *Config) {
	configInstance = c
}

func ShowVersion() bool {
	if configInstance == nil {
		return false
	}
	return configInstance.showVersion
}

func Timeout() time.Duration {
	if configInstance == nil {
		return defaultTimeout
	}
	return configInstance.Timeout
}

func Transport() TrType {
	if configInstance == nil {
		return TrUnknown
	}
	return configInstance.transport
}
