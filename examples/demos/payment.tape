# README hero and example 05: a one-line change with identical HTTP responses,
# and the duplicate charge AFTER observes when it runs both versions. Needs Docker.

Output examples/demos/payment.gif
Source examples/demos/style.tape
Set Width 1200
Set Height 640
Source examples/demos/prelude.tape
Require docker

Type `export AFTER_DOCKER_BINARY="$(command -v docker)" AFTER_DOCKER_HOST="$(docker context inspect --format '{{.Endpoints.docker.Host}}')"`
Enter
Type `cd "$(examples/demos/setup.sh payment 2>/dev/null)" && export XDG_CONFIG_HOME="$PWD/../config" && clear`
Enter
Wait
Show

Type "# One line of a payment service changed. Tests pass. Ship it?"
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
Type "# Capture both versions, then run them in AFTER's offline sandbox:"
Sleep 500ms
Enter
Type "after capture && after run"
Sleep 500ms
Enter
# Wait+Screen does not follow scrolling; the consent prompt appears within a second.
Sleep 5s
Type "yes"
Sleep 500ms
Enter
Sleep 1s
# Skip the sandbox run time.
Hide
Wait
Show
Sleep 3s
Type "# Same responses, but a retry after 12h now charges the card twice."
Sleep 7s
