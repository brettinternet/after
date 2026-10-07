package browser

import "strings"

type keyAction uint8

const (
	keyQuit keyAction = iota
	keyHelp
	keyBack
	keyOverview
	keyChanges
	keyDiff
	keyNext
	keyPrevious
	keyNextFile
	keyPreviousFile
	keyNextHunk
	keyPreviousHunk
	keyDown
	keyUp
	keyPageDown
	keyPageUp
	keyStart
	keyEnd
	keyEnter
	keyPanLeft
	keyPanRight
	keyHex
	keyCapture
	keyImport
	keyUseCapture
	keyPin
	keyAcceptPin
	keyPreview
	keyApprove
	keyDeny
	keySession
	keyCancel
)

type keyContext string

const (
	contextOverview  keyContext = "Overview"
	contextChanges   keyContext = "Changes"
	contextDiff      keyContext = "Diff"
	contextInspector keyContext = "Inspector"
	contextPlan      keyContext = "Preview"
	contextHelp      keyContext = "Help"
)

var allKeyContexts = []keyContext{contextOverview, contextChanges, contextDiff, contextInspector, contextPlan, contextHelp}
var listKeyContexts = []keyContext{contextOverview, contextChanges}
var topKeyContexts = []keyContext{contextOverview, contextChanges, contextDiff}
var documentKeyContexts = []keyContext{contextInspector, contextPlan}
var browseKeyContexts = []keyContext{contextOverview, contextChanges, contextDiff, contextInspector}

type keyBinding struct {
	keys     []string
	hint     string
	label    string
	group    string
	action   keyAction
	contexts []keyContext
	priority int
	disabled func(*Model) string
}

