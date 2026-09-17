package model

import (
	"regexp"
	"sort"
	"strings"
)

// Volatile values inside an error message are replaced in this order: composite values first,
// so their digits are not split into separate numbers. Placeholders contain no digits, which
// keeps ErrorClass idempotent and lets a class be passed back as an action filter.
var errorClassReplacements = []struct {
	pattern     *regexp.Regexp
	replacement string
	keep        func(string) bool
}{
	{
		pattern:     regexp.MustCompile(`\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(?:[.,]\d+)?(?:Z|[+-]\d{2}:?\d{2})?`),
		replacement: "<TIME>",
	},
	{
		pattern:     regexp.MustCompile(`(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b`),
		replacement: "<UUID>",
	},
	{
		pattern:     regexp.MustCompile(`\b0[xX][0-9a-fA-F]+\b`),
		replacement: "<HEX>",
	},
	{
		// Long hex tokens (hashes, trace ids). A purely alphabetic word is not a value, and a
		// purely numeric one is left to the number rule.
		pattern:     regexp.MustCompile(`\b[0-9a-fA-F]{8,}\b`),
		replacement: "<HEX>",
		keep: func(token string) bool {
			return !strings.ContainsAny(token, "0123456789") || !strings.ContainsAny(token, "abcdefABCDEF")
		},
	},
	{
		pattern:     regexp.MustCompile(`\b(?:\d+(?:\.\d+)?(?:ns|us|µs|ms|s|m|h))+\b`),
		replacement: "<DURATION>",
	},
}

// Digits glued to a preceding letter belong to a name (int64, sha256, v2) and are kept.
var errorClassNumber = regexp.MustCompile(`(^|[^\p{L}\d])\d+`)

// ErrorClass returns the failure cause of an error message with volatile values (numbers,
// UUIDs, hashes, timestamps, durations) replaced by placeholders. Messages differing only in
// such values share one class.
func ErrorClass(message string) string {
	for _, item := range errorClassReplacements {
		if item.keep == nil {
			message = item.pattern.ReplaceAllLiteralString(message, item.replacement)
			continue
		}
		message = item.pattern.ReplaceAllStringFunc(message, func(token string) string {
			if item.keep(token) {
				return token
			}
			return item.replacement
		})
	}
	return errorClassNumber.ReplaceAllString(message, "${1}<N>")
}

// GroupRepeatErrorStatsByClass merges per-message statistics into per-class statistics ordered
// by failed count. Each class keeps the most recently failed exact message as its sample.
func GroupRepeatErrorStatsByClass(stats []RepeatErrorStat) []RepeatErrorStat {
	result := make([]RepeatErrorStat, 0, len(stats))
	indexByClass := make(map[string]int, len(stats))
	for _, stat := range stats {
		sample := stat.Sample
		if sample == "" {
			sample = stat.Error
		}
		class := ErrorClass(stat.Error)
		index, ok := indexByClass[class]
		if !ok {
			indexByClass[class] = len(result)
			stat.Error = class
			stat.Sample = sample
			result = append(result, stat)
			continue
		}

		merged := &result[index]
		merged.FailedCount += stat.FailedCount
		if !stat.FirstFailedAt.IsZero() && (merged.FirstFailedAt.IsZero() || stat.FirstFailedAt.Before(merged.FirstFailedAt)) {
			merged.FirstFailedAt = stat.FirstFailedAt
		}
		if stat.LastFailedAt.After(merged.LastFailedAt) {
			merged.LastFailedAt = stat.LastFailedAt
			merged.Sample = sample
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].FailedCount != result[j].FailedCount {
			return result[i].FailedCount > result[j].FailedCount
		}
		return result[i].Error < result[j].Error
	})
	return result
}

// ErrorsOfClass returns the exact error messages that belong to the class of errorMessage.
// errorMessage may be either an exact message or a class returned by ErrorClass.
func ErrorsOfClass(messages []string, errorMessage string) []string {
	class := ErrorClass(errorMessage)
	result := make([]string, 0, len(messages))
	for _, message := range messages {
		if ErrorClass(message) == class {
			result = append(result, message)
		}
	}
	return result
}
