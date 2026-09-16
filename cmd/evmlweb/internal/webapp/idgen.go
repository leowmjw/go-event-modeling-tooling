package webapp

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	slugInvalid = regexp.MustCompile(`[^a-z0-9]+`)
	draftIDRe   = regexp.MustCompile(`^(.+)-(\d{4}-\d{2}-\d{2})-v(\d+)$`)
)

// Slugify turns free-form flow-name text into a fixture-safe kebab-case
// slug, e.g. "Hotel Booking!" -> "hotel-booking".
func Slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = slugInvalid.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// NewDraftID formats a dated, numbered draft version ID for flow.
func NewDraftID(flow, date string, seq int) string {
	return fmt.Sprintf("%s-%s-v%d", flow, date, seq)
}

// newQuestionID returns a short, collision-resistant ID for a staging
// checklist question. IDs are stable across save/load because they're
// generated at creation time and persisted alongside the draft.
func newQuestionID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return "q" + hex.EncodeToString(b)
}

// ParseDraftID splits a draft ID back into its flow name, date, and
// sequence number. It returns ok=false if id isn't in the expected shape.
func ParseDraftID(id string) (flow, date string, seq int, ok bool) {
	m := draftIDRe.FindStringSubmatch(id)
	if m == nil {
		return "", "", 0, false
	}
	n, err := strconv.Atoi(m[3])
	if err != nil {
		return "", "", 0, false
	}
	return m[1], m[2], n, true
}