// keyMap is the single source for keyboard dispatch, contextual hints, and help.
var keyMap = []keyBinding{
	{keys: []string{"q", "ctrl+c"}, hint: "q quit", label: "Quit and restore the terminal", group: "Session", action: keyQuit, contexts: allKeyContexts, priority: 0},
	{keys: []string{"?"}, hint: "? help", label: "Open grouped key help", group: "Session", action: keyHelp, contexts: allKeyContexts, priority: 1},
	{keys: []string{"esc"}, hint: "Esc back", label: "Return to the previous view", group: "Navigation", action: keyBack, contexts: allKeyContexts, priority: 2, disabled: canGoBack},
	{keys: []string{"1"}, hint: "1 Overview", label: "Switch to Overview", group: "Views", action: keyOverview, contexts: topKeyContexts, priority: 3},
	{keys: []string{"2"}, hint: "2 Changes", label: "Switch to Changes", group: "Views", action: keyChanges, contexts: topKeyContexts, priority: 3, disabled: needsData},
	{keys: []string{"3"}, hint: "3 Diff", label: "Switch to Diff", group: "Views", action: keyDiff, contexts: topKeyContexts, priority: 3, disabled: needsData},
	{keys: []string{"tab"}, hint: "Tab next view", label: "Switch to the next view or section", group: "Views", action: keyNext, contexts: append(append([]keyContext{}, topKeyContexts...), documentKeyContexts...), priority: 3, disabled: canMoveView},
	{keys: []string{"shift+tab"}, hint: "Shift+Tab previous", label: "Switch to the previous view or section", group: "Views", action: keyPrevious, contexts: append(append([]keyContext{}, topKeyContexts...), documentKeyContexts...), priority: 3, disabled: canMoveView},
	{keys: []string{"]"}, hint: "] next file", label: "Go to the next captured file", group: "Diff", action: keyNextFile, contexts: []keyContext{contextDiff}, priority: 4, disabled: canNavigateFiles},
	{keys: []string{"["}, hint: "[ previous file", label: "Go to the previous captured file", group: "Diff", action: keyPreviousFile, contexts: []keyContext{contextDiff}, priority: 4, disabled: canNavigateFiles},
	{keys: []string{"}"}, hint: "} next hunk", label: "Go to the next indexed hunk", group: "Diff", action: keyNextHunk, contexts: []keyContext{contextDiff}, priority: 4, disabled: canNavigateHunks},
	{keys: []string{"{"}, hint: "{ previous hunk", label: "Go to the previous indexed hunk", group: "Diff", action: keyPreviousHunk, contexts: []keyContext{contextDiff}, priority: 4, disabled: canNavigateHunks},
	{keys: []string{"j", "down"}, hint: "↓/j down", label: "Move down or scroll", group: "Navigation", action: keyDown, contexts: []keyContext{contextOverview, contextChanges, contextInspector, contextDiff, contextPlan, contextHelp}, priority: 4},
	{keys: []string{"k", "up"}, hint: "↑/k up", label: "Move up or scroll", group: "Navigation", action: keyUp, contexts: []keyContext{contextOverview, contextChanges, contextInspector, contextDiff, contextPlan, contextHelp}, priority: 4},
	{keys: []string{"pgdown"}, hint: "PgDn page", label: "Move down one page", group: "Navigation", action: keyPageDown, contexts: []keyContext{contextOverview, contextChanges, contextInspector, contextDiff, contextPlan, contextHelp}, priority: 5},
	{keys: []string{"pgup"}, hint: "PgUp page", label: "Move up one page", group: "Navigation", action: keyPageUp, contexts: []keyContext{contextOverview, contextChanges, contextInspector, contextDiff, contextPlan, contextHelp}, priority: 5},
	{keys: []string{"home", "g"}, hint: "Home/g start", label: "Move to the beginning", group: "Navigation", action: keyStart, contexts: []keyContext{contextOverview, contextChanges, contextInspector, contextDiff, contextPlan, contextHelp}, priority: 6},
	{keys: []string{"end", "G"}, hint: "End/G end", label: "Move to the end", group: "Navigation", action: keyEnd, contexts: []keyContext{contextOverview, contextChanges, contextInspector, contextDiff, contextPlan, contextHelp}, priority: 6},
	{keys: []string{"enter"}, hint: "Enter open", label: "Inspect the selected row", group: "Navigation", action: keyEnter, contexts: listKeyContexts, priority: 2, disabled: canOpen},
	{keys: []string{"h", "left"}, hint: "←/h pan", label: "Pan left", group: "Documents", action: keyPanLeft, contexts: []keyContext{contextOverview, contextChanges, contextInspector, contextDiff, contextPlan, contextHelp}, priority: 7, disabled: canPan},
	{keys: []string{"l", "right"}, hint: "→/l pan", label: "Pan right", group: "Documents", action: keyPanRight, contexts: []keyContext{contextOverview, contextChanges, contextInspector, contextDiff, contextPlan, contextHelp}, priority: 7, disabled: canPan},
	{keys: []string{"b"}, hint: "b bytes", label: "Toggle exact-byte hex view", group: "Documents", action: keyHex, contexts: []keyContext{contextDiff, contextInspector, contextPlan}, priority: 7, disabled: canHex},
	{keys: []string{"d"}, hint: "d Changes", label: "Open the complete change inventory", group: "Views", action: keyChanges, contexts: topKeyContexts, priority: 4, disabled: needsData},
	{keys: []string{"c"}, hint: "c capture", label: "Capture the working tree in the background", group: "Review", action: keyCapture, contexts: browseKeyContexts, priority: 8, disabled: canCapture},
	{keys: []string{"i"}, hint: "i import", label: "Import the configured report", group: "Review", action: keyImport, contexts: browseKeyContexts, priority: 9, disabled: canImport},
	{keys: []string{"u"}, hint: "u use capture", label: "Use the pending capture with the original base", group: "Review", action: keyUseCapture, contexts: browseKeyContexts, priority: 6, disabled: canUseCapture},
	{keys: []string{"p"}, hint: "p pin", label: "Pin the selected measured count", group: "Review", action: keyPin, contexts: []keyContext{contextOverview}, priority: 8, disabled: canPin},
	{keys: []string{"a"}, hint: "a accept pin", label: "Accept a pin, not a snapshot", group: "Review", action: keyAcceptPin, contexts: allKeyContexts, priority: 10, disabled: func(*Model) string { return "pin acceptance is not available in this review yet" }},
	{keys: []string{"r"}, hint: "r rerun", label: "Prepare an exact rerun preview; nothing runs", group: "Review", action: keyPreview, contexts: browseKeyContexts, priority: 8, disabled: canPreview},
	{keys: []string{"y"}, hint: "y approve once", label: "Run this exact preview once", group: "Consent", action: keyApprove, contexts: []keyContext{contextPlan}, priority: 4, disabled: canApprove},
	{keys: []string{"n"}, hint: "n deny", label: "Deny this preview without execution", group: "Consent", action: keyDeny, contexts: []keyContext{contextPlan}, priority: 5},
	{keys: []string{"s"}, hint: "s session IDs", label: "Inspect restart references", group: "Session", action: keySession, contexts: []keyContext{contextOverview, contextChanges, contextDiff, contextInspector, contextHelp}, priority: 10, disabled: needsData},
	{keys: []string{"x"}, hint: "x cancel", label: "Cancel an active owned job", group: "Jobs", action: keyCancel, contexts: allKeyContexts, priority: 11, disabled: canCancel},
}

