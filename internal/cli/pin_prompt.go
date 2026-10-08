package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/brettinternet/after/internal/browser"
	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/store"
	"github.com/brettinternet/after/internal/terminal"
)

func promptPin(state *invocation, s *store.Store, id evidence.Digest, scope evidence.PinScope, interactive bool) (string, bool, error) {
	if !state.tty || !interactive {
		return "", false, invalidWithFix("pin creation needs an expectation without a terminal", "try: after pin "+string(id)+" --expectation 'Describe the expected behavior'"+state.suggestionFlags)
	}
	receipt, err := store.Get[evidence.Receipt](s, id)
	if err != nil {
		return "", false, operational("receipt is corrupt or unavailable")
	}
	suggestions, err := browser.PinSuggestions(s, receipt)
	if err != nil {
		return "", false, operational("pin suggestions are unavailable: case artifacts are missing or invalid")
	}
	lines := []string{"Pin an expectation (no execution).", "Scope: " + string(scope), "Basis receipt:", string(id), "Base:", string(receipt.Snapshots.Base), "Candidate:", string(receipt.Snapshots.Candidate)}
	for i, suggestion := range suggestions {
		lines = append(lines, fmt.Sprintf("%d. %s", i+1, suggestion))
	}
	for _, line := range lines {
		for _, wrapped := range terminal.Wrap(line, state.columns) {
			if _, err := fmt.Fprintln(state.stderr, wrapped); err != nil {
				return "", false, operational("cannot write pin prompt")
			}
		}
	}
	question := "Expectation (empty line cancels): "
	if len(suggestions) > 0 {
		question = "Number or expectation (empty line cancels): "
	}
	if _, err := fmt.Fprint(state.stderr, question); err != nil {
		return "", false, operational("cannot write pin prompt")
	}
	// Bound input and require a complete line. EOF, including a partial line,
	// never records a human decision.
	line, err := bufio.NewReaderSize(state.reader, 4098).ReadSlice('\n')
	if errors.Is(err, io.EOF) {
		return "", true, nil
	}
	if err != nil {
		return "", false, invalidWithFix("expectation input is unavailable or exceeds 4096 bytes", "use --expectation TEXT with 1 to 4096 bytes")
	}
	expectation := strings.TrimSuffix(strings.TrimSuffix(string(line), "\n"), "\r")
	if expectation == "" {
		return "", true, nil
	}
	if number, err := strconv.Atoi(expectation); err == nil && number >= 1 && number <= len(suggestions) {
		expectation = suggestions[number-1]
	}
	if strings.TrimSpace(expectation) == "" || len(expectation) > 4096 {
		return "", false, invalidWithFix("expectation must contain 1 to 4096 bytes", "use --expectation TEXT with 1 to 4096 bytes")
	}
	return expectation, false, nil
}
