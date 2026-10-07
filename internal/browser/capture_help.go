package browser

func (m *Model) captureHelpLabel() string {
	switch {
	case m.session.Capture.Staged:
		return "Capture HEAD against the saved index (--staged)"
	case m.session.Capture.Base != "":
		return "Capture again with the saved --base/--target refs"
	case len(m.session.Capture.IncludeUntracked) > 0:
		return "Capture HEAD against the working tree with saved untracked paths"
	default:
		return "Capture HEAD against the working tree"
	}
}
