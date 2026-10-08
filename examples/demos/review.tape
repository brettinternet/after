# The review TUI on a messy change with no evidence: every path stays visible,
# the edited golden file is flagged, and the untracked secret is never captured.

Output examples/demos/review.gif
Source examples/demos/style.tape
Source examples/demos/prelude.tape

Type `cd "$(examples/demos/setup.sh review 2>/dev/null)" && export XDG_CONFIG_HOME="$PWD/../config" && clear`
Enter
Wait
Show

Type "after review"
Sleep 500ms
Enter
Sleep 4s
Type "2"
Sleep 4s
Down
Sleep 2s
Down
Sleep 2s
Down
Sleep 2s
Down 3
Sleep 3s
Type "q"
Sleep 1s
