package main

import (
	"os"
	"path/filepath"
	"strings"
)

// Config holds project-level settings loaded from tickets/.ergrc.
type Config struct {
	LabelsSection bool
	Labels        []string
	UpdateURL     string
}

var defaultLabels = []string{"needs-human", "deferred"}

// loadConfig reads and parses .ergrc from dir. Returns (nil, nil) when
// the file is absent — callers fall back to built-in defaults.
func loadConfig(dir string) (*Config, error) {
	data, err := os.ReadFile(filepath.Join(dir, ".ergrc"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return parseErgrc(string(data)), nil
}

func parseErgrc(content string) *Config {
	cfg := &Config{}
	section := ""
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			section = trimmed[1 : len(trimmed)-1]
			if section == "labels" {
				cfg.LabelsSection = true
				cfg.Labels = []string{}
			}
			continue
		}
		switch section {
		case "labels":
			cfg.Labels = append(cfg.Labels, trimmed)
		case "update":
			if k, v, ok := parseINIKeyValue(trimmed); ok && k == "url" {
				cfg.UpdateURL = v
			}
		}
	}
	return cfg
}

func parseINIKeyValue(line string) (string, string, bool) {
	idx := strings.Index(line, "=")
	if idx < 0 {
		return "", "", false
	}
	return strings.TrimSpace(line[:idx]), strings.TrimSpace(line[idx+1:]), true
}

// effectiveLabelSet returns the label vocabulary as a map[string]bool.
func effectiveLabelSet(cfg *Config) map[string]bool {
	var labels []string
	if cfg != nil && cfg.LabelsSection {
		labels = cfg.Labels
	} else {
		labels = defaultLabels
	}
	m := make(map[string]bool, len(labels))
	for _, l := range labels {
		m[l] = true
	}
	return m
}
