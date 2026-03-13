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

package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	labelStyle       = lipgloss.NewStyle().Bold(true)
	defaultStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	errorStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	inputPromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
)

// promptModel is a bubbletea model for a single-line text input prompt.
type promptModel struct {
	label        string
	defaultValue string
	required     bool
	input        textinput.Model
	err          string
	done         bool
	canceled     bool
}

func newPromptModel(label, placeholder, defaultValue string, required bool) promptModel {
	ti := textinput.New()
	ti.Prompt = inputPromptStyle.Render("› ")
	ti.Placeholder = placeholder
	ti.Focus()
	ti.CharLimit = 256

	return promptModel{
		label:        label,
		defaultValue: defaultValue,
		required:     required,
		input:        ti,
	}
}

func (m promptModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m promptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.canceled = true
			return m, tea.Quit
		case "enter":
			val := strings.TrimSpace(m.input.Value())
			if val == "" && m.required && m.defaultValue == "" {
				m.err = "This field is required."
				return m, nil
			}
			m.done = true
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.err = "" // clear error on any typing
	return m, cmd
}

func (m promptModel) View() string {
	if m.done || m.canceled {
		return ""
	}
	var sb strings.Builder

	// Label line.
	label := labelStyle.Render(m.label)
	if m.defaultValue != "" {
		label += " " + defaultStyle.Render(fmt.Sprintf("[%s]", m.defaultValue))
	}
	sb.WriteString(label + "\n")

	// Input field.
	sb.WriteString(m.input.View() + "\n")

	// Error message.
	if m.err != "" {
		sb.WriteString(errorStyle.Render("  ✗ "+m.err) + "\n")
	}

	return sb.String()
}

// RunPrompt displays an interactive single-line text input.
// If the user presses Enter with an empty input and a defaultValue is set, the default is returned.
// Returns an error if the user cancels (Ctrl+C).
func RunPrompt(label, placeholder, defaultValue string, required bool) (string, error) {
	m := newPromptModel(label, placeholder, defaultValue, required)
	p := tea.NewProgram(m)

	result, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("running prompt: %w", err)
	}

	final, ok := result.(promptModel)
	if !ok {
		return "", fmt.Errorf("unexpected model type")
	}
	if final.canceled {
		return "", fmt.Errorf("canceled")
	}

	val := strings.TrimSpace(final.input.Value())
	if val == "" && defaultValue != "" {
		return defaultValue, nil
	}
	return val, nil
}
