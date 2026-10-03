package cases

import "testing"

func TestPass(t *testing.T) { t.Log("reported output only") }
func TestFail(t *testing.T) { t.Error("intentional failure") }
func TestSkip(t *testing.T) { t.Skip("intentional skip") }
func TestSubtests(t *testing.T) {
	t.Run("pass", func(t *testing.T) {})
	t.Run("skip", func(t *testing.T) { t.Skip("not available") })
}
func TestParallelA(t *testing.T) { t.Parallel(); t.Log("A") }
func TestParallelB(t *testing.T) { t.Parallel(); t.Log("B") }
