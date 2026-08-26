package handlers

import (
	"strings"
	"time"
)

// splitName does a best-effort split of a free-text full name into
// first/last, which is all the Ninja identify endpoint needs for matching.
func splitName(fullName string) (first, last string) {
	parts := strings.Fields(fullName)
	switch len(parts) {
	case 0:
		return "", ""
	case 1:
		return parts[0], parts[0]
	default:
		return parts[0], strings.Join(parts[1:], " ")
	}
}

// ageFromDOB computes age in years from a YYYY-MM-DD date string. Returns
// -1 if the date can't be parsed.
func ageFromDOB(dob string) int {
	t, err := time.Parse("2006-01-02", dob)
	if err != nil {
		return -1
	}
	now := time.Now()
	age := now.Year() - t.Year()
	if now.Month() < t.Month() || (now.Month() == t.Month() && now.Day() < t.Day()) {
		age--
	}
	return age
}
