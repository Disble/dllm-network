package sqlite

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// allowedColumns is the allowlist of SQLite identifiers that may appear in
// dynamically assembled SQL for the inferences table. Every column name used
// in a format string or concatenated query must be drawn from this set.
var allowedColumns = map[string]bool{
	"id":          true,
	"at":          true,
	"model":       true,
	"endpoint":    true,
	"method":      true,
	"status":      true,
	"status_code": true,
	"streaming":   true,
	"prompt_size": true,
	"detail":      true,
	"per_sec":     true,
	"latency_ms":  true,
}

// sanitizeColumn returns the column name when it is present in the allowlist.
// The boolean reports whether the name is allowed.
func sanitizeColumn(col string) (string, bool) {
	if allowedColumns[col] {
		return col, true
	}
	return "", false
}

// sqlKeywordSet contains SQL keywords and function names that may appear in
// dynamically assembled clauses but are not column identifiers.
var sqlKeywordSet = map[string]bool{
	"where":   true,
	"and":     true,
	"or":      true,
	"is":      true,
	"not":     true,
	"null":    true,
	"in":      true,
	"between": true,
	"like":    true,
	"asc":     true,
	"desc":    true,
	"count":   true,
	"group":   true,
	"by":      true,
	"order":   true,
	"select":  true,
	"from":    true,
	"limit":   true,
}

// clauseTokenPattern matches lowercase SQL identifiers and keywords.
var clauseTokenPattern = regexp.MustCompile(`[a-z_][a-z0-9_]*`)

// validateClauseColumns returns an error if clause contains an identifier that
// is neither an allowed column nor a known SQL keyword or numeric literal.
func validateClauseColumns(clause string) error {
	if clause == "" {
		return nil
	}
	for _, tok := range clauseTokenPattern.FindAllString(clause, -1) {
		lower := strings.ToLower(tok)
		if sqlKeywordSet[lower] {
			continue
		}
		if _, err := strconv.Atoi(tok); err == nil {
			continue
		}
		if !allowedColumns[lower] {
			return fmt.Errorf("sqlite: disallowed identifier %q in SQL clause", tok)
		}
	}
	return nil
}
