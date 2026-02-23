package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Step tracks which stage of the wizard the user is on.
type Step int

const (
	StepSelectProvider    Step = iota // Choose LLM provider
	StepSelectSchema                   // Choose cloud provider / Steampipe schema
	StepSelectTable                    // Choose resource type (table)
	StepSelectResources                // Multi-select individual resources
	StepConfirmGenerate                // Review selected resources and confirm
	StepGenerating                     // Waiting for LLM response
	StepViewCode                       // Review generated Terraform code
	StepConfirmImport                  // Confirm running terraform import
	StepImporting                      // Running terraform import
	StepDone                           // Show results
)

// listItem wraps a string value to satisfy the list.Item interface.
type listItem struct {
	title string
	desc  string
}

func (i listItem) Title() string       { return i.title }
func (i listItem) Description() string { return i.desc }
func (i listItem) FilterValue() string { return i.title }

// Model is the top-level BubbleTea model for terraclaw.
type Model struct {
	// Current wizard step.
	step Step

	// List component used for menu selections.
	list list.Model

	// Width/height of the terminal.
	width  int
	height int

	// Available choices loaded from Steampipe.
	schemas []string
	tables  []string

	// User selections.
	selectedLLMProvider string
	selectedSchema      string
	selectedTable       string
	selectedResources   []ResourceItem

	// Resources fetched from Steampipe.
	resources []ResourceItem

	// Generated Terraform code.
	generatedCode string

	// Import results.
	importResults string

	// Error message (if any).
	err error

	// Loading spinner frame counter.
	spinnerFrame int

	// Scroll offset for code view.
	codeScrollOffset int

	// Channel to receive async results.
	resultCh chan asyncResult
}

// asyncResult carries the result of an async operation.
type asyncResult struct {
	code    string
	err     error
	imports string
}

// ResourceItem represents a selectable cloud resource in the TUI.
type ResourceItem struct {
	Resource interface{ String() string }
	Selected bool
	label    string
}

func (r ResourceItem) Title() string       { return r.label }
func (r ResourceItem) Description() string { return r.Resource.String() }
func (r ResourceItem) FilterValue() string { return r.label }

// New creates the initial BubbleTea model.
func New(schemas []string) Model {
	llmProviders := []list.Item{
		listItem{title: "ChatGPT (OpenAI)", desc: "GPT-4o powered code generation"},
		listItem{title: "Claude (Anthropic)", desc: "Claude 3.7 Sonnet powered code generation"},
		listItem{title: "Gemini (Google)", desc: "Gemini 2.0 Flash powered code generation"},
	}

	l := newList(llmProviders, "Select LLM Provider", 0)

	return Model{
		step:    StepSelectProvider,
		list:    l,
		schemas: schemas,
		width:   80,
		height:  24,
	}
}

// newList creates a bubbletea list with sensible defaults.
func newList(items []list.Item, title string, height int) list.Model {
	if height == 0 {
		height = 14
	}
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(lipgloss.Color("#AD58B4")).
		BorderForeground(lipgloss.Color("#AD58B4"))
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(lipgloss.Color("#AD58B4"))

	l := list.New(items, delegate, 76, height)
	l.Title = title
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = titleStyle
	return l
}

// spinnerChars cycles through for the loading animation.
var spinnerChars = []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetWidth(msg.Width - 4)
		m.list.SetHeight(msg.Height - 8)
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case spinnerTickMsg:
		m.spinnerFrame = (m.spinnerFrame + 1) % len(spinnerChars)
		return m, tickCmd()

	case asyncResultMsg:
		return m.handleAsyncResult(asyncResult(msg))
	}

	// Handle data-load messages (tables / resources from Steampipe).
	if updated, cmd, handled := m.updateForMessages(msg); handled {
		return updated, cmd
	}

	// Delegate to the list when we're on a list step.
	if m.isListStep() {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) isListStep() bool {
	switch m.step {
	case StepSelectProvider, StepSelectSchema, StepSelectTable, StepSelectResources, StepConfirmGenerate, StepConfirmImport:
		return true
	}
	return false
}

