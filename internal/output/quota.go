package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/DemografixGenderize/demografix-cli/internal/api"
)

// RenderQuota writes the quota for the `quota` command. json/jsonl add a
// computed reset_at; everything else renders the human summary. now is the
// reference time for reset_at (injectable for tests).
func RenderQuota(w io.Writer, f Format, rl api.RateLimit, now time.Time) error {
	resetAt := now.Add(time.Duration(rl.Reset) * time.Second).UTC()

	payload := struct {
		api.RateLimit
		ResetAt string `json:"reset_at"`
	}{RateLimit: rl, ResetAt: resetAt.Format(time.RFC3339)}

	switch f {
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		return enc.Encode(payload)
	case FormatJSONL:
		enc := json.NewEncoder(w)
		enc.SetEscapeHTML(false)
		return enc.Encode(payload)
	default:
		fmt.Fprintf(w, "Tier       %s\n", rl.Tier)
		fmt.Fprintf(w, "Quota      %s / %s remaining\n", humanInt(rl.Remaining), humanInt(rl.Limit))
		fmt.Fprintf(w, "Resets in  %s   (%s)\n", HumanDuration(rl.Reset), resetAt.Format("2006-01-02 15:04:05 UTC"))
		return nil
	}
}

// QuotaFooter is the one-line summary printed to stderr after a predict/enrich
// run, unless quiet. Returns "" when there is no quota to show.
func QuotaFooter(q api.Quota, tier string) string {
	if q.Limit == 0 {
		return ""
	}
	left := fmt.Sprintf("%s / %s names left", humanInt(q.Remaining), humanInt(q.Limit))
	if tier != "" {
		left += " (" + tier + ")"
	}
	if q.Reset > 0 {
		left += ", resets in " + HumanDuration(q.Reset)
	}
	return "# " + left
}

func humanInt(n int) string {
	s := strconv.Itoa(n)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteByte(s[i])
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}

func HumanDuration(sec int) string {
	if sec <= 0 {
		return "0s"
	}
	d := sec / 86400
	h := (sec % 86400) / 3600
	m := (sec % 3600) / 60
	var parts []string
	if d > 0 {
		parts = append(parts, fmt.Sprintf("%dd", d))
	}
	if h > 0 {
		parts = append(parts, fmt.Sprintf("%dh", h))
	}
	if m > 0 && d == 0 {
		parts = append(parts, fmt.Sprintf("%dm", m))
	}
	if len(parts) == 0 {
		return fmt.Sprintf("%ds", sec)
	}
	return strings.Join(parts, " ")
}