func keyBindingFor(key string) (keyBinding, bool) {
	for _, binding := range keyMap {
		for _, candidate := range binding.keys {
			if candidate == key {
				return binding, true
			}
		}
	}
	return keyBinding{}, false
}

func contextForScreen(screen string) keyContext {
	switch screen {
	case "examples":
		return contextOverview
	case "inventory":
		return contextChanges
	case "patch":
		return contextDiff
	case "inspector":
		return contextInspector
	case "plan":
		return contextPlan
	default:
		return contextHelp
	}
}

func contextReason(contexts []keyContext) string {
	if len(contexts) == len(allKeyContexts) {
		return "available on any view"
	}
	names := make([]string, len(contexts))
	for i, context := range contexts {
		names[i] = string(context)
	}
	return "available in " + strings.Join(names, ", ")
}

func (m *Model) keyReason(binding keyBinding, forHelp bool) string {
	context := contextForScreen(m.screen)
	if forHelp && m.screen == "help" {
		context = contextForScreen(m.helpFrom)
	}
	if !containsContext(binding.contexts, context) {
		return contextReason(binding.contexts)
	}
	if binding.disabled != nil {
		return binding.disabled(m)
	}
	return ""
}

func containsContext(contexts []keyContext, want keyContext) bool {
	for _, context := range contexts {
		if context == want {
			return true
		}
	}
	return false
}

func canGoBack(m *Model) string {
	switch m.screen {
	case "help", "inspector", "plan", "inventory", "patch":
		return ""
	default:
		return "already on Overview"
	}
}
func needsData(m *Model) string {
	if m.data == nil {
		return "wait for stored records to load"
	}
	return ""
}
func keyScreen(m *Model) string {
	if m.screen == "help" && m.helpFrom != "" {
		return m.helpFrom
	}
	return m.screen
}

