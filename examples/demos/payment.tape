# Same responses, different behavior: a one-line retention change passes review
# by diff, but AFTER observes a duplicate provider charge. Needs Docker.

Output examples/demos/payment.gif
Source examples/demos/style.tape
Require docker

Type `export AFTER_DOCKER_BINARY="$(command -v docker)" AFTER_DOCKER_HOST="$(docker context inspect --format '{{.Endpoints.docker.Host}}')"`
Enter
Type `cd "$(examples/demos/setup.sh payment 2>/dev/null)" && export XDG_CONFIG_HOME="$PWD/../config" && clear`
Enter
Wait
Show

Type "# A one-line change to a payment service. Looks harmless?"
Sleep 500ms
Enter
Type "after diff"
Sleep 500ms
Enter
Wait
Sleep 4s

Hide
Type "clear"
Enter
Show
Type "after capture"
Sleep 500ms
Enter
Wait
Sleep 2s

Hide
Type "clear"
Enter
Show
Type "# Run base and candidate in the offline sandbox: POST, then retry the same key"
Sleep 500ms
Enter
Type "after run"
Sleep 500ms
Enter
Wait+Screen /Type yes/
Sleep 3s
Type "yes"
Sleep 500ms
Enter
Sleep 2s
# Skip the sandbox run time.
Hide
Wait
Show
Sleep 3s
Type "# Identical responses, but a retry after 12h now charges the card twice."
Sleep 6s
