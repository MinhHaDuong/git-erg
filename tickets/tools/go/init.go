package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	agentsPointerLine = "git-erg local tickets: see tickets/README.md"
	gitignoreLine     = "tickets/tools/go/erg"

	hookStartMarker = "# --- git-erg: begin managed block ---"
	hookEndMarker   = "# --- git-erg: end managed block ---"

	manifestRelPath = "tickets/.erg-bootstrap-manifest.json"
)

var managedAssetPaths = []string{
	"tickets/README.md",
	"tickets/spec-erg-v1.md",
	"tickets/integration/manifest.json",
	"tickets/integration/settings.json",
	"tickets/integration/hooks/pre-commit",
	"tickets/integration/skills/ticket-close/SKILL.md",
	"tickets/integration/skills/ticket-new/SKILL.md",
	"tickets/integration/skills/ticket-ready/SKILL.md",
}

type bootstrapManifest struct {
	Version            int      `json:"version"`
	ManagedFiles       []string `json:"managed_files"`
	AddedAgentsPointer bool     `json:"added_agents_pointer"`
	AddedGitignoreLine bool     `json:"added_gitignore_line"`
	AddedHookBlock     bool     `json:"added_hook_block"`
}

func cmdInit(args []string) int {
	root := "."
	if len(args) > 0 {
		root = args[0]
	}

	fmt.Printf("Plan: initialize git-erg support in %s\n", root)
	fmt.Printf("- refresh %d managed files under tickets/\n", len(managedAssetPaths))
	fmt.Println("- append AGENTS.md entry, .gitignore entry, and pre-commit hook block when absent")

	manifest := bootstrapManifest{
		Version:      1,
		ManagedFiles: append([]string(nil), managedAssetPaths...),
	}

	created := 0
	refreshed := 0
	unchanged := 0

	for _, rel := range managedAssetPaths {
		content, ok := bootstrapAsset(rel)
		if !ok {
			fmt.Fprintf(os.Stderr, "init: missing embedded asset: %s\n", rel)
			return 1
		}
		target := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			fmt.Fprintf(os.Stderr, "init: cannot create directory for %s: %v\n", rel, err)
			return 1
		}
		existing, err := os.ReadFile(target)
		exists := err == nil
		if exists && string(existing) == content {
			unchanged++
			continue
		}
		if err := os.WriteFile(target, []byte(content), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "init: cannot write %s: %v\n", rel, err)
			return 1
		}
		if exists {
			refreshed++
		} else {
			created++
		}
	}

	agentsPath := filepath.Join(root, "AGENTS.md")
	addedAgents, err := ensureLine(agentsPath, agentsPointerLine)
	if err != nil {
		fmt.Fprintf(os.Stderr, "init: cannot update AGENTS.md: %v\n", err)
		return 1
	}
	manifest.AddedAgentsPointer = hasLine(agentsPath, agentsPointerLine)

	gitignorePath := filepath.Join(root, ".gitignore")
	addedGitignore, err := ensureLine(gitignorePath, gitignoreLine)
	if err != nil {
		fmt.Fprintf(os.Stderr, "init: cannot update .gitignore: %v\n", err)
		return 1
	}
	manifest.AddedGitignoreLine = hasLine(gitignorePath, gitignoreLine)

	hookPath := filepath.Join(root, ".git", "hooks", "pre-commit")
	hookAsset, ok := bootstrapAsset("tickets/integration/hooks/pre-commit")
	if !ok {
		fmt.Fprintln(os.Stderr, "init: missing embedded hook asset")
		return 1
	}
	addedHook, err := ensureManagedBlock(hookPath, hookStartMarker, hookEndMarker, hookBodyFromAsset(hookAsset))
	if err != nil {
		fmt.Fprintf(os.Stderr, "init: cannot update pre-commit hook: %v\n", err)
		return 1
	}
	manifest.AddedHookBlock = hasManagedBlock(hookPath, hookStartMarker, hookEndMarker)

	if err := writeManifest(root, manifest); err != nil {
		fmt.Fprintf(os.Stderr, "init: cannot write manifest: %v\n", err)
		return 1
	}

	fmt.Println("Applied:")
	fmt.Printf("- files created: %d\n", created)
	fmt.Printf("- files refreshed: %d\n", refreshed)
	fmt.Printf("- files unchanged: %d\n", unchanged)
	agentsWord := "no"
	if addedAgents {
		agentsWord = "yes"
	}
	gitignoreWord := "no"
	if addedGitignore {
		gitignoreWord = "yes"
	}
	hookWord := "no"
	if addedHook {
		hookWord = "yes"
	}
	fmt.Printf("- AGENTS.md entry added: %s\n", agentsWord)
	fmt.Printf("- .gitignore entry added: %s\n", gitignoreWord)
	fmt.Printf("- pre-commit block added: %s\n", hookWord)
	fmt.Printf("- manifest written: %s\n", manifestRelPath)
	return 0
}