func canMoveView(m *Model) string {
	screen := keyScreen(m)
	if screen == "inspector" {
		entry := selectedEntry(m, screen)
		if entry == nil || len(entry.Sections) < 2 {
			return "this detail has no other section"
		}
		return ""
	}
	if screen == "plan" {
		return "this preview has no other section"
	}
	if screen == "examples" || screen == "inventory" || screen == "patch" {
		if m.data == nil {
			return "wait for stored records to load"
		}
		return ""
	}
	return "view navigation is unavailable here"
}
func selectedEntry(m *Model, screen string) *Entry {
	if m.data == nil {
		return nil
	}
	if screen == "inventory" || screen == "inspector" && m.returnTo == "inventory" {
		if m.inventory >= 0 && m.inventory < len(m.data.Inventory) {
			return &m.data.Inventory[m.inventory]
		}
		return nil
	}
	if m.index >= 0 && m.index < len(m.data.Entries) {
		return &m.data.Entries[m.index]
	}
	return nil
}
func canOpen(m *Model) string {
	if selectedEntry(m, keyScreen(m)) == nil {
		return "there is no selected row to inspect"
	}
	return ""
}
func canNavigateFiles(m *Model) string {
	if m.section != 0 {
		return "switch to the captured patch section first"
	}
	if m.hex {
		return "return to text view with b before navigating files"
	}
	if m.data == nil || m.data.Diff == nil {
		return "no shared captured patch is available"
	}
	if m.data.Diff.VisibleFiles < 2 {
		return "the captured patch has fewer than two indexed files"
	}
	return ""
}
func canNavigateHunks(m *Model) string {
	if m.section != 0 {
		return "switch to the captured patch section first"
	}
	if m.hex {
		return "return to text view with b before navigating hunks"
	}
	if m.data == nil || m.data.Diff == nil {
		return "no shared captured patch is available"
	}
	if len(m.data.Diff.HunkRows) < 2 {
		return "the captured patch has fewer than two indexed hunks"
	}
	return ""
}
func canPan(m *Model) string {
	screen := keyScreen(m)
	if (screen == "inspector" || screen == "plan" || screen == "patch") && m.doc == nil {
		return "wait for the document to load"
	}
	return ""
}
func canHex(m *Model) string {
	if m.doc == nil {
		return "no document is open"
	}
	return ""
}
func canCapture(m *Model) string {
	if m.data == nil {
		return "wait for stored records to load"
	}
	if m.capturing || m.busy {
		return "a capture or import is already running"
	}
	if m.jobs.Actions == nil && m.jobs.Capture == nil {
		return "capture is unavailable in this review"
	}
	return ""
}
func canImport(m *Model) string {
	if m.data == nil {
		return "wait for stored records to load"
	}
	if m.busy {
		return "another import is already running"
	}
	if m.jobs.Import == nil {
		return "no --import-file was configured"
	}
	return ""
}
func canUseCapture(m *Model) string {
	if m.pending == nil {
		return "no new capture is waiting"
	}
	if m.jobs.Actions == nil {
		return "snapshot selection is unavailable in this review"
	}
	if m.actionBusy || m.data == nil {
		return "wait for the current review action to finish"
	}
	return ""
}
func canPin(m *Model) string {
	if m.data == nil || len(m.data.Entries) == 0 {
		return "there is no selected observation"
	}
	if m.jobs.Actions == nil {
		return "pin actions are unavailable in this review"
	}
	if m.actionBusy {
		return "another review action is running"
	}
	i := min(max(m.index, 0), len(m.data.Entries)-1)
	if m.data.Entries[i].Receipt == "" || m.data.Entries[i].Expectation == "" {
		return "select a complete current observation"
	}
	if len(m.selected.Evidence) >= MaxEvidence {
		return "the evidence limit has been reached"
	}
	return ""
}
func canPreview(m *Model) string {
	if m.data == nil {
		return "wait for stored records to load"
	}
	if m.jobs.Actions == nil {
		return "rerun actions are unavailable in this review"
	}
	if m.actionBusy || m.running {
		return "another review action or run is active"
	}
	return ""
}
func canApprove(m *Model) string {
	if len(m.preview) == 0 || m.digest == "" || m.planPair != m.selected.Pair {
		return "there is no current exact preview to approve"
	}
	if m.running || m.actionBusy {
		return "another review action or run is active"
	}
	return ""
}
func canCancel(m *Model) string {
	if m.jobCancel == nil && m.runCancel == nil {
		return "no cancellable job is active"
	}
	return ""
}

func (m *Model) keyHints() string {
	if m.height < 7 {
		return "? help · q quit"
	}
	bindings := append([]keyBinding(nil), keyMap...)
	// The static table is already priority ordered; stable sorting makes that
	// property explicit if neighboring entries are later added.
	for i := 1; i < len(bindings); i++ {
		for j := i; j > 0 && bindings[j].priority < bindings[j-1].priority; j-- {
			bindings[j], bindings[j-1] = bindings[j-1], bindings[j]
		}
	}
	parts := []string{}
	used := 0
	limit := m.width
	for _, binding := range bindings {
		if m.keyReason(binding, false) != "" {
			continue
		}
		if m.width < 60 && binding.priority > 4 {
			continue
		}
		separator := " · "
		cost := len(binding.hint)
		if len(parts) > 0 {
			cost += len(separator)
		}
		if used+cost > limit {
			break
		}
		parts = append(parts, binding.hint)
		used += cost
	}
	return strings.Join(parts, " · ")
}

func (m *Model) helpLines() []string {
	lines := []string{}
	groups := []string{}
	byGroup := make(map[string][]keyBinding)
	for _, binding := range keyMap {
		if _, ok := byGroup[binding.group]; !ok {
			groups = append(groups, binding.group)
		}
		byGroup[binding.group] = append(byGroup[binding.group], binding)
	}
	for _, group := range groups {
		lines = append(lines, group)
		for _, binding := range byGroup[group] {
			reason := m.keyReason(binding, true)
			line := "  " + strings.Join(binding.keys, "/") + "  " + binding.label
			if reason != "" {
				line += "  [unavailable: " + reason + "]"
			}
			lines = append(lines, line)
		}
	}
	return lines
}
