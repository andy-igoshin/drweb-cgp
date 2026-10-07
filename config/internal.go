package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

func dumpConfig(c *Config) {
	yml, err := yaml.Marshal(c)
	if err != nil {
		fmt.Println("config:", err)
		return
	}
	fmt.Print(string(yml))
	fmt.Printf("debug: %v\n", c.debug)
	fmt.Println("showVersion:", c.showVersion)
}

const (
	defaultHost    = "/run/.com.drweb.clamd"
	defaultTimeout = 180 * time.Second
)

func loadFile(path string) (*Config, error) {
	c := &Config{
		Host: defaultHost,
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	if err := yaml.Unmarshal(data, c); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}

	// Если timeout не задан в конфиге (нулевое значение), применяем default.
	if c.Timeout == 0 {
		c.Timeout = defaultTimeout
	}

	c.postProcess()
	return c, nil
}

func (c *Config) postProcess() {
	if strings.HasPrefix(c.Host, "/") {
		c.transport = TrUnix
	} else {
		c.transport = TrTcp
		if !strings.Contains(c.Host, ":") {
			c.Host += ":3310"
		}
	}

	c.allowMap = make(map[string]struct{})
	for _, d := range c.Notifications.External.Domains.Allow {
		d = strings.ToLower(strings.TrimSpace(d))
		if d != "" {
			c.allowMap[d] = struct{}{}
		}
	}

	c.denyMap = make(map[string]struct{})
	for _, d := range c.Notifications.External.Domains.Deny {
		d = strings.ToLower(strings.TrimSpace(d))
		if d != "" {
			c.denyMap[d] = struct{}{}
		}
	}
}
