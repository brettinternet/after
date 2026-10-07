package browser

import (
	"strings"
	"testing"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/terminal"
)

func TestOverviewCardPreviewIsAsynchronousAndRejectsStaleResults(t *testing.T) {
	s, selection := setup(t, false)
	selection.Evidence = []evidence.Digest{viewComparison(t, s, selection, false, evidence.Complete)}
	data, err := Load(t.Context(), selection)
	if err != nil {
		t.Fatal(err)
	}
	model := New(t.Context(), selection, Jobs{})
	t.Cleanup(model.Close)
	model.data, model.selected = data, selection
	model.screen, model.width, model.height = "examples", 120, 40
	model.theme = terminal.Theme{}
	rows := model.overviewRows()
	positions := []int{}
	for index, row := range rows {
		if row.kind == overviewEvidence {
			positions = append(positions, index)
		}
	}
	if len(positions) != 2 {
		t.Fatalf("evidence rows = %v, want two case Cards", positions)
	}
	model.selectOverviewPosition(positions[0])
	first := model.startOverviewPreview()
	firstRequest, firstKey := model.previewRequest, model.previewKey
	if first == nil || model.previewDoc != nil || firstKey == "" {
		t.Fatal("first Card preview did not remain pending off the event loop")
	}
	model.selectOverviewPosition(positions[1])
	second := model.startOverviewPreview()
	if second == nil || model.previewRequest == firstRequest || model.previewKey == firstKey {
		t.Fatal("selection change did not request a distinct Card preview")
	}
	stale := first()
	model.Update(stale)
	if model.previewDoc != nil {
		t.Fatal("stale preview replaced the pending selected Card")
	}
	model.Update(second())
	if model.previewDoc == nil {
		t.Fatal("current selected Card preview was not installed")
	}
	output := model.View()
	var preview strings.Builder
	for _, line := range strings.Split(output, "\n") {
		if separator := strings.Index(line, "│"); separator >= 0 {
			preview.WriteString(line[separator+len("│"):])
			preview.WriteByte('\n')
		}
	}
	if !strings.Contains(preview.String(), "30s same-key retry") || strings.Contains(preview.String(), "12h same-key retry") {
		t.Fatalf("preview did not follow the selected Card: %s", output)
	}
}
