//go:build darwin

package host

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// selectFile shows a native file dialog via osascript. AppKit's NSOpenPanel must
// run on the process main thread, which the Ebiten game loop does not own, so
// GoST drives the dialog through AppleScript instead; osascript runs it in its
// own process and can safely be invoked from any goroutine.
func selectFile(spec FileDialogSpec) (string, error) {
	prompt := spec.Title
	if prompt == "" {
		if spec.ForSave {
			prompt = "Save file"
		} else {
			prompt = "Select a file"
		}
	}

	var script string
	if spec.ForSave {
		script = fmt.Sprintf(
			"try\nset theFile to choose file name with prompt %q\nPOSIX path of theFile\non error number -128\nreturn \"\"\nend try",
			prompt,
		)
	} else {
		typeClause := ""
		if exts := appleScriptTypeList(spec.Extensions); exts != "" {
			typeClause = " of type " + exts
		}
		script = fmt.Sprintf(
			"try\nset theFile to choose file with prompt %q%s\nPOSIX path of theFile\non error number -128\nreturn \"\"\nend try",
			prompt, typeClause,
		)
	}

	out, err := exec.Command("osascript", "-e", script).Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			msg := strings.TrimSpace(string(exitErr.Stderr))
			if msg == "" {
				msg = "osascript failed"
			}
			return "", fmt.Errorf("file dialog: %s", msg)
		}
		return "", fmt.Errorf("file dialog: %w", err)
	}

	path := strings.TrimRight(string(out), "\r\n")
	if strings.TrimSpace(path) == "" {
		return "", ErrFileDialogCanceled
	}
	return path, nil
}

func appleScriptTypeList(extensions []string) string {
	quoted := make([]string, 0, len(extensions))
	for _, ext := range extensions {
		ext = strings.TrimSpace(strings.TrimPrefix(ext, "."))
		if ext == "" {
			continue
		}
		quoted = append(quoted, fmt.Sprintf("%q", ext))
	}
	if len(quoted) == 0 {
		return ""
	}
	return "{" + strings.Join(quoted, ", ") + "}"
}
