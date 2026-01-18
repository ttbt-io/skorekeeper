package search

import (
	"strings"
	"unicode"
)

// Operator defines the type of comparison for a filter.
type Operator string

const (
	OpEqual          Operator = "="
	OpGreater        Operator = ">"
	OpGreaterOrEqual Operator = ">="
	OpLess           Operator = "<"
	OpLessOrEqual    Operator = "<="
	OpRange          Operator = ".." // for date:2025-01..2025-02
)

// Filter represents a structured criteria derived from the query string.
type Filter struct {
	Key      string   // e.g., "event", "date", "is"
	Value    string   // e.g., "Finals", "2025-01-01", "local"
	MaxValue string   // Used only for OpRange
	Operator Operator // e.g., "=", ">="
}

// Query represents the parsed search query.
type Query struct {
	Filters  []Filter
	FreeText []string
}

// Parse parses a search query string into a structured Query object.
// It handles:
// - quoted strings (key:"value with spaces")
// - key:value pairs
// - simple operators for specific keys (mainly date)
// - flags (is:local)
func Parse(input string) Query {
	q := Query{
		Filters:  make([]Filter, 0),
		FreeText: make([]string, 0),
	}

	tokens := tokenize(input)

	for _, token := range tokens {
		// Check for key:value pattern
		// We split by first colon.
		parts := strings.SplitN(token, ":", 2)
		if len(parts) == 2 {
			key := strings.ToLower(strings.TrimSpace(parts[0]))
			val := strings.TrimSpace(parts[1])

			// If value contains unquoted colon, treat as free text to avoid ambiguity (e.g. broken:range:..)
			// Exception: if starts with quote
			if strings.Contains(val, ":") && !strings.HasPrefix(val, "\"") && !strings.HasPrefix(val, "'") {
				q.FreeText = append(q.FreeText, token)
				continue
			}

			if key == "" || val == "" {
				// Treat as free text if key or value is empty (e.g. "foo:")
				q.FreeText = append(q.FreeText, token)
				continue
			}

			// Parse Value for Operators (mainly for date)
			// Check for range ".."
			if strings.Contains(val, "..") {
				rangeParts := strings.SplitN(val, "..", 2)
				if len(rangeParts) == 2 {
					q.Filters = append(q.Filters, Filter{
						Key:      key,
						Value:    rangeParts[0],
						MaxValue: rangeParts[1],
						Operator: OpRange,
					})
					continue
				}
			}

			// Check for >=, <=, >, <
			// Note: tokenization keeps value intact.
			// "date:>=2025" -> key="date", val=">=2025"
			if strings.HasPrefix(val, ">=") {
				q.Filters = append(q.Filters, Filter{
					Key:      key,
					Value:    removeQuotes(strings.TrimPrefix(val, ">=")),
					Operator: OpGreaterOrEqual,
				})
			} else if strings.HasPrefix(val, "<=") {
				q.Filters = append(q.Filters, Filter{
					Key:      key,
					Value:    removeQuotes(strings.TrimPrefix(val, "<=")),
					Operator: OpLessOrEqual,
				})
			} else if strings.HasPrefix(val, ">") {
				q.Filters = append(q.Filters, Filter{
					Key:      key,
					Value:    removeQuotes(strings.TrimPrefix(val, ">")),
					Operator: OpGreater,
				})
			} else if strings.HasPrefix(val, "<") {
				q.Filters = append(q.Filters, Filter{
					Key:      key,
					Value:    removeQuotes(strings.TrimPrefix(val, "<")),
					Operator: OpLess,
				})
			} else {
				// Default Equal
				// Also removes quotes from value if present
				cleanedVal := removeQuotes(val)
				q.Filters = append(q.Filters, Filter{
					Key:      key,
					Value:    cleanedVal,
					Operator: OpEqual,
				})
			}
		} else {
			q.FreeText = append(q.FreeText, removeQuotes(token))
		}
	}

	return q
}

// tokenize splits the string by spaces, respecting quotes.
func tokenize(input string) []string {
	var tokens []string
	var currentToken strings.Builder
	inQuote := false
	quoteChar := rune(0)

	for _, r := range input {
		switch {
		case inQuote:
			if r == quoteChar {
				inQuote = false
				currentToken.WriteRune(r)
			} else {
				currentToken.WriteRune(r)
			}
		case unicode.IsSpace(r):
			if currentToken.Len() > 0 {
				tokens = append(tokens, currentToken.String())
				currentToken.Reset()
			}
		case r == '"' || r == '\'':
			inQuote = true
			quoteChar = r
			currentToken.WriteRune(r)
		default:
			currentToken.WriteRune(r)
		}
	}
	if currentToken.Len() > 0 {
		tokens = append(tokens, currentToken.String())
	}
	return tokens
}

func removeQuotes(s string) string {
	if len(s) >= 2 {
		first := s[0]
		last := s[len(s)-1]
		if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
