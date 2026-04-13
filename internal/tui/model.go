package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/arunim2405/terraclaw/internal/cache"
	"github.com/arunim2405/terraclaw/internal/debuglog"
	"github.com/arunim2405/terraclaw/internal/graph"
	"github.com/arunim2405/terraclaw/internal/llm"
	"github.com/arunim2405/terraclaw/internal/opencode"
	"github.com/arunim2405/terraclaw/internal/provider"
	"github.com/arunim2405/terraclaw/internal/steampipe"
)

// Step tracks which stage of the wizard the user is on.
type Step int

const (
	StepSelectSchema        Step = iota // Choose cloud provider / Steampipe schema
	StepSelectScanMode                  // Choose "Key Resources" or "All Resources"
	StepCheckingCache                   // Checking for cached scan results
	StepCacheChoice                     // Choose cached resources or fresh scan
	StepScanning                        // Scanning tables / building graph (progress)
	StepBrowseResourceTypes             // Browse discovered resource types
	StepSelectResources                 // Select individual resources (with related expansion)
	StepConfirmGenerate                 // Confirm before calling LLM
	StepGenerating                      // Waiting for LLM response
	StepViewCode                        // Review generated Terraform code
	StepConfirmImport                   // Confirm running terraform import
	StepImporting                       // Running terraform import
	StepDone                            // Show results
)

// listItem wraps a string value to satisfy the list.Item interface.
type listItem struct {
	title string
	desc  string
}

func (i listItem) Title() string       { return i.title }
func (i listItem) Description() string { return i.desc }
func (i listItem) FilterValue() string { return i.title }

// ResourceItem represents a selectable cloud resource in the TUI.
type ResourceItem struct {
	Resource interface{ String() string }
	Selected bool
	NodeKey  string // graph node key for relationship lookups
	label    string
}

func (r ResourceItem) Title() string       { return r.label }
func (r ResourceItem) Description() string { return r.Resource.String() }
func (r ResourceItem) FilterValue() string { return r.label }

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

	// User selections.

	selectedSchema   string
	selectedScanMode string // "key" or "all"
	cloud            provider.Cloud

	// Cache info for the current schema+scanMode (nil if no cache).
	cachedScan *cache.ScanInfo

	// Resource graph.
	resourceGraph *graph.Graph

	// Browsing state.
	resourceTypes     []string       // discovered resource types
	selectedType      string         // currently selected resource type
	resources         []ResourceItem // resources for current type
	selectedResources []ResourceItem // user-toggled resources across all types

	// Generated Terraform files.
	generatedFiles  []llm.GeneratedFile
	selectedFileIdx int // which file is currently being viewed

	// Import results.
	importResults string

	// CLI command equivalent of the current TUI selection.
	cliCommand string

	// Error message (if any).
	err error

	// Loading spinner frame counter.
	spinnerFrame int

	// Scroll offset for code view.
	codeScrollOffset int

	// Scan progress.
	scanProgress string

	// Agent status during code generation.
	agentStatus     string
	activeSessionID string
	activeResultCh  <-chan opencode.PromptResult
	messageTracker  *opencode.MessageTracker

	// Pipeline stage tracking (1 = blueprint, 2 = terraform, 3 = import).
	generationStage int

	// Stage 3 refinement iteration (1-based, up to MaxRefinementIterations).
	refinementIteration int

	// Channel to receive async results.
	resultCh chan asyncResult
}

// asyncResult carries the result of an async operation.
type asyncResult struct {
	files   []llm.GeneratedFile
	err     error
	imports string
}

// New creates the initial BubbleTea model.
// Code generation is handled by the OpenCode coding agent.
func New(schemas []string) Model {
	// Use sensible defaults; real dimensions arrive via tea.WindowSizeMsg.
	w, h := 80, 24
	l := buildSchemaList(schemas, w, h)

	return Model{
		step:    StepSelectSchema,
		list:    l,
		schemas: schemas,
		width:   w,
		height:  h,
	}
}

