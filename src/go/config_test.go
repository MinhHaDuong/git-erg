package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTDDAnchor_PostTalkRejectedWithoutErgrc(t *testing.T) {
	dir := t.TempDir()
	content := "%erg 0.1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\nLabel: post-talk\n\n--- log ---\n--- body ---\n"
	writeErg(t, dir, "0001-test.erg", content)

	tickets, parseErrs := loadErgs(dir)
	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	errs := validateCorpus(tickets, parseErrs, cfg)

	if !errsContain(errs, "unknown Label value") {
		t.Errorf("expected 'unknown Label value' error for post-talk with no .ergrc, got: %v", errs)
	}
}

func TestConfig_CustomErgrc(t *testing.T) {
	dir := t.TempDir()
	ergrc := "[labels]\nmy-label\n"
	if err := os.WriteFile(filepath.Join(dir, ".ergrc"), []byte(ergrc), 0644); err != nil {
		t.Fatal(err)
	}
	content := "%erg 0.1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\nLabel: my-label\n\n--- log ---\n--- body ---\n"
	writeErg(t, dir, "0001-test.erg", content)

	tickets, parseErrs := loadErgs(dir)
	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	errs := validateCorpus(tickets, parseErrs, cfg)
	if len(errs) > 0 {
		t.Errorf("expected no errors with custom .ergrc, got: %v", errs)
	}
}

func TestConfig_MissingErgrcFallback(t *testing.T) {
	dir := t.TempDir()
	content := "%erg 0.1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\nLabel: needs-human\n\n--- log ---\n--- body ---\n"
	writeErg(t, dir, "0001-test.erg", content)

	tickets, parseErrs := loadErgs(dir)
	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	errs := validateCorpus(tickets, parseErrs, cfg)
	if len(errs) > 0 {
		t.Errorf("expected no errors with default labels, got: %v", errs)
	}
}

func TestConfig_EmptyLabelsSection(t *testing.T) {
	dir := t.TempDir()
	ergrc := "[labels]\n"
	if err := os.WriteFile(filepath.Join(dir, ".ergrc"), []byte(ergrc), 0644); err != nil {
		t.Fatal(err)
	}
	content := "%erg 0.1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\nLabel: needs-human\n\n--- log ---\n--- body ---\n"
	writeErg(t, dir, "0001-test.erg", content)

	tickets, parseErrs := loadErgs(dir)
	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	errs := validateCorpus(tickets, parseErrs, cfg)
	if !errsContain(errs, "unknown Label value") {
		t.Errorf("expected 'unknown Label value' error with empty [labels], got: %v", errs)
	}
}

func TestConfig_LabelsAbsentUpdatePresent(t *testing.T) {
	dir := t.TempDir()
	ergrc := "[update]\nurl = https://example.com/erg\n"
	if err := os.WriteFile(filepath.Join(dir, ".ergrc"), []byte(ergrc), 0644); err != nil {
		t.Fatal(err)
	}
	content := "%erg 0.1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\nLabel: needs-human\n\n--- log ---\n--- body ---\n"
	writeErg(t, dir, "0001-test.erg", content)

	tickets, parseErrs := loadErgs(dir)
	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	errs := validateCorpus(tickets, parseErrs, cfg)
	if len(errs) > 0 {
		t.Errorf("expected no errors (labels fallback to defaults), got: %v", errs)
	}
	if cfg.UpdateURL != "https://example.com/erg" {
		t.Errorf("expected UpdateURL from config, got: %q", cfg.UpdateURL)
	}
}

func TestConfig_CommentsAndBlankLines(t *testing.T) {
	dir := t.TempDir()
	ergrc := "# comment\n\n[labels]\n# another comment\nneeds-human\n\ndeferred\n"
	if err := os.WriteFile(filepath.Join(dir, ".ergrc"), []byte(ergrc), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if !cfg.LabelsSection {
		t.Error("expected LabelsSection to be true")
	}
	if len(cfg.Labels) != 2 {
		t.Errorf("expected 2 labels, got %d: %v", len(cfg.Labels), cfg.Labels)
	}
}

func TestLoadConfig_NoFile(t *testing.T) {
	dir := t.TempDir()
	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg != nil {
		t.Errorf("expected nil config when no .ergrc, got: %+v", cfg)
	}
}

func TestEffectiveLabelSet_WithConfig(t *testing.T) {
	cfg := &Config{LabelsSection: true, Labels: []string{"alpha", "beta"}}
	labels := effectiveLabelSet(cfg)
	if !labels["alpha"] || !labels["beta"] {
		t.Errorf("expected alpha and beta in label set, got: %v", labels)
	}
	if labels["needs-human"] {
		t.Error("should not contain default labels when config has [labels]")
	}
}

func TestEffectiveLabelSet_NilConfig(t *testing.T) {
	labels := effectiveLabelSet(nil)
	if !labels["needs-human"] || !labels["deferred"] {
		t.Errorf("expected default labels, got: %v", labels)
	}
	if labels["post-talk"] || labels["post-conference"] {
		t.Error("defaults should not include post-talk or post-conference")
	}
}

func TestEffectiveLabelSet_EmptyLabels(t *testing.T) {
	cfg := &Config{LabelsSection: true, Labels: []string{}}
	labels := effectiveLabelSet(cfg)
	if len(labels) != 0 {
		t.Errorf("expected empty label set, got: %v", labels)
	}
}

func TestConfig_Precedence_UpdateURL(t *testing.T) {
	dir := t.TempDir()
	ergrc := "[update]\nurl = https://from-config.example.com/erg\n"
	if err := os.WriteFile(filepath.Join(dir, ".ergrc"), []byte(ergrc), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg.UpdateURL != "https://from-config.example.com/erg" {
		t.Errorf("expected config URL, got: %q", cfg.UpdateURL)
	}
}

func TestLoadConfig_MinimalLabelsSection(t *testing.T) {
	dir := t.TempDir()
	content := strings.Repeat("[labels]\n", 1)
	if err := os.WriteFile(filepath.Join(dir, ".ergrc"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("loadConfig should not error on valid file: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if !cfg.LabelsSection {
		t.Error("expected LabelsSection true")
	}
}

func TestUpdateURL_EnvVarOverridesConfig(t *testing.T) {
	dir := t.TempDir()
	ergrc := "[update]\nurl = https://from-config.example.com/erg\n"
	if err := os.WriteFile(filepath.Join(dir, ".ergrc"), []byte(ergrc), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg.UpdateURL != "https://from-config.example.com/erg" {
		t.Fatalf("config URL not loaded: %q", cfg.UpdateURL)
	}

	const def = "origin"
	envURL := "https://from-env.example.com/erg"

	// Env var set -> wins over both config and default.
	if got := resolveUpdateRemote(envURL, cfg.UpdateURL, def); got != envURL {
		t.Errorf("env var should override config: got %q, want %q", got, envURL)
	}
	// Env var unset -> config wins over default.
	if got := resolveUpdateRemote("", cfg.UpdateURL, def); got != cfg.UpdateURL {
		t.Errorf("config should override default when env unset: got %q, want %q", got, cfg.UpdateURL)
	}
	// Env and config unset -> compiled-in default.
	if got := resolveUpdateRemote("", "", def); got != def {
		t.Errorf("default should apply when env and config unset: got %q, want %q", got, def)
	}
}
