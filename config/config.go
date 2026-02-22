package config

import (
	"flag"
	"strings"
	"time"
)

type Config struct {
	Debug         bool
	Host          string
	MirrorDiscard bool
	MirrorTo      []string
	Transport     string
	Timeout       time.Duration
}

const (
	TrUnix = "unix"
	TrTcp  = "tcp"
)

const port = ":3310"

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

	var mirrorTo string

	flag.BoolVar(&config.Debug, "debug", false, "Export debug information (for developers)")
	flag.StringVar(&config.Host, "host", "/run/.com.drweb.clamd", "DrWeb host to connect")
	flag.StringVar(&mirrorTo, "mirror-to", "", "Mirror selected messages to email")
	flag.DurationVar(&config.Timeout, "timeout", 180*time.Second, "DrWeb request timeout")

	flag.Parse()

	if strings.HasPrefix(config.Host, "/") {
		config.Transport = TrUnix
	} else {
		config.Transport = TrTcp

		if !strings.Contains(config.Host, ":") {
			config.Host += port
		}
	}

	if len(mirrorTo) > 0 {
		config.MirrorDiscard = true
		config.MirrorTo = strings.Split(strings.ReplaceAll(mirrorTo, " ", ""), ",")
	}

	return config
}

func MirrorDiscard() bool {
	return config.MirrorDiscard
}

func MirrorTo() []string {
	return config.MirrorTo
}

func Timeout() time.Duration {
	return config.Timeout
}

func Transport() string {
	return config.Transport
}
