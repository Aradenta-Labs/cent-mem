package tui

import (
	"errors"
	"os/exec"
	"strings"
)

// clipboardCmdFunc is the function used to copy text to the clipboard.
// It can be overridden in tests.
var clipboardCmdFunc = defaultCopyToClipboard

func defaultCopyToClipboard(text string) error {
	if path, err := exec.LookPath("pbcopy"); err == nil {
		cmd := exec.Command(path)
		cmd.Stdin = strings.NewReader(text)
		return cmd.Run()
	}
	if path, err := exec.LookPath("wl-copy"); err == nil {
		cmd := exec.Command(path)
		cmd.Stdin = strings.NewReader(text)
		return cmd.Run()
	}
	if path, err := exec.LookPath("xclip"); err == nil {
		cmd := exec.Command(path, "-selection", "clipboard")
		cmd.Stdin = strings.NewReader(text)
		return cmd.Run()
	}
	if path, err := exec.LookPath("xsel"); err == nil {
		cmd := exec.Command(path, "--clipboard", "--input")
		cmd.Stdin = strings.NewReader(text)
		return cmd.Run()
	}
	if path, err := exec.LookPath("clip.exe"); err == nil {
		cmd := exec.Command(path)
		cmd.Stdin = strings.NewReader(text)
		return cmd.Run()
	}
	if path, err := exec.LookPath("clip"); err == nil {
		cmd := exec.Command(path)
		cmd.Stdin = strings.NewReader(text)
		return cmd.Run()
	}
	return errors.New("clipboard: no supported tool found (pbcopy, xclip, wl-copy, clip)")
}

// CopyToClipboard writes the given string to the system clipboard.
func CopyToClipboard(text string) error {
	return clipboardCmdFunc(text)
}
