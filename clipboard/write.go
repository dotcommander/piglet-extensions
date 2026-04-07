package clipboard

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// WriteText copies text to the macOS clipboard via pbcopy.
func WriteText(text string) error {
	cmd := exec.Command("pbcopy")
	cmd.Stdin = strings.NewReader(text)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to write text to clipboard")
	}
	return nil
}

// WriteImage writes a base64-encoded image to the macOS clipboard.
func WriteImage(data string, mime string) error {
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return fmt.Errorf("invalid base64 image data: %w", err)
	}

	clipFormat := "PNG"
	if strings.Contains(mime, "jpeg") || strings.Contains(mime, "jpg") {
		clipFormat = "JPEG"
	}

	return writeImageViaTempFile(decoded, clipFormat)
}

func writeImageViaTempFile(data []byte, format string) error {
	ext := strings.ToLower(format)
	tmp := fmt.Sprintf("/tmp/piglet-clipboard-img.%s", ext)
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return fmt.Errorf("failed to write temp image: %w", err)
	}
	defer os.Remove(tmp)

	script := fmt.Sprintf(
		"set theClip to (read (POSIX file %q as \u00ABclass %s\u00BB)\n"+
			"set the clipboard to theClip",
		tmp, format,
	)
	cmd := exec.Command("osascript", "-e", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to set clipboard image: %s: %w", string(out), err)
	}
	return nil
}