// newList creates a bubbletea list sized to the terminal.
// termWidth and termHeight are the current terminal dimensions.
// minHeight overrides the computed height when non-zero (used for short lists
// like confirm dialogs where we don't want to fill the whole screen).
func newList(items []list.Item, title string, termWidth, termHeight, minHeight int) list.Model {
	w := termWidth - 4
	if w < 40 {
		w = 40
	}
	// Reserve lines for title, help bar, padding.
	h := termHeight - 8
	if h < 8 {
		h = 8
	}
	// For short fixed lists (confirm, cache choice), cap at a small height
	// so the list doesn't look stretched.
	if minHeight > 0 && h > minHeight {
		h = minHeight
	}

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(lipgloss.Color("#AD58B4")).
		BorderForeground(lipgloss.Color("#AD58B4"))
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(lipgloss.Color("#AD58B4"))

	l := list.New(items, delegate, w, h)
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
		w := msg.Width - 4
		if w < 40 {
			w = 40
		}
		h := msg.Height - 8
		if h < 8 {
			h = 8
		}
		m.list.SetWidth(w)
		m.list.SetHeight(h)
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case spinnerTickMsg:
		m.spinnerFrame = (m.spinnerFrame + 1) % len(spinnerChars)
		return m, tickCmd()

	case asyncResultMsg:
		return m.handleAsyncResult(asyncResult(msg))

	case scanProgressMsg:
		m.scanProgress = msg.message
		return m, nil

	case cacheCheckMsg:
		return m.handleCacheCheck(msg)

	case cacheLoadedMsg:
		if msg.err != nil {
			// Cache load failed — fall back to live scan.
			debuglog.Log("[tui] cache load failed: %v, falling back to scan", msg.err)
			m.scanProgress = "Cache load failed, scanning..."
			return m, scanResourcesCmd(m.selectedSchema, m.selectedScanMode)
		}
		return m.handleGraphBuilt(graphBuiltMsg{graph: msg.graph})

	case graphBuiltMsg:
		return m.handleGraphBuilt(msg)

	case generatingStartedMsg:
		// OpenCode session is set up; start polling for progress.
		m.activeSessionID = msg.sessionID
		m.activeResultCh = msg.resultCh
		m.agentStatus = "Agent is starting..."
		m.generationStage = 1
		m.messageTracker = opencode.NewMessageTracker()
		debuglog.Log("[tui] generation started, polling session %s", msg.sessionID)
		return m, tea.Batch(tickCmd(), pollAgentStatusCmd(msg.sessionID, msg.resultCh, m.messageTracker))

	case agentStatusMsg:
		// Update agent status display and continue polling.
		m.agentStatus = msg.status
		if m.activeSessionID != "" && m.activeResultCh != nil {
			return m, pollAgentStatusCmd(m.activeSessionID, m.activeResultCh, m.messageTracker)
		}
		return m, nil

	case promptDoneMsg:
		if msg.err != nil {
			if m.generationStage == 3 {
				// Stage 3 error — show result and go to done.
				m.importResults = fmt.Sprintf("Import failed (iteration %d): %v",
					m.refinementIteration, msg.err)
				m.activeSessionID = ""
				m.activeResultCh = nil
				m.agentStatus = ""
				m.generationStage = 0
				m.step = StepDone
				return m, nil
			}
			prefix := "stage 1 (blueprint)"
			if m.generationStage == 2 {
				prefix = "stage 2 (terraform)"
			}
			m.err = fmt.Errorf("%s failed: %w", prefix, msg.err)
			m.activeSessionID = ""
			m.activeResultCh = nil
			m.agentStatus = ""
			m.generationStage = 0
			m.step = StepDone
			return m, nil
		}
		if m.generationStage == 1 {
			// Stage 1 done — transition to Stage 2.
			m.agentStatus = "Blueprint generated, starting Terraform generation..."
			debuglog.Log("[tui] stage 1 complete, transitioning to stage 2")
			return m, transitionToStage2Cmd(m.activeSessionID, msg.response, m.cloud)
		}
		if m.generationStage == 3 {
			// Stage 3 iteration done — check import result.
			importResult, parseErr := llm.ExtractImportResult(msg.response)

			if parseErr == nil && importResult.Status == "success" {
				result := fmt.Sprintf("All %d imports successful after %d iteration(s)!",
					importResult.Successful, m.refinementIteration)
				debuglog.Log("[tui] stage 3 complete: %s", result)
				return m, scanAndFinishImportCmd(result)
			}

			if m.refinementIteration < llm.MaxRefinementIterations {
				iterInfo := ""
				if importResult != nil {
					iterInfo = fmt.Sprintf(" (successful: %d, failed: %d)",
						importResult.Successful, importResult.Failed)
				}
				debuglog.Log("[tui] stage 3 iteration %d incomplete%s, starting iteration %d",
					m.refinementIteration, iterInfo, m.refinementIteration+1)
				m.agentStatus = fmt.Sprintf("Iteration %d had failures%s, starting next iteration...",
					m.refinementIteration, iterInfo)
				return m, importViaOpencodeCmd(m.activeSessionID, m.refinementIteration+1, m.cloud)
			}

			// Max iterations reached.
			var result string
			if importResult != nil {
				result = fmt.Sprintf("Reached max iterations (%d). Successful: %d, Failed: %d",
					llm.MaxRefinementIterations, importResult.Successful, importResult.Failed)
			} else {
				result = fmt.Sprintf("Reached max iterations (%d). Import results could not be parsed.",
					llm.MaxRefinementIterations)
			}
			debuglog.Log("[tui] stage 3 reached max iterations (%d)", llm.MaxRefinementIterations)
			return m, scanAndFinishImportCmd(result)
		}
		// Stage 2 done — scan files but keep session for Stage 3.
		m.activeResultCh = nil
		m.agentStatus = ""
		debuglog.Log("[tui] stage 2 complete, scanning files")
		return m, scanGeneratedFilesCmd()

	case stage2StartedMsg:
		m.generationStage = 2
		m.activeResultCh = msg.resultCh
		m.agentStatus = "Stage 2: Generating Terraform Code..."
		debuglog.Log("[tui] stage 2 started, polling session %s", m.activeSessionID)
		return m, pollAgentStatusCmd(m.activeSessionID, msg.resultCh, m.messageTracker)

	case stage3StartedMsg:
		m.generationStage = 3
		m.refinementIteration = msg.iteration
		m.activeResultCh = msg.resultCh
		m.agentStatus = fmt.Sprintf("Stage 3: Import & Validation (iteration %d/%d)...",
			msg.iteration, llm.MaxRefinementIterations)
		debuglog.Log("[tui] stage 3 started (iteration %d), polling session %s",
			msg.iteration, m.activeSessionID)
		return m, pollAgentStatusCmd(m.activeSessionID, msg.resultCh, m.messageTracker)

	case importFinishedMsg:
		m.activeSessionID = ""
		m.activeResultCh = nil
		m.agentStatus = ""
		m.generationStage = 0
		m.refinementIteration = 0
		if msg.err != nil {
			m.err = msg.err
			m.step = StepDone
			return m, nil
		}
		if len(msg.files) > 0 {
			m.generatedFiles = msg.files
		}
		m.importResults = msg.results
		m.step = StepDone
		debuglog.Log("[tui] import finished, transitioning to done")
		return m, nil

	case generationDoneMsg:
		// Keep activeSessionID alive — needed for Stage 3 import via OpenCode.
		m.activeResultCh = nil
		m.agentStatus = ""
		m.generationStage = 0
		if msg.err != nil {
			m.err = msg.err
			m.activeSessionID = "" // Clear on error — nothing to import.
			debuglog.Log("[tui] generation error: %v", msg.err)
			m.step = StepDone
			return m, nil
		}
		debuglog.Log("[tui] step: Generating → ViewCode (%d files)", len(msg.files))
		m.generatedFiles = msg.files
		m.selectedFileIdx = 0
		m.step = StepViewCode
		m.codeScrollOffset = 0
		return m, nil
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
	case StepSelectSchema, StepSelectScanMode, StepCacheChoice, StepBrowseResourceTypes,
		StepSelectResources, StepConfirmGenerate, StepConfirmImport:
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

	case "r":
		// 'r' on resource step: expand related resources for the highlighted item.
		if m.step == StepSelectResources {
			return m.expandRelated()
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

	case "tab":
		if m.step == StepViewCode && len(m.generatedFiles) > 1 {
			m.selectedFileIdx = (m.selectedFileIdx + 1) % len(m.generatedFiles)
			m.codeScrollOffset = 0
			return m, nil
		}

	case "shift+tab":
		if m.step == StepViewCode && len(m.generatedFiles) > 1 {
			m.selectedFileIdx--
			if m.selectedFileIdx < 0 {
				m.selectedFileIdx = len(m.generatedFiles) - 1
			}
			m.codeScrollOffset = 0
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
		return m, tea.Quit
	case StepSelectScanMode:
		m.step = StepSelectSchema
		m.list = buildSchemaList(m.schemas, m.width, m.height)
	case StepCacheChoice:
		m.step = StepSelectScanMode
		m.list = buildScanModeList(m.width, m.height)
	case StepBrowseResourceTypes:
		m.step = StepSelectScanMode
		m.list = buildScanModeList(m.width, m.height)
	case StepSelectResources:
		m.step = StepBrowseResourceTypes
		m.list = buildResourceTypeList(m.resourceTypes, m.resourceGraph, m.width, m.height)
	case StepConfirmGenerate:
		m.step = StepBrowseResourceTypes
		m.list = buildResourceTypeList(m.resourceTypes, m.resourceGraph, m.width, m.height)
	case StepViewCode:
		m.step = StepConfirmGenerate
		m.list = buildConfirmList("Generate Terraform code for selected resources?", m.width, m.height)
	}
	return m, nil
}

func (m Model) handleEnter() (tea.Model, tea.Cmd) {
	switch m.step {
	case StepSelectSchema:
		if item, ok := m.list.SelectedItem().(listItem); ok {
			debuglog.Log("[tui] step: SelectSchema → SelectScanMode (schema=%q)", item.title)
			m.selectedSchema = item.title
			m.cloud = provider.DetectFromSchema(item.title)
			m.step = StepSelectScanMode
			m.list = buildScanModeList(m.width, m.height, m.cloud)
		}

	case StepSelectScanMode:
		if item, ok := m.list.SelectedItem().(listItem); ok {
			switch item.title {
			case "Key Resources (Recommended)":
				m.selectedScanMode = "key"
			case "All Resources":
				m.selectedScanMode = "all"
			}
			// Check cache before scanning.
			debuglog.Log("[tui] step: SelectScanMode → CheckingCache (mode=%q)", m.selectedScanMode)
			m.step = StepCheckingCache
			return m, tea.Batch(tickCmd(), checkCacheCmd(m.selectedSchema, m.selectedScanMode))
		}

	case StepCacheChoice:
		if item, ok := m.list.SelectedItem().(listItem); ok {
			switch item.title {
			case "Use Cached Resources":
				debuglog.Log("[tui] step: CacheChoice → loading from cache (scanID=%d)", m.cachedScan.ID)
				m.step = StepScanning
				m.scanProgress = "Loading resources from cache..."
				return m, tea.Batch(tickCmd(), loadCacheCmd(m.cachedScan.ID))
			case "Rescan from Cloud":
				debuglog.Log("[tui] step: CacheChoice → Scanning (fresh)")
				m.cachedScan = nil
				m.step = StepScanning
				m.scanProgress = "Starting scan..."
				return m, tea.Batch(tickCmd(), scanResourcesCmd(m.selectedSchema, m.selectedScanMode))
			}
		}

	case StepBrowseResourceTypes:
		if item, ok := m.list.SelectedItem().(listItem); ok {
			resourceType := item.title
			debuglog.Log("[tui] step: BrowseResourceTypes → SelectResources (type=%q)", resourceType)
			m.selectedType = resourceType
			m.resources = buildResourceItems(m.resourceGraph, resourceType)
			m.step = StepSelectResources
			m.list = buildResourceList(m.resources, resourceType, m.width, m.height)
		}

	case StepSelectResources:
		// If user didn't explicitly toggle any resources, auto-select all visible.
		if len(m.selectedResources) == 0 && len(m.resources) > 0 {
			for i := range m.resources {
				m.resources[i].Selected = true
			}
			m.selectedResources = make([]ResourceItem, len(m.resources))
			copy(m.selectedResources, m.resources)
		}
		debuglog.Log("[tui] step: SelectResources → ConfirmGenerate (%d resource(s) selected)", len(m.selectedResources))
		m.step = StepConfirmGenerate
		m.cliCommand = buildCLICommand(m.selectedResources, m.selectedSchema)
		m.list = buildConfirmList(fmt.Sprintf("Generate Terraform code for %d selected resource(s)?", len(m.selectedResources)), m.width, m.height)

	case StepConfirmGenerate:
		if item, ok := m.list.SelectedItem().(listItem); ok {
			if item.title == "Yes" {
				debuglog.Log("[tui] step: ConfirmGenerate → Generating")
				m.step = StepGenerating
				return m, tea.Batch(tickCmd(), generateCodeCmd(m.selectedResources, m.selectedSchema))
			}
			debuglog.Log("[tui] step: ConfirmGenerate → Quit (user declined)")
			return m, tea.Quit
		}

	case StepViewCode:
		debuglog.Log("[tui] step: ViewCode → ConfirmImport")
		m.step = StepConfirmImport
		m.list = buildConfirmList("Run terraform import for selected resources?", m.width, m.height)

	case StepConfirmImport:
		if item, ok := m.list.SelectedItem().(listItem); ok {
			if item.title == "Yes" {
				debuglog.Log("[tui] step: ConfirmImport → Importing")
				m.step = StepImporting
				// Use OpenCode for import+refinement if session is still active.
				if m.activeSessionID != "" && opencodeServer != nil {
					debuglog.Log("[tui] using OpenCode session %s for Stage 3 import", m.activeSessionID)
					return m, tea.Batch(tickCmd(), importViaOpencodeCmd(m.activeSessionID, 1, m.cloud))
				}
				// Fallback to direct import.sh execution.
				debuglog.Log("[tui] no active session, falling back to direct import")
				return m, tea.Batch(tickCmd(), runImportCmd(m.selectedResources, ""))
			}
			debuglog.Log("[tui] step: ConfirmImport → Quit (user declined)")
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

		// Rebuild selectedResources from all selected items.
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

// expandRelated finds all resources related to the currently highlighted
// resource (via the graph) and adds them to the current resource list.
func (m Model) expandRelated() (tea.Model, tea.Cmd) {
	idx := m.list.Index()
	if idx < 0 || idx >= len(m.resources) || m.resourceGraph == nil {
		return m, nil
	}

	nodeKey := m.resources[idx].NodeKey
	if nodeKey == "" {
		return m, nil
	}

	related := m.resourceGraph.RelatedTo(nodeKey)
	debuglog.Log("[tui] expanding related for %s: %d node(s) found", nodeKey, len(related))

	// Build a set of existing resource keys.
	existing := make(map[string]bool)
	for _, r := range m.resources {
		existing[r.NodeKey] = true
	}

	added := 0
	for _, node := range related {
		if existing[node.Key] {
			continue
		}
		label := node.Resource.Name
		if label == "" {
			label = node.Resource.ID
		}
		m.resources = append(m.resources, ResourceItem{
			Resource: node.Resource,
			Selected: true, // auto-select related resources
			NodeKey:  node.Key,
			label:    fmt.Sprintf("[%s] %s", node.Resource.Type, label),
		})
		existing[node.Key] = true
		added++
	}

	if added > 0 {
		// Also add them to selectedResources.
		m.selectedResources = nil
		for _, r := range m.resources {
			if r.Selected {
				m.selectedResources = append(m.selectedResources, r)
			}
		}

		// Rebuild the list.
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
		debuglog.Log("[tui] added %d related resource(s), total=%d", added, len(m.resources))
	}

	return m, nil
}

func (m Model) handleCacheCheck(msg cacheCheckMsg) (tea.Model, tea.Cmd) {
	if msg.scanInfo == nil {
		// No valid cache — go straight to scanning.
		debuglog.Log("[tui] no cache hit, proceeding to scan")
		m.step = StepScanning
		m.scanProgress = "Starting scan..."
		return m, tea.Batch(tickCmd(), scanResourcesCmd(m.selectedSchema, m.selectedScanMode))
	}

	// Valid cache found — show choice to user.
	m.cachedScan = msg.scanInfo
	m.step = StepCacheChoice
	m.list = buildCacheChoiceList(msg.scanInfo, m.width, m.height)
	debuglog.Log("[tui] cache hit: scan_id=%d, resources=%d, age=%s",
		msg.scanInfo.ID, msg.scanInfo.Stats.ResourceCount,
		time.Since(msg.scanInfo.FinishedAt).Round(time.Second))
	return m, nil
}

func (m Model) handleGraphBuilt(msg graphBuiltMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.err = msg.err
		debuglog.Log("[tui] graph build error: %v", msg.err)
		return m, nil
	}

	m.resourceGraph = msg.graph
	m.resourceTypes = msg.graph.ResourceTypes()
	debuglog.Log("[tui] graph built: %d type(s), %d node(s), %d edge(s)",
		len(m.resourceTypes), msg.graph.Stats.ResourceCount, msg.graph.Stats.EdgeCount)

	m.step = StepBrowseResourceTypes
	m.list = buildResourceTypeList(m.resourceTypes, m.resourceGraph, m.width, m.height)
	return m, nil
}

func (m Model) handleAsyncResult(res asyncResult) (tea.Model, tea.Cmd) {
	m.err = res.err
	if res.err != nil {
		debuglog.Log("[tui] async error: %v", res.err)
	}
	if len(res.files) > 0 {
		debuglog.Log("[tui] step: Generating → ViewCode (%d files)", len(res.files))
		m.generatedFiles = res.files
		m.selectedFileIdx = 0
		m.step = StepViewCode
		m.codeScrollOffset = 0
	} else if res.imports != "" {
		debuglog.Log("[tui] step: Importing → Done")
		m.importResults = res.imports
		m.step = StepDone
	}
	return m, nil
}

// View implements tea.Model.
func (m Model) View() string {
	switch m.step {
	case StepSelectSchema, StepSelectScanMode, StepCacheChoice, StepBrowseResourceTypes,
		StepSelectResources, StepConfirmGenerate, StepConfirmImport:
		return m.listView()

	case StepCheckingCache:
		return m.loadingView("Checking for cached resources...")

	case StepScanning:
		return m.scanView()

	case StepGenerating:
		return m.generatingView()

	case StepImporting:
		if m.generationStage == 3 {
			return m.importingView()
		}
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
	switch m.step {
	case StepSelectResources:
		sb.WriteString(infoStyle.Render("  [space] toggle • [r] expand related • [enter] confirm • [esc] back • [q] quit"))
	case StepConfirmGenerate:
		sb.WriteString(infoStyle.Render("  [enter] select • [esc] back • [q] quit"))
		if m.cliCommand != "" {
			sb.WriteString("\n\n")
			sb.WriteString(subtleStyle.Render("  💡 Equivalent CLI command:"))
			sb.WriteString("\n")
			sb.WriteString(fmt.Sprintf("  %s", m.cliCommand))
		}
	default:
		sb.WriteString(infoStyle.Render("  [enter] select • [esc] back • [q] quit"))
	}
	if m.err != nil {
		sb.WriteString("\n" + errorStyle.Render("  Error: "+m.err.Error()))
	}
	return sb.String()
}

func (m Model) scanView() string {
	spinner := spinnerChars[m.spinnerFrame%len(spinnerChars)]
	return fmt.Sprintf("\n\n  %s %s\n\n  %s\n",
		spinner,
		infoStyle.Render("Scanning cloud resources..."),
		infoStyle.Render(m.scanProgress),
	)
}

func (m Model) loadingView(msg string) string {
	spinner := spinnerChars[m.spinnerFrame%len(spinnerChars)]
	return fmt.Sprintf("\n\n  %s %s\n", spinner, infoStyle.Render(msg))
}

func (m Model) generatingView() string {
	var sb strings.Builder
	spinner := spinnerChars[m.spinnerFrame%len(spinnerChars)]

	sb.WriteString("\n")
	sb.WriteString(titleStyle.Render(" Generating Terraform Code "))
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("  %s %s\n", spinner, infoStyle.Render(func() string {
		switch m.generationStage {
		case 1:
			return "Stage 1: Generating Blueprint..."
		case 2:
			return "Stage 2: Generating Terraform Code..."
		default:
			return "OpenCode is generating Terraform files..."
		}
	}())))
	sb.WriteString("\n")

	if m.agentStatus != "" {
		// Word wrap the status to fit the terminal width.
		maxWidth := m.width - 6
		if maxWidth < 40 {
			maxWidth = 80
		}
		status := m.agentStatus
		if len(status) > maxWidth {
			status = status[:maxWidth-3] + "..."
		}
		sb.WriteString(fmt.Sprintf("  %s\n", subtleStyle.Render(status)))
	}

	if m.err != nil {
		sb.WriteString("\n")
		sb.WriteString(errorStyle.Render("  Error: " + m.err.Error()))
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	sb.WriteString(subtleStyle.Render("  Press q to quit"))
	sb.WriteString("\n")
	return sb.String()
}

func (m Model) importingView() string {
	var sb strings.Builder
	spinner := spinnerChars[m.spinnerFrame%len(spinnerChars)]

	sb.WriteString("\n")
	sb.WriteString(titleStyle.Render(" Importing Terraform Resources "))
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("  %s %s\n", spinner, infoStyle.Render(
		fmt.Sprintf("Stage 3: Import & Validation (iteration %d/%d)...",
			m.refinementIteration, llm.MaxRefinementIterations))))
	sb.WriteString("\n")

	if m.agentStatus != "" {
		maxWidth := m.width - 6
		if maxWidth < 40 {
			maxWidth = 80
		}
		status := m.agentStatus
		if len(status) > maxWidth {
			status = status[:maxWidth-3] + "..."
		}
		sb.WriteString(fmt.Sprintf("  %s\n", subtleStyle.Render(status)))
	}

	sb.WriteString("\n")
	sb.WriteString(subtleStyle.Render("  Press q to quit"))
	sb.WriteString("\n")
	return sb.String()
}

func (m Model) codeView() string {
	if len(m.generatedFiles) == 0 {
		return fmt.Sprintf("\n%s\n\n%s\n",
			titleStyle.Render(" Generated Terraform Files "),
			errorStyle.Render("  No .tf files were generated."),
		)
	}

	// File tabs header.
	var tabs strings.Builder
	for i, f := range m.generatedFiles {
		if i == m.selectedFileIdx {
			tabs.WriteString(fmt.Sprintf(" [%s] ", f.Name))
		} else {
			tabs.WriteString(fmt.Sprintf("  %s  ", f.Name))
		}
	}

	// Show content of selected file.
	currentFile := m.generatedFiles[m.selectedFileIdx]
	lines := strings.Split(currentFile.Content, "\n")
	visible := m.height - 10
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

	fileInfo := fmt.Sprintf("%d file(s) generated • viewing: %s (%d/%d)",
		len(m.generatedFiles), currentFile.Name, m.selectedFileIdx+1, len(m.generatedFiles))

	return fmt.Sprintf(
		"\n%s\n  %s\n\n%s\n\n%s",
		titleStyle.Render(" Generated Terraform Files "),
		infoStyle.Render(tabs.String()),
		codeStyle.Width(m.width-6).Render(snippet),
		infoStyle.Render(fmt.Sprintf("  %s\n  [tab/shift+tab] switch file • [↑/↓] scroll • [enter] proceed to import • [esc] back • [q] quit", fileInfo)),
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
	if m.cliCommand != "" {
		sb.WriteString("\n\n")
		sb.WriteString(subtleStyle.Render("  💡 To re-run this generation:"))
		sb.WriteString("\n")
		sb.WriteString(fmt.Sprintf("  %s", m.cliCommand))
	}
	sb.WriteString("\n\n")
	sb.WriteString(infoStyle.Render("  [q] quit"))
	return sb.String()
}

// ---------------------------------------------------------------------------
// Helper builders
// ---------------------------------------------------------------------------

func buildSchemaList(schemas []string, w, h int) list.Model {
	items := make([]list.Item, len(schemas))
	for i, s := range schemas {
		items[i] = listItem{title: s, desc: "Steampipe plugin schema"}
	}
	return newList(items, "Select Cloud Provider", w, h, 0)
}

func buildScanModeList(w, h int, cloud ...provider.Cloud) list.Model {
	c := provider.AWS
	if len(cloud) > 0 {
		c = cloud[0]
	}
	defaultTables := graph.DefaultTablesForProvider(c)

	var desc string
	switch c {
	case provider.Azure:
		desc = fmt.Sprintf("Scan %d key Azure resource types (VNets, VMs, AKS, Storage, SQL, etc.)", len(defaultTables))
	default:
		desc = fmt.Sprintf("Scan %d key AWS resource types (VPCs, EC2, IAM, S3, RDS, etc.)", len(defaultTables))
	}

	items := []list.Item{
		listItem{
			title: "Key Resources (Recommended)",
			desc:  desc,
		},
		listItem{
			title: "All Resources",
			desc:  "Scan ALL resource types (may take several minutes)",
		},
	}
	return newList(items, "Select Scan Mode", w, h, 8)
}

func buildResourceTypeList(types []string, g *graph.Graph, w, h int) list.Model {
	items := make([]list.Item, len(types))
	for i, t := range types {
		nodes := g.NodesByType(t)
		items[i] = listItem{
			title: t,
			desc:  fmt.Sprintf("%d resource(s) discovered", len(nodes)),
		}
	}
	return newList(items, "Select Resource Type", w, h, 0)
}

func buildResourceList(resources []ResourceItem, resourceType string, w, h int) list.Model {
	items := make([]list.Item, len(resources))
	for i, r := range resources {
		prefix := "  "
		if r.Selected {
			prefix = "✓ "
		}
		items[i] = listItem{
			title: prefix + r.label,
			desc:  r.Resource.String(),
		}
	}
	return newList(items, fmt.Sprintf("Select Resources (%s)", resourceType), w, h, 0)
}

func buildResourceItems(g *graph.Graph, resourceType string) []ResourceItem {
	nodes := g.NodesByType(resourceType)
	items := make([]ResourceItem, len(nodes))
	for i, n := range nodes {
		label := n.Resource.Name
		if label == "" {
			label = n.Resource.ID
		}
		relatedCount := len(n.Edges)
		if relatedCount > 0 {
			label = fmt.Sprintf("%s (↔ %d related)", label, relatedCount)
		}
		items[i] = ResourceItem{
			Resource: n.Resource,
			NodeKey:  n.Key,
			label:    label,
		}
	}
	return items
}

func buildCacheChoiceList(info *cache.ScanInfo, w, h int) list.Model {
	age := time.Since(info.FinishedAt).Round(time.Second)
	items := []list.Item{
		listItem{
			title: "Use Cached Resources",
			desc:  fmt.Sprintf("Cached %s ago — %d resources, %d edges", formatDuration(age), info.Stats.ResourceCount, info.Stats.EdgeCount),
		},
		listItem{
			title: "Rescan from Cloud",
			desc:  "Discard cache and scan live resources from Steampipe",
		},
	}
	return newList(items, "Cached scan found", w, h, 8)
}

// formatDuration returns a human-friendly duration string.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d seconds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%d minutes", int(d.Minutes()))
	}
	hours := int(d.Hours())
	if hours < 24 {
		return fmt.Sprintf("%d hours", hours)
	}
	return fmt.Sprintf("%d days", hours/24)
}

func buildConfirmList(question string, w, h int) list.Model {
	items := []list.Item{
		listItem{title: "Yes", desc: "Proceed"},
		listItem{title: "No", desc: "Cancel"},
	}
	return newList(items, question, w, h, 6)
}

// buildCLICommand constructs the equivalent `terraclaw generate --resources` command
// from the selected resources, so users can re-run the generation without the TUI.
func buildCLICommand(resources []ResourceItem, schema string) string {
	var arns []string
	for _, ri := range resources {
		if r, ok := ri.Resource.(steampipe.Resource); ok {
			// Prefer the ARN from properties, fall back to ID.
			arn := ""
			if v, exists := r.Properties["arn"]; exists && v != "" {
				arn = v
			} else if r.ID != "" {
				arn = r.ID
			}
			if arn != "" {
				arns = append(arns, arn)
			}
		}
	}
	if len(arns) == 0 {
		return ""
	}
	if schema == "" {
		schema = "aws"
	}
	return fmt.Sprintf("terraclaw generate --resources %s --schema %s",
		strings.Join(arns, ","), schema)
}
