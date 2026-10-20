package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseChecksums(t *testing.T) {
	a := strings.Repeat("a", 64)
	b := strings.Repeat("B", 64)
	data := a + "  xboard-node-linux-amd64\n" +
		b + " *xbctl-linux-amd64\n" +
		"garbage line\n" +
		"abc  too-short-digest\n"
	sums := parseChecksums([]byte(data))
	if len(sums) != 2 {
		t.Fatalf("expected 2 entries, got %d: %v", len(sums), sums)
	}
	if sums["xboard-node-linux-amd64"] != a {
		t.Errorf("unexpected digest for xboard-node: %q", sums["xboard-node-linux-amd64"])
	}
	if sums["xbctl-linux-amd64"] != strings.ToLower(b) {
		t.Errorf("binary-mode entry not parsed or not lowercased: %q", sums["xbctl-linux-amd64"])
	}
}

func TestConfigInitMachineAutoNode(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yml")
	err := runConfigInit([]string{
		"--mode", "machine",
		"--panel-url", "https://panel.example.com",
		"--machine-id", "4",
		"--kernel", "singbox",
		"--output", cfgPath,
		"--auto-node-host", "203.0.113.7",
		"--auto-node", "false",
	})
	if err != nil {
		t.Fatalf("runConfigInit: %v", err)
	}

	root, err := loadWritableRootConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	m := root.Instances[0].Machine
	if m == nil || m.AutoNodeHost != "203.0.113.7" || m.AutoNode == nil || *m.AutoNode {
		t.Fatalf("auto node settings not written: %+v", m)
	}

	// Rewriting the config (as bind/remove do) must keep the settings.
	if err := writeRootConfig(cfgPath, root); err != nil {
		t.Fatal(err)
	}
	root, err = loadWritableRootConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if m := root.Instances[0].Machine; m.AutoNodeHost != "203.0.113.7" || m.AutoNode == nil || *m.AutoNode {
		t.Errorf("auto node settings lost on rewrite: %+v", m)
	}
}

func TestVerifyChecksum(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bin")
	content := []byte("hello xboard")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(content)
	good := hex.EncodeToString(sum[:])

	if err := verifyChecksum(path, "bin", map[string]string{"bin": good}); err != nil {
		t.Errorf("expected match, got %v", err)
	}
	if err := verifyChecksum(path, "bin", map[string]string{"bin": strings.Repeat("0", 64)}); err == nil {
		t.Error("expected mismatch error")
	}
	if err := verifyChecksum(path, "bin", map[string]string{}); err == nil {
		t.Error("expected missing-entry error")
	}
}
