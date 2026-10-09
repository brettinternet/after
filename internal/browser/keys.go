package browser

import (
	"strings"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/terminal"
	"github.com/rivo/uniseg"
)

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
	keyActivity
	keySession
	keyCancel
	keySearch
	keyNextMatch
	keyPreviousMatch
)

type keyContext string

const (
	contextOverview  keyContext = "Overview"
	contextChanges   keyContext = "Changes"
	contextDiff      keyContext = "Diff"
	contextActivity  keyContext = "Activity"
	contextInspector keyContext = "Inspector"
	contextPlan      keyContext = "Preview"
	contextHelp      keyContext = "Help"
)

var allKeyContexts = []keyContext{contextOverview, contextChanges, contextDiff, contextActivity, contextInspector, contextPlan, contextHelp}
var listKeyContexts = []keyContext{contextOverview, contextChanges, contextActivity}
var topKeyContexts = []keyContext{contextOverview, contextChanges, contextDiff, contextActivity}
var documentKeyContexts = []keyContext{contextInspector, contextPlan}
var browseKeyContexts = []keyContext{contextOverview, contextChanges, contextDiff, contextInspector}
var searchKeyContexts = []keyContext{contextOverview, contextChanges, contextDiff, contextActivity, contextInspector}

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
	{keys: []string{"q", "ctrl+c"}, hint: "q quit", label: "Quit (q confirms during a run; Ctrl-C quits now)", group: "Session", action: keyQuit, contexts: allKeyContexts, priority: 0},
	{keys: []string{"?"}, hint: "? help", label: "Open grouped key help", group: "Session", action: keyHelp, contexts: allKeyContexts, priority: 1},
	{keys: []string{"esc", "ctrl+g"}, hint: "Esc back", label: "Return to the previous view", group: "Navigation", action: keyBack, contexts: allKeyContexts, priority: 2, disabled: canGoBack},
	{keys: []string{"/"}, hint: "/ search", label: "Search displayed text", group: "Search", action: keySearch, contexts: searchKeyContexts, priority: 3, disabled: canSearch},
	{keys: []string{"n"}, hint: "n next match", label: "Go to the next search match", group: "Search", action: keyNextMatch, contexts: searchKeyContexts, priority: 4, disabled: canSearchNext},
	{keys: []string{"N"}, hint: "N previous match", label: "Go to the previous search match", group: "Search", action: keyPreviousMatch, contexts: searchKeyContexts, priority: 4, disabled: canSearchNext},
	{keys: []string{"1"}, hint: "1 Overview", label: "Switch to Overview", group: "Views", action: keyOverview, contexts: topKeyContexts, priority: 3},
	{keys: []string{"2"}, hint: "2 Changes", label: "Switch to Changes", group: "Views", action: keyChanges, contexts: topKeyContexts, priority: 3, disabled: needsData},
	{keys: []string{"3"}, hint: "3 Diff", label: "Switch to Diff", group: "Views", action: keyDiff, contexts: topKeyContexts, priority: 3, disabled: needsData},
	{keys: []string{"4"}, hint: "4 Activity", label: "Switch to Activity", group: "Views", action: keyActivity, contexts: topKeyContexts, priority: 3, disabled: needsData},
	{keys: []string{"tab"}, hint: "Tab next view", label: "Switch to the next view or section", group: "Views", action: keyNext, contexts: append(append([]keyContext{}, topKeyContexts...), documentKeyContexts...), priority: 3, disabled: canMoveView},
	{keys: []string{"shift+tab"}, hint: "Shift+Tab previous", label: "Switch to the previous view or section", group: "Views", action: keyPrevious, contexts: append(append([]keyContext{}, topKeyContexts...), documentKeyContexts...), priority: 3, disabled: canMoveView},
	{keys: []string{"]"}, hint: "] next file", label: "Go to the next captured file", group: "Diff", action: keyNextFile, contexts: []keyContext{contextDiff}, priority: 4, disabled: canNavigateFiles},
	{keys: []string{"["}, hint: "[ previous file", label: "Go to the previous captured file", group: "Diff", action: keyPreviousFile, contexts: []keyContext{contextDiff}, priority: 4, disabled: canNavigateFiles},
	{keys: []string{"}"}, hint: "} next hunk", label: "Go to the next indexed hunk", group: "Diff", action: keyNextHunk, contexts: []keyContext{contextDiff}, priority: 4, disabled: canNavigateHunks},
	{keys: []string{"{"}, hint: "{ previous hunk", label: "Go to the previous indexed hunk", group: "Diff", action: keyPreviousHunk, contexts: []keyContext{contextDiff}, priority: 4, disabled: canNavigateHunks},
	{keys: []string{"j", "down", "ctrl+n"}, hint: "↓/j down", label: "Move down or scroll", group: "Navigation", action: keyDown, contexts: []keyContext{contextOverview, contextChanges, contextActivity, contextInspector, contextDiff, contextPlan, contextHelp}, priority: 4},
	{keys: []string{"k", "up", "ctrl+p"}, hint: "↑/k up", label: "Move up or scroll", group: "Navigation", action: keyUp, contexts: []keyContext{contextOverview, contextChanges, contextActivity, contextInspector, contextDiff, contextPlan, contextHelp}, priority: 4},
	{keys: []string{"pgdown", "ctrl+d", "ctrl+f", "ctrl+v"}, hint: "PgDn page", label: "Move down one page", group: "Navigation", action: keyPageDown, contexts: []keyContext{contextOverview, contextChanges, contextActivity, contextInspector, contextDiff, contextPlan, contextHelp}, priority: 5},
	{keys: []string{"pgup", "ctrl+u", "ctrl+b", "alt+v"}, hint: "PgUp page", label: "Move up one page", group: "Navigation", action: keyPageUp, contexts: []keyContext{contextOverview, contextChanges, contextActivity, contextInspector, contextDiff, contextPlan, contextHelp}, priority: 5},
	{keys: []string{"home", "g", "alt+<"}, hint: "Home/g start", label: "Move to the beginning", group: "Navigation", action: keyStart, contexts: []keyContext{contextOverview, contextChanges, contextActivity, contextInspector, contextDiff, contextPlan, contextHelp}, priority: 6},
	{keys: []string{"end", "G", "alt+>"}, hint: "End/G end", label: "Move to the end", group: "Navigation", action: keyEnd, contexts: []keyContext{contextOverview, contextChanges, contextActivity, contextInspector, contextDiff, contextPlan, contextHelp}, priority: 6},
	{keys: []string{"enter"}, hint: "Enter open", label: "Inspect the selected row", group: "Navigation", action: keyEnter, contexts: listKeyContexts, priority: 2, disabled: canOpen},
	{keys: []string{"l"}, hint: "l open", label: "Inspect the selected row", group: "Navigation", action: keyEnter, contexts: listKeyContexts, priority: 7, disabled: canOpen},
	{keys: []string{"h"}, hint: "h back", label: "Return to the previous view", group: "Navigation", action: keyBack, contexts: []keyContext{contextChanges, contextActivity}, priority: 7, disabled: canGoBack},
	{keys: []string{"left"}, hint: "← pan", label: "Pan left", group: "Documents", action: keyPanLeft, contexts: []keyContext{contextOverview, contextChanges, contextInspector, contextDiff, contextPlan, contextHelp}, priority: 7, disabled: canPan},
	{keys: []string{"right"}, hint: "→ pan", label: "Pan right", group: "Documents", action: keyPanRight, contexts: []keyContext{contextOverview, contextChanges, contextInspector, contextDiff, contextPlan, contextHelp}, priority: 7, disabled: canPan},
	{keys: []string{"h"}, hint: "h pan", label: "Pan left", group: "Documents", action: keyPanLeft, contexts: []keyContext{contextInspector, contextDiff, contextPlan, contextHelp}, priority: 7, disabled: canPan},
	{keys: []string{"l"}, hint: "l pan", label: "Pan right", group: "Documents", action: keyPanRight, contexts: []keyContext{contextInspector, contextDiff, contextPlan, contextHelp}, priority: 7, disabled: canPan},
	{keys: []string{"b"}, hint: "b bytes", label: "Toggle exact-byte hex view", group: "Documents", action: keyHex, contexts: []keyContext{contextDiff, contextInspector, contextPlan}, priority: 7, disabled: canHex},
	{keys: []string{"d"}, hint: "d Changes", label: "Open the complete change inventory", group: "Views", action: keyChanges, contexts: topKeyContexts, priority: 4, disabled: needsData},
	{keys: []string{"c"}, hint: "c capture", label: "Capture the working tree in the background", group: "Review", action: keyCapture, contexts: browseKeyContexts, priority: 8, disabled: canCapture},
	{keys: []string{"i"}, hint: "i import", label: "Import the configured report", group: "Review", action: keyImport, contexts: browseKeyContexts, priority: 9, disabled: canImport},
	{keys: []string{"u"}, hint: "u use capture", label: "Choose original-base or last-inspected comparison (confirms)", group: "Review", action: keyUseCapture, contexts: browseKeyContexts, priority: 6, disabled: canUseCapture},
	{keys: []string{"p"}, hint: "p pin", label: "Pin the selected measured count (confirms)", group: "Review", action: keyPin, contexts: []keyContext{contextOverview}, priority: 8, disabled: canPin},
	{keys: []string{"a"}, hint: "a accept pin", label: "Accept a pin's current complete result (confirms)", group: "Review", action: keyAcceptPin, contexts: allKeyContexts, priority: 2, disabled: canAcceptPin},
	{keys: []string{"r"}, hint: "r rerun", label: "Prepare an exact rerun preview; nothing runs", group: "Review", action: keyPreview, contexts: browseKeyContexts, priority: 8, disabled: canPreview},
	{keys: []string{"y"}, hint: "y approve", label: "Run this exact preview once", group: "Consent", action: keyApprove, contexts: []keyContext{contextPlan}, priority: 4, disabled: canApprove},
	{keys: []string{"n"}, hint: "n deny", label: "Deny this preview without execution", group: "Consent", action: keyDeny, contexts: []keyContext{contextPlan}, priority: 5},
	{keys: []string{"s"}, hint: "s Activity", label: "Open Activity and record this session reference", group: "Session", action: keySession, contexts: []keyContext{contextOverview, contextChanges, contextDiff, contextActivity, contextInspector, contextHelp}, priority: 10, disabled: needsData},
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

