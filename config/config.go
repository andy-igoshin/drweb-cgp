package config

import (
	"flag"
	"strings"
	"time"
)

type Config struct {
	Debug     bool
	Host      string
	Transport string
	Timeout   time.Duration
}

const (
	TrUnix = "unix"
	TrTcp  = "tcp"
)

var config *Config

func init() {
	config = New()
}

func Debug() bool {
	return config.Debug
}

func Host() string {
	return config.Host
}

func New() *Config {

	config := new(Config)

	flag.BoolVar(&config.Debug, "debug", false, "Export debug information (for developers)")
	flag.StringVar(&config.Host, "host", "/run/.com.drweb.clamd", "DrWeb host to connect")
	flag.DurationVar(&config.Timeout, "timeout", 180*time.Second, "DrWeb request timeout")

	flag.Parse()

	if strings.HasPrefix(config.Host, "/") {
		config.Transport = TrUnix
	} else {
		config.Transport = TrTcp
	}

	if config.Timeout < time.Second {
		config.Timeout *= time.Second
	}

	return config
}

func Timeout() time.Duration {
	return config.Timeout
}

func Transport() string {
	return config.Transport
}
