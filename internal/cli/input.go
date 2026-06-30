package cli

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/term"
)

// collectNames reads names from positional args, or one-per-line from stdin when
// no args are given (or when the sole arg is "-"). Blank lines are skipped and
// surrounding whitespace trimmed; internal spaces are preserved.
func (a *App) collectNames(args []string) ([]string, error) {
	if len(args) == 1 && args[0] == "-" {
		return readLines(a.In)
	}
	if len(args) > 0 {
		return args, nil
	}
	if a.In != nil && !term.IsTerminal(int(a.In.Fd())) {
		names, err := readLines(a.In)
		if err != nil {
			return nil, err
		}
		if len(names) > 0 {
			return names, nil
		}
	}
	return nil, UsageErrorf("no names provided (pass names as arguments or pipe them on stdin)")
}

func readLines(in io.Reader) ([]string, error) {
	var names []string
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		names = append(names, line)
	}
	return names, sc.Err()
}

// readSecret reads an API key without echo from a terminal, or a single line
// when stdin is not interactive (scripts, tests).
func readSecret(in *os.File) (string, error) {
	if term.IsTerminal(int(in.Fd())) {
		b, err := term.ReadPassword(int(in.Fd()))
		return string(b), err
	}
	r := bufio.NewReader(in)
	line, err := r.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return line, nil
}

func normalizeCountry(c string) string { return strings.ToUpper(strings.TrimSpace(c)) }

func validateCountry(c string) error {
	if c == "" {
		return nil
	}
	if len(c) != 2 || !isAlpha(c) {
		return UsageErrorf("--country must be a 2-letter ISO code (got %q)", c)
	}
	return nil
}

func isAlpha(s string) bool {
	for _, r := range s {
		if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')) {
			return false
		}
	}
	return true
}

func isTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

// prettyPath replaces the home dir with ~ for display.
func prettyPath(p string) string {
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(p, home) {
		return filepath.Join("~", strings.TrimPrefix(p, home))
	}
	return p
}
