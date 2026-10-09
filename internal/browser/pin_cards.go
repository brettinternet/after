package browser

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/review"
	"github.com/brettinternet/after/internal/store"
)

func pinSections(s *store.Store, id evidence.Digest, pin evidence.Pin, view review.View) ([]Section, []evidence.Digest, []Entry, error) {
	if len(pin.History) == 0 {
		return nil, nil, nil, fmt.Errorf("pin has no history")
	}
	last := pin.History[len(pin.History)-1]
	target := last.Review.Target
	current := "no current result attached"
	if view.CurrentReceipt != nil {
		current = fmt.Sprintf("receipt %s · %s · %s · %s", shortID(view.CurrentReceipt.ID), view.CurrentReceipt.State.Execution, view.CurrentReceipt.State.Comparison, view.CurrentReceipt.Completeness)
	}
	card := cardSection(
		textPart("Expectation", pin.Expectation+" (your words; AFTER does not evaluate them)"),
		textPart("Scope", fmt.Sprintf("%s · basis receipt %s", pin.Scope, shortID(pin.BasisReceipt))),
		textPart("Reviewing", fmt.Sprintf("%s %s → %s", reviewModeLabel(last.Review.Mode), shortID(target.Snapshots.Base), shortID(target.Snapshots.Candidate))),
		textPart("Evidence", fmt.Sprintf("%s · %s", view.Applicability, view.Reason)),
		textPart("Current", current),
	)
	sections := []Section{card}
	ids := []evidence.Digest{id, pin.Scenario, pin.BasisReceipt, pin.BasisSnapshots.Base, pin.BasisSnapshots.Candidate}
	historyParts := []Part{}
	seenReceipts := map[evidence.Digest]bool{}
	receiptOrder := []evidence.Digest{}
	for index, event := range pin.History {
		eventRaw, err := json.MarshalIndent(event, "", "  ")
		if err != nil {
			return nil, nil, nil, err
		}
		summary := event.At.Format(time.RFC3339) + " · " + event.Reason
		if event.Review != nil && event.Review.Receipt != "" {
			summary += " · receipt " + shortID(event.Review.Receipt)
		}
		historyParts = append(historyParts,
			textPart(fmt.Sprintf("event %d · %s", index+1, event.Decision), summary),
			Part{Title: fmt.Sprintf("history · event %d · %s", index+1, event.Decision), Content: eventRaw, detail: true})
		if event.Review == nil {
			continue
		}
		target := event.Review.Target
		ids = appendIDs(ids, event.Review.Receipt, target.Snapshots.Base, target.Snapshots.Candidate)
		if target.Bindings != nil {
			ids = appendIDs(ids, target.Bindings.Scenario, target.Bindings.Input, target.Bindings.Driver, target.Bindings.Observer, target.Bindings.Rules)
		}
		for _, environment := range []*evidence.Environment{target.BaseEnvironment, target.CandidateEnvironment} {
			if environment != nil {
				ids = appendIDs(ids, environment.Environment, environment.Toolchain, environment.Dependencies)
			}
		}
		if event.Review.Receipt == "" || seenReceipts[event.Review.Receipt] {
			continue
		}
		seenReceipts[event.Review.Receipt] = true
		receiptOrder = append(receiptOrder, event.Review.Receipt)
		receipt, err := store.Get[evidence.Receipt](s, event.Review.Receipt)
		if err != nil {
			historyParts = append(historyParts, textPart("historical receipt", "Receipt unavailable; retained ID: "+string(event.Review.Receipt)))
			continue
		}
		parts, receiptIDs := receiptParts(s, receipt)
		historyParts = append(historyParts, parts...)
		ids = appendIDs(ids, receiptIDs...)
	}
	sections = append(sections, Section{Name: "History", Parts: historyParts})
	if view.CurrentReceipt != nil {
		parts, receiptIDs := receiptParts(s, *view.CurrentReceipt)
		sections = append(sections, Section{Name: "Current result", Parts: parts})
		ids = appendIDs(ids, receiptIDs...)
	} else {
		sections = append(sections, Section{Name: "Current result", Parts: []Part{textPart("current result", "No result is attached to the selected review basis.")}})
	}
	if basis, err := store.Get[evidence.Receipt](s, pin.BasisReceipt); err == nil {
		parts, receiptIDs := receiptParts(s, basis)
		sections = append(sections, Section{Name: "Basis receipt", Parts: parts})
		ids = appendIDs(ids, receiptIDs...)
	} else {
		sections = append(sections, Section{Name: "Basis receipt", Parts: []Part{recordPart(s, "receipt", pin.BasisReceipt, "basis receipt")}})
		ids = appendIDs(ids, pin.BasisReceipt)
	}
	sections = append(sections, idsSection(ids))
	rows := []Entry{}
	for _, receiptID := range receiptOrder {
		if receiptID == "" || view.CurrentReceipt != nil && receiptID == view.CurrentReceipt.ID {
			continue
		}
		entries, err := loadEvidence(s, receiptID, target.Snapshots)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.State.Kind == evidence.Observed && entry.State.Applicability == evidence.Stale {
				rows = append(rows, entry)
			}
		}
	}
	return sections, ids, rows, nil
}

func receiptParts(s *store.Store, receipt evidence.Receipt) ([]Part, []evidence.Digest) {
	parts := []Part{recordPart(s, "receipt", receipt.ID, "receipt · "+shortID(receipt.ID))}
	ids := []evidence.Digest{receipt.ID, receipt.Snapshots.Base, receipt.Snapshots.Candidate}
	if receipt.Bindings != nil {
		ids = appendIDs(ids, receipt.Bindings.Scenario, receipt.Bindings.Input, receipt.Bindings.Driver, receipt.Bindings.Observer, receipt.Bindings.Rules)
		if scenario, err := store.Get[evidence.Scenario](s, receipt.Bindings.Scenario); err == nil {
			scenarioSection, scenarioIDs := scenarioSection(s, scenario)
			parts = append(parts, scenarioSection.Parts...)
			ids = appendIDs(ids, scenarioIDs...)
		}
	}
	known, other := orderedArtifacts(receipt.Artifacts)
	for _, item := range known {
		parts = append(parts, blobPart(s, item.artifact.Content, artifactTitle(item), artifactFormat(item.artifact.Channel)))
		ids = appendIDs(ids, item.artifact.Content)
	}
	for _, artifact := range other {
		title := "Other artifacts · " + strconv.Quote(artifact.Channel)
		parts = append(parts, blobPart(s, artifact.Content, title, formatVerbatim))
		ids = appendIDs(ids, artifact.Content)
	}
	comparisons, err := s.List("comparison")
	if err != nil {
		parts = append(parts, textPart("comparison history limitation", "Stored comparisons could not be listed; receipt artifacts remain available."))
	} else {
		for _, entry := range comparisons {
			comparison, err := store.Get[evidence.Comparison](s, entry.ID)
			if err != nil || comparison.Receipt != receipt.ID {
				continue
			}
			parts = append(parts, recordPart(s, "comparison", comparison.ID, "comparison · "+shortID(comparison.ID)))
			ids = appendIDs(ids, comparison.ID)
			if comparison.Details != nil {
				parts = append(parts, blobPart(s, comparison.Details.Content, "comparison details · "+shortID(comparison.Details.Content), formatComparison))
				ids = appendIDs(ids, comparison.Details.Content)
			}
		}
	}
	return parts, ids
}