func keyBindingForContext(key, screen string) (keyBinding, bool) {
	context := contextForScreen(screen)
	for _, binding := range keyMap {
		if !containsContext(binding.contexts, context) {
			continue
		}
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
	case "activity":
		return contextActivity
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
	case "help", "inspector", "plan", "inventory", "patch", "activity":
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
		if len(m.sections()) < 2 {
			return "this preview has no other section"
		}
		return ""
	}
	if screen == "examples" || screen == "inventory" || screen == "patch" || screen == "activity" {
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
	if screen == "examples" {
		return m.selectedOverviewEntry()
	}
	if screen == "inventory" || screen == "inspector" && (m.returnTo == "inventory" || m.returnTo == "examples" && m.inspectInventory) {
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
	if keyScreen(m) == "examples" {
		if row, ok := m.currentOverviewRow(); ok && row.kind == overviewGroupHeader {
			return ""
		}
	}
	if keyScreen(m) == "activity" {
		if len(m.activity) == 0 {
			return "there is no Activity event to inspect"
		}
		return ""
	}
	if selectedEntry(m, keyScreen(m)) == nil {
		return "there is no selected row to inspect"
	}
	return ""
}
func canNavigateFiles(m *Model) string {
	if m.section != 0 {
		return "switch to the displayed diff section first"
	}
	if m.hex {
		return "return to text view with b before navigating files"
	}
	if m.data == nil || m.data.Diff == nil {
		return "no diff is available"
	}
	if m.data.Diff.VisibleFiles < 2 {
		return "the displayed diff has fewer than two indexed files"
	}
	return ""
}
func canNavigateHunks(m *Model) string {
	if m.section != 0 {
		return "switch to the displayed diff section first"
	}
	if m.hex {
		return "return to text view with b before navigating hunks"
	}
	if m.data == nil || m.data.Diff == nil {
		return "no diff is available"
	}
	if len(m.data.Diff.HunkRows) < 2 {
		return "the displayed diff has fewer than two navigable hunks"
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
	if m.screen == "plan" && m.section == 0 && !m.summaryUnavailable {
		return "open Exact plan to inspect the approved preview bytes"
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
	if m.running || m.actionBusy {
		return "wait for the active run or review action to finish"
	}
	if m.jobs.Import == nil {
		return "no --import-file was configured"
	}
	return ""
}
func canUseCapture(m *Model) string {
	if m.busy {
		return "wait for the import to finish before switching candidates"
	}
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
	if m.actionBusy || m.busy || m.running {
		return "another review action, import or run is active"
	}
	entry := m.selectedOverviewEntry()
	if entry == nil || entry.Receipt == "" || entry.Expectation == "" || entry.State.Kind != evidence.Observed || entry.State.Applicability != evidence.Current || entry.State.Execution != evidence.Completed || entry.Completeness != evidence.Complete {
		return "select a complete current observation"
	}
	if len(m.selected.Evidence) >= MaxEvidence {
		return "the evidence limit has been reached"
	}
	return ""
}
func canAcceptPin(m *Model) string {
	if m.data == nil {
		return "wait for stored records to load"
	}
	if m.jobs.Actions == nil {
		return "pin actions are unavailable in this review"
	}
	if m.actionBusy || m.busy || m.running {
		return "another review action, import or run is active"
	}
	if keyScreen(m) != "examples" {
		return "select a pin on Overview"
	}
	entry := m.selectedOverviewEntry()
	if entry == nil || entry.Decision == "" || entry.PinID == "" {
		return "select a pin on Overview"
	}
	if entry.CurrentReceipt == "" || entry.State.Applicability != evidence.Current || entry.MissingCurrentResult {
		return "the selected pin has no current complete result for this pair"
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
	if m.actionBusy || m.busy || m.running {
		return "another review action, import or run is active"
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
	if m.jobCancel == nil && m.captureCancel == nil && m.runCancel == nil {
		return "no cancellable job is active"
	}
	return ""
}

func (m *Model) keyHints() string {
	if m.screen == "prompt" {
		if m.prompt != nil && m.prompt.action == keyUseCapture {
			return "←/→ mode · Enter confirm · Esc cancel · Ctrl-U clear · Ctrl-C quit"
		}
		return "Enter confirm · Esc cancel · Ctrl-U clear · Ctrl-C quit"
	}
	if m.searchEditing {
		return "Enter search · Esc clear · Backspace erase · Ctrl-U clear · Ctrl-C quit"
	}
	if m.height < 7 {
		return "? help · q quit"
	}
	bindings := append([]keyBinding(nil), keyMap...)
	if m.screen == "plan" {
		for i := range bindings {
			if bindings[i].action == keyApprove || bindings[i].action == keyDeny || bindings[i].action == keyHex {
				bindings[i].priority = 3
			}
		}
	}
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
		if m.screen == "plan" && m.width < 100 && binding.action == keyPrevious {
			continue
		}
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

// helpRow is one help line: a group heading when keys is empty.
type helpRow struct {
	group, keys, label, reason string
}

// keyName is the display spelling of a dispatch key.
func keyName(key string) string {
	names := map[string]string{
		"esc": "Esc", "enter": "Enter", "tab": "Tab", "shift+tab": "Shift-Tab", "down": "↓", "up": "↑",
		"left": "←", "right": "→", "pgdown": "PgDn", "pgup": "PgUp", "home": "Home", "end": "End",
	}
	if name, ok := names[key]; ok {
		return name
	}
	if rest, ok := strings.CutPrefix(key, "ctrl+"); ok {
		return "Ctrl-" + strings.ToUpper(rest)
	}
	if rest, ok := strings.CutPrefix(key, "alt+"); ok {
		return "Alt-" + rest
	}
	return key
}

// helpRows groups every binding in key-map order. Bindings that share a
// group and label (one action under several contexts) merge into one row.
func (m *Model) helpRows() []helpRow {
	groups := []string{}
	byGroup := make(map[string][]helpRow)
	for _, binding := range keyMap {
		if _, ok := byGroup[binding.group]; !ok {
			groups = append(groups, binding.group)
		}
		label := binding.label
		if binding.action == keyCapture {
			label = m.captureHelpLabel()
		}
		names := make([]string, len(binding.keys))
		for index, key := range binding.keys {
			names[index] = keyName(key)
		}
		reason := m.keyReason(binding, true)
		rows := byGroup[binding.group]
		merged := false
		for index := range rows {
			if rows[index].label == label {
				rows[index].keys += " " + strings.Join(names, " ")
				if reason == "" {
					rows[index].reason = ""
				}
				merged = true
			}
		}
		if !merged {
			rows = append(rows, helpRow{group: binding.group, keys: strings.Join(names, " "), label: label, reason: reason})
		}
		byGroup[binding.group] = rows
	}
	out := []helpRow{}
	for index, group := range groups {
		if index > 0 {
			out = append(out, helpRow{})
		}
		out = append(out, helpRow{group: group})
		out = append(out, byGroup[group]...)
	}
	return out
}

func (m *Model) helpLines() []string {
	rows := m.helpRows()
	lines := make([]string, len(rows))
	for index, row := range rows {
		switch {
		case row.keys == "":
			lines[index] = row.group
		case row.reason != "":
			lines[index] = "  " + row.keys + "  " + row.label + "  [unavailable: " + row.reason + "]"
		default:
			lines[index] = "  " + row.keys + "  " + row.label
		}
	}
	return lines
}

// helpView styles helpRows: group headings strong, keys accented in an
// aligned column, and unavailable rows muted with their reason.
func (m *Model) helpView(width int) []string {
	rows := m.helpRows()
	keyWidth := 0
	for _, row := range rows {
		keyWidth = max(keyWidth, uniseg.StringWidth(row.keys))
	}
	keyWidth = min(keyWidth, max(width/3, 8))
	top := min(m.top, max(len(rows)-1, 0))
	out := []string{}
	for _, row := range rows[top:min(top+m.bodyRows(), len(rows))] {
		if row.keys == "" {
			out = append(out, m.theme.Render(row.group, width, terminal.Strong, false))
			continue
		}
		keys := terminal.Line(row.keys, keyWidth)
		keys += strings.Repeat(" ", keyWidth-uniseg.StringWidth(keys))
		keyStyle, labelStyle, reason := terminal.Accent, terminal.Plain, ""
		if row.reason != "" {
			keyStyle, labelStyle, reason = terminal.Muted, terminal.Muted, "  unavailable: "+row.reason
		}
		out = append(out, m.segments(width, segment{"  ", terminal.Plain}, segment{keys, keyStyle}, segment{"  ", terminal.Plain}, segment{row.label, labelStyle}, segment{reason, terminal.Muted}))
	}
	return out
}
