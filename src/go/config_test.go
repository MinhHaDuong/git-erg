package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTDDAnchor_PostTalkRejectedWithoutErgrc(t *testing.T) {
	dir := t.TempDir()
	content := "%erg 0.1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\nTag: post-talk\n\n--- log ---\n--- body ---\n"
	writeErg(t, dir, "0001-test.erg", content)

	tickets, parseErrs := loadErgs(dir)
	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	errs := validateCorpus(tickets, parseErrs, cfg)

	if !errsContain(errs, "unknown Tag value") {
		t.Errorf("expected 'unknown Tag value' error for post-talk with no .ergrc, got: %v", errs)
	}
}

func TestConfig_CustomErgrc(t *testing.T) {
	dir := t.TempDir()
	ergrc := "[tags]\nmy-tag\n"
	if err := os.WriteFile(filepath.Join(dir, ".ergrc"), []byte(ergrc), 0644); err != nil {
		t.Fatal(err)
	}
	content := "%erg 0.1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\nTag: my-tag\n\n--- log ---\n--- body ---\n"
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
	content := "%erg 0.1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\nTag: needs-human\n\n--- log ---\n--- body ---\n"
	writeErg(t, dir, "0001-test.erg", content)

	tickets, parseErrs := loadErgs(dir)
	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	errs := validateCorpus(tickets, parseErrs, cfg)
	if len(errs) > 0 {
		t.Errorf("expected no errors with default tags, got: %v", errs)
	}
}

func TestConfig_EmptyTagsSection(t *testing.T) {
	dir := t.TempDir()
	ergrc := "[tags]\n"
	if err := os.WriteFile(filepath.Join(dir, ".ergrc"), []byte(ergrc), 0644); err != nil {
		t.Fatal(err)
	}
	content := "%erg 0.1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\nTag: needs-human\n\n--- log ---\n--- body ---\n"
	writeErg(t, dir, "0001-test.erg", content)

	tickets, parseErrs := loadErgs(dir)
	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	errs := validateCorpus(tickets, parseErrs, cfg)
	if !errsContain(errs, "unknown Tag value") {
		t.Errorf("expected 'unknown Tag value' error with empty [tags], got: %v", errs)
	}
}

func TestConfig_TagsAbsentUpdatePresent(t *testing.T) {
	dir := t.TempDir()
	ergrc := "[update]\nurl = https://example.com/erg\n"
	if err := os.WriteFile(filepath.Join(dir, ".ergrc"), []byte(ergrc), 0644); err != nil {
		t.Fatal(err)
	}
	content := "%erg 0.1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\nTag: needs-human\n\n--- log ---\n--- body ---\n"
	writeErg(t, dir, "0001-test.erg", content)

	tickets, parseErrs := loadErgs(dir)
	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	errs := validateCorpus(tickets, parseErrs, cfg)
	if len(errs) > 0 {
		t.Errorf("expected no errors (tags fallback to defaults), got: %v", errs)
	}
	if cfg.UpdateURL != "https://example.com/erg" {
		t.Errorf("expected UpdateURL from config, got: %q", cfg.UpdateURL)
	}
}

func TestConfig_CommentsAndBlankLines(t *testing.T) {
	dir := t.TempDir()
	ergrc := "# comment\n\n[tags]\n# another comment\nneeds-human\n\ndeferred\n"
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
	if !cfg.TagsSection {
		t.Error("expected TagsSection to be true")
	}
	if len(cfg.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d: %v", len(cfg.Tags), cfg.Tags)
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

func TestEffectiveTagSet_WithConfig(t *testing.T) {
	cfg := &Config{TagsSection: true, Tags: []string{"alpha", "beta"}}
	tags := effectiveTagSet(cfg)
	if !tags["alpha"] || !tags["beta"] {
		t.Errorf("expected alpha and beta in tag set, got: %v", tags)
	}
	if tags["needs-human"] {
		t.Error("should not contain default tags when config has [tags]")
	}
}

func TestEffectiveTagSet_NilConfig(t *testing.T) {
	tags := effectiveTagSet(nil)
	if !tags["needs-human"] || !tags["deferred"] {
		t.Errorf("expected default tags, got: %v", tags)
	}
	if tags["post-talk"] || tags["post-conference"] {
		t.Error("defaults should not include post-talk or post-conference")
	}
}

func TestEffectiveTagSet_EmptyTags(t *testing.T) {
	cfg := &Config{TagsSection: true, Tags: []string{}}
	tags := effectiveTagSet(cfg)
	if len(tags) != 0 {
		t.Errorf("expected empty tag set, got: %v", tags)
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

func TestLoadConfig_MinimalTagsSection(t *testing.T) {
	dir := t.TempDir()
	content := strings.Repeat("[tags]\n", 1)
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
	if !cfg.TagsSection {
		t.Error("expected TagsSection true")
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

	const def = "https://default.example.com/erg"
	envURL := "https://from-env.example.com/erg"

	// Env var set → wins over both config and default.
	if got := resolveUpdateURL(envURL, cfg.UpdateURL, def); got != envURL {
		t.Errorf("env var should override config: got %q, want %q", got, envURL)
	}
	// Env var unset → config wins over default.
	if got := resolveUpdateURL("", cfg.UpdateURL, def); got != cfg.UpdateURL {
		t.Errorf("config should override default when env unset: got %q, want %q", got, cfg.UpdateURL)
	}
	// Env and config unset → compiled-in default.
	if got := resolveUpdateURL("", "", def); got != def {
		t.Errorf("default should apply when env and config unset: got %q, want %q", got, def)
	}
}