// handleKey processes keyboard input.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit

	case "enter":
		return m.handleEnter()

	case " ":
		// Space toggles selection on resource step.
		if m.step == StepSelectResources {
			return m.toggleResource()
		}

	case "up", "k":
		if m.step == StepViewCode {
			if m.codeScrollOffset > 0 {
				m.codeScrollOffset--
			}
			return m, nil
		}

	case "down", "j":
		if m.step == StepViewCode {
			m.codeScrollOffset++
			return m, nil
		}

	case "esc":
		return m.goBack()
	}

	// Delegate to list.
	if m.isListStep() {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) goBack() (tea.Model, tea.Cmd) {
	switch m.step {
	case StepSelectSchema:
		m.step = StepSelectProvider
		m.list = buildProviderList()
	case StepSelectTable:
		m.step = StepSelectSchema
		m.list = buildSchemaList(m.schemas)
	case StepSelectResources:
		m.step = StepSelectTable
		m.list = buildTableList(m.tables)
	case StepViewCode:
		m.step = StepConfirmGenerate
		m.list = buildConfirmList("Generate Terraform code for selected resources?")
	}
	return m, nil
}

func (m Model) handleEnter() (tea.Model, tea.Cmd) {
	switch m.step {
	case StepSelectProvider:
		if item, ok := m.list.SelectedItem().(listItem); ok {
			m.selectedLLMProvider = item.title
			m.step = StepSelectSchema
			m.list = buildSchemaList(m.schemas)
		}

	case StepSelectSchema:
		if item, ok := m.list.SelectedItem().(listItem); ok {
			m.selectedSchema = item.title
			// Tables are loaded via command; show loading step.
			return m, fetchTablesCmd(m.selectedSchema)
		}

	case StepSelectTable:
		if item, ok := m.list.SelectedItem().(listItem); ok {
			m.selectedTable = item.title
			return m, fetchResourcesCmd(m.selectedSchema, m.selectedTable)
		}

	case StepSelectResources:
		// Move to confirmation step.
		m.step = StepConfirmGenerate
		m.list = buildConfirmList("Generate Terraform code for selected resources?")

	case StepConfirmGenerate:
		if item, ok := m.list.SelectedItem().(listItem); ok {
			if item.title == "Yes" {
				m.step = StepGenerating
				return m, tea.Batch(tickCmd(), generateCodeCmd(m.selectedLLMProvider, m.selectedResources))
			}
			return m, tea.Quit
		}

	case StepViewCode:
		// Enter on code view → confirm import.
		m.step = StepConfirmImport
		m.list = buildConfirmList("Run terraform import for selected resources?")

	case StepConfirmImport:
		if item, ok := m.list.SelectedItem().(listItem); ok {
			if item.title == "Yes" {
				m.step = StepImporting
				return m, tea.Batch(tickCmd(), runImportCmd(m.selectedResources, m.generatedCode))
			}
			return m, tea.Quit
		}

	case StepDone:
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) toggleResource() (tea.Model, tea.Cmd) {
	idx := m.list.Index()
	if idx >= 0 && idx < len(m.resources) {
		m.resources[idx].Selected = !m.resources[idx].Selected
		m.selectedResources = nil
		for _, r := range m.resources {
			if r.Selected {
				m.selectedResources = append(m.selectedResources, r)
			}
		}
		// Rebuild list items to reflect selection state.
		items := make([]list.Item, len(m.resources))
		for i, r := range m.resources {
			prefix := "  "
			if r.Selected {
				prefix = "✓ "
			}
			items[i] = listItem{
				title: prefix + r.label,
				desc:  r.Resource.String(),
			}
		}
		m.list.SetItems(items)
	}
	return m, nil
}

func (m Model) handleAsyncResult(res asyncResult) (tea.Model, tea.Cmd) {
	m.err = res.err
	if res.code != "" {
		m.generatedCode = res.code
		m.step = StepViewCode
		m.codeScrollOffset = 0
	} else if res.imports != "" {
		m.importResults = res.imports
		m.step = StepDone
	}
	return m, nil
}

// View implements tea.Model.
func (m Model) View() string {
	switch m.step {
	case StepSelectProvider, StepSelectSchema, StepSelectTable, StepSelectResources,
		StepConfirmGenerate, StepConfirmImport:
		return m.listView()

	case StepGenerating:
		return m.loadingView("Generating Terraform code...")

	case StepImporting:
		return m.loadingView("Running terraform import...")

	case StepViewCode:
		return m.codeView()

	case StepDone:
		return m.doneView()
	}
	return ""
}

func (m Model) listView() string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(m.list.View())
	sb.WriteString("\n\n")
	if m.step == StepSelectResources {
		sb.WriteString(infoStyle.Render("  [space] toggle selection • [enter] confirm • [esc] back • [q] quit"))
	} else {
		sb.WriteString(infoStyle.Render("  [enter] select • [esc] back • [q] quit"))
	}
	if m.err != nil {
		sb.WriteString("\n" + errorStyle.Render("  Error: "+m.err.Error()))
	}
	return sb.String()
}

