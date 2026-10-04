package main

import (
	"crypto/sha256"
	"fmt"
	"sort"
)

type studyTrial struct {
	Case      string `json:"case"`
	Condition string `json:"condition"`
}
type studyAssignment struct {
	Participant string        `json:"participant"`
	Trials      [3]studyTrial `json:"trials"`
}

func assignments(seed string, count int) ([]studyAssignment, error) {
	if seed == "" || count < 1 || count > 18 {
		return nil, fmt.Errorf("provide a nonempty seed and 1–18 anonymous slots")
	}
	orders := [6][3]string{{"raw", "tour", "after"}, {"raw", "after", "tour"}, {"tour", "raw", "after"}, {"tour", "after", "raw"}, {"after", "raw", "tour"}, {"after", "tour", "raw"}}
	cases := []string{"A", "B", "C"}
	sort.Slice(cases, func(i, j int) bool {
		return fmt.Sprintf("%x", sha256.Sum256([]byte(seed+cases[i]))) < fmt.Sprintf("%x", sha256.Sum256([]byte(seed+cases[j])))
	})
	result := make([]studyAssignment, 0, count)
	for block := 0; len(result) < count; block++ {
		rows := []int{0, 1, 2, 3, 4, 5}
		sort.Slice(rows, func(i, j int) bool {
			return fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%s/%d/%d", seed, block, rows[i])))) < fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%s/%d/%d", seed, block, rows[j]))))
		})
		for _, row := range rows {
			if len(result) == count {
				break
			}
			a := studyAssignment{Participant: fmt.Sprintf("P%02d", len(result)+1)}
			for period := range 3 {
				a.Trials[period] = studyTrial{cases[(period+block)%3], orders[row][period]}
			}
			result = append(result, a)
		}
	}
	return result, nil
}
