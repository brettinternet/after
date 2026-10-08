# Shared look for the demo tapes. Regenerate every GIF from the repository
# root with: task demo:gifs (see examples/README.md).

Require ttyd
Require git
Require go
Require jq

Set Shell "bash"
Set FontSize 18
Set Width 1400
Set Height 820
Set Padding 24
Set Margin 16
Set MarginFill "#11111b"
Set BorderRadius 10
Set WindowBar "Colorful"
Set Theme "Catppuccin Mocha"
Set TypingSpeed 40ms
Set Framerate 20
Set WaitPattern /^\$\s*$/
Set WaitTimeout 3m
