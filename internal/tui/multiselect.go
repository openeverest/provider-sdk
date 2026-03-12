// Copyright (C) 2026 The OpenEverest Contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package tui provides reusable bubbletea-based TUI components for provider-sdk commands.
package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	checkboxChecked   = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render("◉")
	checkboxUnchecked = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("◯")
	cursorStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	selectedItemStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
	normalItemStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	hintStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true)
	titleStyle        = lipgloss.NewStyle().Bold(true)
)

// multiSelectModel is a bubbletea model for a checkbox multi-select list.
type multiSelectModel struct {
	title    string
	items    []string
	checked  []bool
	cursor   int
	done     bool
	canceled bool
}

func newMultiSelectModel(title string, items []string, allChecked bool) multiSelectModel {
	checked := make([]bool, len(items))
	if allChecked {
		for i := range checked {
			checked[i] = true
		}
	}
	return multiSelectModel{
		title:   title,
		items:   items,
		checked: checked,
	}
}

func (m multiSelectModel) Init() tea.Cmd { return nil }

func (m multiSelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.canceled = true
			return m, tea.Quit
		case "enter":
			m.done = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case " ":
			m.checked[m.cursor] = !m.checked[m.cursor]
		case "a":
			// Toggle all: if any are unchecked, check all; otherwise uncheck all.
			anyUnchecked := false
			for _, c := range m.checked {
				if !c {
					anyUnchecked = true
					break
				}
			}
			for i := range m.checked {
				m.checked[i] = anyUnchecked
			}
		}
	}
	return m, nil
}

func (m multiSelectModel) View() string {
	if m.done || m.canceled {
		return ""
	}
	var sb strings.Builder

	sb.WriteString(titleStyle.Render(m.title))
	sb.WriteString("\n\n")

	for i, item := range m.items {
		checkbox := checkboxUnchecked
		if m.checked[i] {
			checkbox = checkboxChecked
		}

		cursor := "  "
		name := normalItemStyle.Render(item)
		if i == m.cursor {
			cursor = cursorStyle.Render("> ")
			name = selectedItemStyle.Render(item)
		}

		sb.WriteString(fmt.Sprintf("%s%s %s\n", cursor, checkbox, name))
	}

	sb.WriteString("\n")
	sb.WriteString(hintStyle.Render("up/down navigate  space toggle  a toggle all  enter confirm  q cancel"))
	sb.WriteString("\n")

	return sb.String()
}

// RunMultiSelect displays an interactive checkbox list and returns the selected items.
// The list starts with all items checked when startAllChecked is true.
// Returns an error if the user cancels (Ctrl+C or q).
func RunMultiSelect(title string, items []string, startAllChecked bool) ([]string, error) {
	if len(items) == 0 {
		return nil, nil
	}

	m := newMultiSelectModel(title, items, startAllChecked)
	p := tea.NewProgram(m)

	result, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("running selector: %w", err)
	}

	final, ok := result.(multiSelectModel)
	if !ok {
		return nil, fmt.Errorf("unexpected model type")
	}
	if final.canceled {
		return nil, fmt.Errorf("canceled")
	}

	selected := make([]string, 0, len(items))
	for i, name := range items {
		if final.checked[i] {
			selected = append(selected, name)
		}
	}
	return selected, nil
}
