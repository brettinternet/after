# All tests pass because the test changed too: AFTER flags the edited test as a
# potential oracle and keeps imported reports separate from observed behavior.

Output examples/demos/oracle.gif
Source examples/demos/style.tape

Type `cd "$(examples/demos/setup.sh oracle 2>/dev/null)" && export XDG_CONFIG_HOME="$PWD/../config" && clear`
Enter
Wait
Show

Type "# Members got 10% off. A candidate makes it 15%, and its test agrees."
Sleep 500ms
Enter
Type "after diff"
Sleep 500ms
Enter
Wait
Sleep 5s

Hide
Type "clear"
Enter
Show
Type "go test -json ./... | after import --producer 'candidate tests'"
Sleep 500ms
Enter
Wait
Sleep 4s

Hide
Type "clear"
Enter
Show
Type "# Green. But AFTER flags the edited test:"
Sleep 500ms
Enter
Type "after inspect"
Sleep 500ms
Enter
Wait
Sleep 4s

Hide
Type "clear"
Enter
Show
Type "# The original tests, run against the candidate code:"
Sleep 500ms
Enter
Type "after import ../frozen.jsonl --producer 'base tests on candidate code'"
Sleep 500ms
Enter
Wait
Sleep 3s
Type "# Green suite, moved oracle: the discount behavior changed."
Sleep 6s
