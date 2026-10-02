package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestInteractiveBrowserSelectionNavigationAndReview(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "docs"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"README.md", "docs/a.md", "docs/b.md"} {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), []byte("Documentation"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	m, err := newSetupBrowserModel(context.Background(), setupSession{output: io.Discard}, root, nil)
	if err != nil {
		t.Fatal(err)
	}
	press := func(code rune) { m.Update(tea.KeyPressMsg{Code: code}) }
	m.list.Select(1)
	press(tea.KeySpace)
	if len(m.files) != 2 {
		t.Fatalf("marked directory: %v", m.files)
	}
	press(tea.KeyEnter)
	if m.current != filepath.Join(root, "docs") {
		t.Fatal("Enter did not open directory")
	}
	press(tea.KeyBackspace)
	if m.current != root || len(m.files) != 2 {
		t.Fatal("selection lost during navigation")
	}
	press(tea.KeyTab)
	press(tea.KeySpace)
	if len(m.files) != 1 {
		t.Fatal("review did not remove selected document")
	}
	m.list.GoToEnd()
	press(tea.KeyEnter)
	if !m.done {
		t.Fatal("continue action did not finish")
	}
}

func TestInteractiveBrowserCancelAndSensitiveContent(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "notes.md"), []byte("OPENAI_API_KEY=sk-test-secret-value"), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newSetupBrowserModel(context.Background(), setupSession{output: io.Discard}, root, nil)
	if err != nil {
		t.Fatal(err)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	if len(m.files) != 0 || m.status == "" {
		t.Fatal("unsafe content accepted or rejection hidden")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !m.cancelled || m.done {
		t.Fatal("cancel did not stop selection")
	}
}

func TestInteractiveBrowserStaysRootedInNarrowViewport(t *testing.T) {
	root := t.TempDir()
	m, err := newSetupBrowserModel(context.Background(), setupSession{output: io.Discard}, root, nil)
	if err != nil {
		t.Fatal(err)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	if m.current != root {
		t.Fatal("parent navigation escaped root")
	}
	m.Update(tea.WindowSizeMsg{Width: 10, Height: 8})
	if m.list.Width() > 10 {
		t.Fatal("list exceeds narrow viewport")
	}
}

func TestInteractiveBrowserHidesEscapeSymlink(t *testing.T) {
	root := t.TempDir()
	m, err := newSetupBrowserModel(context.Background(), setupSession{output: io.Discard}, root, nil)
	if err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "outside")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := m.refresh(); err != nil {
		t.Fatal(err)
	}
	for _, item := range m.list.Items() {
		if item.(setupBrowserItem).path == filepath.Join(root, "outside") {
			t.Fatal("escape link shown")
		}
	}
	m.current = filepath.Join(root, "outside")
	if err := m.refresh(); err == nil {
		t.Fatal("browser accepted path through escape link")
	}
}
