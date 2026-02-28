package config

import (
	"fmt"
	"flag"
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
	Debug         bool
	Host          string
	MirrorDiscard bool
	MirrorTo      []string
	transport     TrType
	timeout       time.Duration
	version       bool
}

const port = ":3310"

var configInstance *Config

func Debug() bool {
	return configInstance.Debug
}

func Host() string {
	return configInstance.Host
}

func GetVersion() string {
	return fmt.Sprintf("drweb-cgp helper\n  Version:  %s\n  Commit:   %s\n  Built:    %s\n",
		Version, Commit, BuildTime)
}

func New(args []string) (*Config, error) {
	c := &Config{}

	fs := flag.NewFlagSet("drweb-cgp", flag.ContinueOnError)

	var mirrorTo string

	fs.BoolVar(&c.Debug, "debug", false, "Export debug information (for developers)")
	fs.StringVar(&c.Host, "host", "/run/.com.drweb.clamd", "DrWeb host to connect")
	fs.StringVar(&mirrorTo, "mirror-to", "", "Mirror selected messages to email")
	fs.DurationVar(&c.timeout, "timeout", 180*time.Second, "DrWeb request timeout")
	fs.BoolVar(&c.version, "version", false, "print version and exit")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	if err := c.postProcess(mirrorTo); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Config) postProcess(mirrorTo string) error {
	if strings.HasPrefix(c.Host, "/") {
		c.transport = TrUnix
	} else {
		c.transport = TrTcp
		if !strings.Contains(c.Host, ":") {
			c.Host += port
		}
	}

	if len(mirrorTo) > 0 {
		c.MirrorDiscard = true
		cleanMirror := strings.ReplaceAll(mirrorTo, " ", "")
		c.MirrorTo = strings.Split(cleanMirror, ",")
	}

	return nil
}

func MirrorDiscard() bool {
	return configInstance.MirrorDiscard
}

func MirrorTo() []string {
	return configInstance.MirrorTo
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
