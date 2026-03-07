package config

import (
	"flag"
	"fmt"
	"strings"
	"time"
)

// Эти значения будут перезаписаны при сборке через -ldflags
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
	debug     bool
	host      string
	mirrorTo  []string
	transport TrType
	timeout   time.Duration
	version   bool
}

const port = ":3310"

var configInstance *Config

func Debug() bool {
	return configInstance.debug
}

func Host() string {
	return configInstance.host
}

func GetVersion() string {
	return fmt.Sprintf("drweb-cgp\n  version:  %s\n  commit:   %s\n  built:    %s\n",
		Version, Commit, BuildTime)
}

func New(args []string) (*Config, error) {
	c := &Config{}

	fs := flag.NewFlagSet("drweb-cgp", flag.ContinueOnError)

	var mirrorTo string

	fs.BoolVar(&c.debug, "debug", false, "Export debug information (for developers)")
	fs.StringVar(&c.host, "host", "/run/.com.drweb.clamd", "DrWeb host to connect")
	fs.StringVar(&mirrorTo, "mirror-to", "", "Mirror selected messages to email")
	fs.DurationVar(&c.timeout, "timeout", 180*time.Second, "DrWeb request timeout")
	fs.BoolVar(&c.version, "version", false, "Print version and exit")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	if err := c.postProcess(mirrorTo); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Config) postProcess(mirrorTo string) error {
	if strings.HasPrefix(c.host, "/") {
		c.transport = TrUnix
	} else {
		c.transport = TrTcp
		if !strings.Contains(c.host, ":") {
			c.host += port
		}
	}

	if len(mirrorTo) > 0 {
		cleanMirror := strings.ReplaceAll(mirrorTo, " ", "")
		c.mirrorTo = strings.Split(cleanMirror, ",")
	}

	return nil
}

func MirrorTo() []string {
	return configInstance.mirrorTo
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
	return configInstance.version
}

func Timeout() time.Duration {
	return configInstance.timeout
}

func Transport() TrType {
	return configInstance.transport
}