func cmdUninstall(args []string) int {
	root := "."
	if len(args) > 0 {
		root = args[0]
	}

	fmt.Printf("Plan: uninstall git-erg managed assets from %s\n", root)
	fmt.Println("- remove managed support files and managed fragments")
	fmt.Println("- preserve user-created ticket files")

	manifest, manifestFound, err := readManifest(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "uninstall: cannot read manifest: %v\n", err)
		return 1
	}
	if !manifestFound {
		manifest = bootstrapManifest{ManagedFiles: append([]string(nil), managedAssetPaths...)}
	}

	removedFiles := 0
	for _, rel := range manifest.ManagedFiles {
		target := filepath.Join(root, filepath.FromSlash(rel))
		if _, err := os.Stat(target); err == nil {
			if err := os.Remove(target); err == nil {
				removedFiles++
			}
		}
	}

	removedAgents := false
	if manifest.AddedAgentsPointer {
		removedAgents, _ = removeLine(filepath.Join(root, "AGENTS.md"), agentsPointerLine)
	}

	removedGitignore := false
	if manifest.AddedGitignoreLine {
		removedGitignore, _ = removeLine(filepath.Join(root, ".gitignore"), gitignoreLine)
	}

	removedHook, err := removeManagedBlock(filepath.Join(root, ".git", "hooks", "pre-commit"), hookStartMarker, hookEndMarker)
	if err != nil {
		fmt.Fprintf(os.Stderr, "uninstall: cannot update pre-commit hook: %v\n", err)
		return 1
	}

	manifestPath := filepath.Join(root, filepath.FromSlash(manifestRelPath))
	_ = os.Remove(manifestPath)

	cleanupIfEmpty(filepath.Join(root, "tickets", "integration", "skills", "ticket-close"))
	cleanupIfEmpty(filepath.Join(root, "tickets", "integration", "skills", "ticket-new"))
	cleanupIfEmpty(filepath.Join(root, "tickets", "integration", "skills", "ticket-ready"))
	cleanupIfEmpty(filepath.Join(root, "tickets", "integration", "skills"))
	cleanupIfEmpty(filepath.Join(root, "tickets", "integration", "hooks"))
	cleanupIfEmpty(filepath.Join(root, "tickets", "integration"))
	cleanupIfEmpty(filepath.Join(root, "tickets"))

	agentsRemovedWord := "no"
	if removedAgents {
		agentsRemovedWord = "yes"
	}
	gitignoreRemovedWord := "no"
	if removedGitignore {
		gitignoreRemovedWord = "yes"
	}
	hookRemovedWord := "no"
	if removedHook {
		hookRemovedWord = "yes"
	}
	fmt.Println("Applied:")
	fmt.Printf("- managed files removed: %d\n", removedFiles)
	fmt.Printf("- AGENTS.md entry removed: %s\n", agentsRemovedWord)
	fmt.Printf("- .gitignore entry removed: %s\n", gitignoreRemovedWord)
	fmt.Printf("- pre-commit block removed: %s\n", hookRemovedWord)
	fmt.Println("- user ticket data preserved under tickets/: yes")
	return 0
}

func ensureLine(path, line string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return true, os.WriteFile(path, []byte(line+"\n"), 0644)
		}
		return false, err
	}
	text := string(data)
	for _, existing := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if existing == line {
			return false, nil
		}
	}
	if len(text) > 0 && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	text += line + "\n"
	return true, os.WriteFile(path, []byte(text), 0644)
}

func removeLine(path, line string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	out := make([]string, 0, len(lines))
	removed := false
	for _, l := range lines {
		if l == line {
			removed = true
			continue
		}
		out = append(out, l)
	}
	if !removed {
		return false, nil
	}
	text := strings.Join(out, "\n")
	text = strings.TrimRight(text, "\n")
	if text != "" {
		text += "\n"
	}
	return true, os.WriteFile(path, []byte(text), 0644)
}

func ensureManagedBlock(path, startMarker, endMarker, body string) (bool, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return false, err
	}
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	if strings.Contains(text, startMarker) && strings.Contains(text, endMarker) {
		return false, nil
	}

	block := startMarker + "\n" + strings.TrimSpace(body) + "\n" + endMarker + "\n"
	if strings.TrimSpace(text) == "" {
		text = block
	} else {
		if !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		text += "\n" + block
	}
	if err := os.WriteFile(path, []byte(text), 0755); err != nil {
		return false, err
	}
	return true, nil
}

func removeManagedBlock(path, startMarker, endMarker string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	start := strings.Index(text, startMarker)
	if start == -1 {
		return false, nil
	}
	endStart := strings.Index(text[start:], endMarker)
	if endStart == -1 {
		return false, nil
	}
	end := start + endStart + len(endMarker)
	if end < len(text) && text[end] == '\n' {
		end++
	}
	trimStart := start
	for trimStart > 0 && text[trimStart-1] == '\n' {
		trimStart--
		if trimStart == 0 || text[trimStart-1] != '\n' {
			break
		}
	}
	newText := text[:trimStart] + text[end:]
	newText = strings.TrimRight(newText, "\n")
	if newText != "" {
		newText += "\n"
	}
	return true, os.WriteFile(path, []byte(newText), 0755)
}

func hookBodyFromAsset(asset string) string {
	text := strings.ReplaceAll(asset, "\r\n", "\n")
	lines := strings.Split(text, "\n")
	if len(lines) > 0 && strings.HasPrefix(lines[0], "#!") {
		lines = lines[1:]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func writeManifest(root string, manifest bootstrapManifest) error {
	path := filepath.Join(root, filepath.FromSlash(manifestRelPath))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0644)
}

func readManifest(root string) (bootstrapManifest, bool, error) {
	path := filepath.Join(root, filepath.FromSlash(manifestRelPath))
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return bootstrapManifest{}, false, nil
		}
		return bootstrapManifest{}, false, err
	}
	var manifest bootstrapManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return bootstrapManifest{}, false, err
	}
	if len(manifest.ManagedFiles) == 0 {
		manifest.ManagedFiles = append([]string(nil), managedAssetPaths...)
	}
	return manifest, true, nil
}

func cleanupIfEmpty(path string) {
	_ = os.Remove(path)
}

func hasLine(path, line string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	for _, existing := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		if existing == line {
			return true
		}
	}
	return false
}

func hasManagedBlock(path, startMarker, endMarker string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	text := string(data)
	start := strings.Index(text, startMarker)
	if start == -1 {
		return false
	}
	end := strings.Index(text[start:], endMarker)
	return end != -1
}
