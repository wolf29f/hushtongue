package dictionaryexport

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/adrg/xdg"
	"github.com/wolf29f/hushtongue/internal/services"
	"github.com/wolf29f/hushtongue/internal/services/export"
	"github.com/wolf29f/hushtongue/internal/tui"
	"github.com/wolf29f/hushtongue/internal/tui/components/confirmmodal"
)

const maxInputWidth = 80

// Model is the page exporting the dictionary to a file, for the import into
// hushtongue v2.
type Model struct {
	services *services.Services

	// State
	status      string
	statusStyle lipgloss.Style

	// Components
	input textinput.Model

	// UI stuff
	width, height int
}

// NewModel creates the export page, its path defaulting to a dated file on
// the desktop.
func NewModel(services *services.Services) Model {
	input := textinput.New()
	input.SetValue(defaultPath())
	input.Focus()

	return Model{
		services: services,
		input:    input,
	}
}

// defaultPath returns a dated file on the desktop, or in the home directory
// when there is no desktop.
func defaultPath() string {
	name := "hushtongue-export-" + time.Now().Format("2006-01-02") + ".json"
	if info, err := os.Stat(xdg.UserDirs.Desktop); err == nil && info.IsDir() {
		return filepath.Join(xdg.UserDirs.Desktop, name)
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, name)
	}
	return name
}

func (m Model) Init() tea.Cmd {
	return tui.SetKeyMap(keys)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.SetWidth(min(maxInputWidth, max(0, m.width-lipgloss.Width(m.input.Prompt)-1)))
		return m, nil
	case overwriteConfirmedMsg:
		return m, m.write(msg.path)
	case exportedMsg:
		m.status = "Dictionnaire exporté dans :\n" + msg.path
		m.statusStyle = successStyle
		return m, nil
	case tea.KeyPressMsg:
		m.status = ""
		switch {
		case key.Matches(msg, keys.Quit):
			return m, tui.PopPage
		case key.Matches(msg, keys.Enter):
			return m.submit()
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// submit writes the file, after a confirmation when it already exists.
func (m Model) submit() (Model, tea.Cmd) {
	path, err := expandHome(strings.TrimSpace(m.input.Value()))
	if err != nil {
		slog.Error("unable to find home directory", "error", err)
		return m.fail("Impossible de trouver le dossier personnel.")
	}
	if path == "" {
		return m.fail("Saisissez le chemin du fichier.")
	}
	path, err = filepath.Abs(path)
	if err != nil {
		slog.Error("unable to resolve path", "path", path, "error", err)
		return m.fail("Chemin invalide.")
	}

	dir := filepath.Dir(path)
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return m.fail(fmt.Sprintf("Le dossier « %s » n'existe pas.", dir))
	}

	info, err := os.Stat(path)
	if err == nil && info.IsDir() {
		return m.fail(fmt.Sprintf("« %s » est un dossier.", path))
	}
	if err == nil {
		return m, tui.PushModal(confirmmodal.New(confirmmodal.Config{
			Title:        fmt.Sprintf("Remplacer « %s » ?", filepath.Base(path)),
			ConfirmLabel: "Remplacer",
			OnConfirm:    func() tea.Msg { return overwriteConfirmedMsg{path: path} },
		}))
	}
	return m, m.write(path)
}

func (m Model) fail(status string) (Model, tea.Cmd) {
	m.status = status
	m.statusStyle = errorStyle
	return m, nil
}

// write exports to a buffer first, so that a failed export never truncates
// an existing file.
func (m Model) write(path string) tea.Cmd {
	return func() tea.Msg {
		var data bytes.Buffer
		if err := export.Write(&data, m.services.Storage); err != nil {
			slog.Error("unable to export dictionary", "error", err)
			return tui.ShowError("Impossible d'exporter le dictionnaire.")()
		}
		if err := os.WriteFile(path, data.Bytes(), 0o644); err != nil {
			slog.Error("unable to write export file", "path", path, "error", err)
			return tui.ShowError(fmt.Sprintf("Impossible d'écrire le fichier « %s ».", path))()
		}
		return exportedMsg{path: path}
	}
}

// expandHome replaces a leading ~ with the home directory.
func expandHome(path string) (string, error) {
	if path != "~" && !strings.HasPrefix(path, "~/") && !strings.HasPrefix(path, `~\`) {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, path[1:]), nil
}

func (m Model) View() tea.View {
	parts := []string{
		titleStyle.Render("Exporter le dictionnaire"),
		hintStyle.Render("Fichier à importer dans hushtongue v2. Chemin du fichier :"),
		m.input.View(),
	}
	if m.status != "" {
		parts = append(parts, m.statusStyle.Render(m.status))
	}

	return tea.NewView(lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		lipgloss.JoinVertical(lipgloss.Left, parts...),
	))
}
