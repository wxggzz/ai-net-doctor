# Demo recording for ai-net-doctor.
# Install vhs (https://github.com/charmbracelet/vhs), then run:
#   vhs docs/demo.tape
# It writes docs/demo.gif; uncomment the image line in README.md to show it.

Output docs/demo.gif

Set Shell "bash"
Set FontSize 15
Set Width 1100
Set Height 760
Set Padding 18

# Build quietly and put the binary on PATH, then clear the screen.
Hide
Type "go build -o /tmp/ai-net-doctor ./cmd/ai-net-doctor && export PATH=/tmp:$PATH && clear"
Enter
Sleep 2s
Show

Sleep 500ms
Type "ai-net-doctor --verbose"
Sleep 800ms
Enter
Sleep 7s
