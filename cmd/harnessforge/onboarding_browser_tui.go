package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"path/filepath"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type setupBrowserItem struct {
	path, title         string
	dir, parent, finish bool
}

func (i setupBrowserItem) Title() string       { return i.title }
func (i setupBrowserItem) Description() string { return "" }
func (i setupBrowserItem) FilterValue() string { return i.title }

type setupBrowserModel struct {
	session                           setupSession
	ctx                               context.Context
	root, current                     string
	files                             []setupDocument
	list                              list.Model
	review, done, cancelled, advanced bool
	status                            string
}

func newSetupBrowserModel(ctx context.Context, s setupSession, root string, files []setupDocument) (*setupBrowserModel, error) {
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false
	m := &setupBrowserModel{session: s, ctx: ctx, root: root, current: root, files: append([]setupDocument(nil), files...), list: list.New(nil, delegate, 76, 18)}
	m.list.DisableQuitKeybindings()
	m.list.SetFilteringEnabled(false)
	m.list.SetShowHelp(false)
	m.list.SetShowStatusBar(false)
	err := m.refresh()
	return m, err
}

func (m *setupBrowserModel) refresh() error {
	var items []list.Item
	if m.review {
		m.list.Title = m.session.uiText("Selected context")
		for _, file := range m.files {
			items = append(items, setupBrowserItem{path: file.Path, title: "[x] " + file.Source})
		}
	} else {
		rel, _ := filepath.Rel(m.root, m.current)
		m.list.Title = m.session.uiText("Context browser") + ": " + filepath.ToSlash(rel)
		if m.current != m.root {
			items = append(items, setupBrowserItem{path: filepath.Dir(m.current), title: "../", parent: true, dir: true})
		}
		entries, err := setupBrowserEntries(m.root, m.current)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			path := filepath.Join(m.current, entry.Name())
			mark := "[ ] "
			for _, file := range m.files {
				if file.Path == path || (entry.IsDir() && setupInside(path, file.Path)) {
					mark = "[x] "
					break
				}
			}
			title := mark + entry.Name()
			if entry.IsDir() {
				title += "/"
			}
			items = append(items, setupBrowserItem{path: path, title: title, dir: entry.IsDir()})
		}
	}
	items = append(items, setupBrowserItem{title: m.session.uiText("Continue"), finish: true})
	m.list.SetItems(items)
	return nil
}

func (m *setupBrowserModel) Init() tea.Cmd { return nil }

func (m *setupBrowserModel) toggle(item setupBrowserItem) {
	var kept []setupDocument
	removed := false
	for _, file := range m.files {
		if file.Path == item.path || (item.dir && setupInside(item.path, file.Path)) {
			removed = true
		} else {
			kept = append(kept, file)
		}
	}
	if removed {
		m.files = kept
		return
	}
	added, err := readSetupPath(m.ctx, m.root, item.path)
	if err != nil {
		m.status = err.Error()
		return
	}
	candidate := append(append([]setupDocument(nil), m.files...), added...)
	if len(candidate) > setupMaxDocuments {
		m.status = fmt.Sprintf(m.session.uiText("Context exceeds %d files"), setupMaxDocuments)
		return
	}
	if err := setupContextBytes(candidate, ""); err != nil {
		m.status = err.Error()
		return
	}
	m.files = candidate
}

func (m *setupBrowserModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(max(1, msg.Width-4), max(1, msg.Height-5))
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit
		case "p":
			m.advanced = true
			return m, tea.Quit
		case "tab":
			m.review = !m.review
			m.list.ResetSelected()
			if err := m.refresh(); err != nil {
				m.status = err.Error()
			}
			return m, nil
		case "backspace", "left":
			if m.review {
				m.review = false
			} else if m.current != m.root {
				m.current = filepath.Dir(m.current)
			}
			m.list.ResetSelected()
			if err := m.refresh(); err != nil {
				m.status = err.Error()
			}
			return m, nil
		case "enter", "space":
			item, ok := m.list.SelectedItem().(setupBrowserItem)
			if !ok {
				return m, nil
			}
			m.status = ""
			if item.finish {
				if msg.String() == "enter" {
					m.done = true
					return m, tea.Quit
				}
				return m, nil
			}
			if item.parent || (item.dir && msg.String() == "enter") {
				m.current = item.path
				m.list.ResetSelected()
			} else {
				m.toggle(item)
			}
			if err := m.refresh(); err != nil {
				m.status = err.Error()
			}
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *setupBrowserModel) View() tea.View {
	footer := fmt.Sprintf(m.session.uiText("Selected: %d document(s)"), len(m.files)) + "\n" + m.session.uiText("Arrows: navigate | Enter: open/continue | Space: select | Tab: review | p: paths | Esc: cancel") + "\n" + m.status
	return tea.NewView(m.list.View() + "\n" + lipgloss.NewStyle().Width(m.list.Width()).Render(footer))
}

func (s setupSession) interactiveDocuments(ctx context.Context, root string, existing []setupDocument) ([]setupDocument, error) {
	files := existing
	for {
		m, err := newSetupBrowserModel(ctx, s, root, files)
		if err != nil {
			return nil, err
		}
		_, err = tea.NewProgram(m, tea.WithInput(s.input), tea.WithOutput(s.uiOutput()), tea.WithContext(ctx)).Run()
		if err != nil {
			return nil, err
		}
		if m.cancelled {
			return nil, io.EOF
		}
		if m.done {
			return m.files, nil
		}
		if m.advanced {
			plain := s
			plain.interactive = false
			plain.reader = bufio.NewReader(s.input)
			files, err = plain.setupDocumentPaths(ctx, root, m.files)
			if err != nil {
				return nil, err
			}
		}
	}
}