func (m Model) loadingView(msg string) string {
	spinner := spinnerChars[m.spinnerFrame%len(spinnerChars)]
	return fmt.Sprintf("\n\n  %s %s\n", spinner, infoStyle.Render(msg))
}

func (m Model) codeView() string {
	lines := strings.Split(m.generatedCode, "\n")
	visible := m.height - 8
	if visible < 5 {
		visible = 5
	}
	start := m.codeScrollOffset
	if start > len(lines)-visible {
		start = len(lines) - visible
	}
	if start < 0 {
		start = 0
	}
	end := start + visible
	if end > len(lines) {
		end = len(lines)
	}

	snippet := strings.Join(lines[start:end], "\n")
	return fmt.Sprintf(
		"\n%s\n\n%s\n\n%s",
		titleStyle.Render(" Generated Terraform Code "),
		codeStyle.Width(m.width-6).Render(snippet),
		infoStyle.Render("  [↑/↓] scroll • [enter] proceed to import • [esc] back • [q] quit"),
	)
}

func (m Model) doneView() string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(titleStyle.Render(" Import Results "))
	sb.WriteString("\n\n")
	if m.err != nil {
		sb.WriteString(errorStyle.Render("  Error: " + m.err.Error()))
	} else {
		sb.WriteString(successStyle.Render("  Terraform import complete!"))
		sb.WriteString("\n\n")
		sb.WriteString(m.importResults)
	}
	sb.WriteString("\n\n")
	sb.WriteString(infoStyle.Render("  [q] quit"))
	return sb.String()
}

// ---------------------------------------------------------------------------
// Helper builders
// ---------------------------------------------------------------------------

func buildProviderList() list.Model {
	items := []list.Item{
		listItem{title: "ChatGPT (OpenAI)", desc: "GPT-4o powered code generation"},
		listItem{title: "Claude (Anthropic)", desc: "Claude 3.7 Sonnet powered code generation"},
		listItem{title: "Gemini (Google)", desc: "Gemini 2.0 Flash powered code generation"},
	}
	return newList(items, "Select LLM Provider", 0)
}

func buildSchemaList(schemas []string) list.Model {
	items := make([]list.Item, len(schemas))
	for i, s := range schemas {
		items[i] = listItem{title: s, desc: "Steampipe plugin schema"}
	}
	return newList(items, "Select Cloud Provider", 0)
}

func buildTableList(tables []string) list.Model {
	items := make([]list.Item, len(tables))
	for i, t := range tables {
		items[i] = listItem{title: t, desc: "Resource type"}
	}
	return newList(items, "Select Resource Type", 0)
}

func buildConfirmList(question string) list.Model {
	items := []list.Item{
		listItem{title: "Yes", desc: "Proceed"},
		listItem{title: "No", desc: "Cancel"},
	}
	return newList(items, question, 4)
}
