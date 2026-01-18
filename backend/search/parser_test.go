package search

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		input    string
		expected Query
	}{
		{
			input: "event:Finals",
			expected: Query{
				Filters: []Filter{
					{Key: "event", Value: "Finals", Operator: OpEqual},
				},
				FreeText: []string{},
			},
		},
		{
			input: "event:\"World Series\" location:\"New York\"",
			expected: Query{
				Filters: []Filter{
					{Key: "event", Value: "World Series", Operator: OpEqual},
					{Key: "location", Value: "New York", Operator: OpEqual},
				},
				FreeText: []string{},
			},
		},
		{
			input: "is:local something",
			expected: Query{
				Filters: []Filter{
					{Key: "is", Value: "local", Operator: OpEqual},
				},
				FreeText: []string{"something"},
			},
		},
		{
			input: "date:>=\"2025-01-01\"",
			expected: Query{
				Filters: []Filter{
					{Key: "date", Value: "2025-01-01", Operator: OpGreaterOrEqual},
				},
				FreeText: []string{},
			},
		},
		{
			input: "date:<2026",
			expected: Query{
				Filters: []Filter{
					{Key: "date", Value: "2026", Operator: OpLess},
				},
				FreeText: []string{},
			},
		},
		{
			input: "date:2025-01..2025-03",
			expected: Query{
				Filters: []Filter{
					{Key: "date", Value: "2025-01", MaxValue: "2025-03", Operator: OpRange},
				},
				FreeText: []string{},
			},
		},
		{
			input: "mixed query \"free text\" key:val",
			expected: Query{
				Filters: []Filter{
					{Key: "key", Value: "val", Operator: OpEqual},
				},
				FreeText: []string{"mixed", "query", "free text"},
			},
		},
		{
			input: "broken:range:..",
			expected: Query{
				Filters:  []Filter{},
				FreeText: []string{"broken:range:.."},
			},
		},
		{
			input: "time:12:00", // Unquoted colon -> FreeText
			expected: Query{
				Filters:  []Filter{},
				FreeText: []string{"time:12:00"},
			},
		},
		{
			input: "time:\"12:00\"", // Quoted colon -> Filter
			expected: Query{
				Filters: []Filter{
					{Key: "time", Value: "12:00", Operator: OpEqual},
				},
				FreeText: []string{},
			},
		},
	}

	for _, tt := range tests {
		got := Parse(tt.input)
		// Helper to compare slices empty vs nil
		if len(got.FreeText) == 0 && len(tt.expected.FreeText) == 0 {
			got.FreeText = []string{}
			tt.expected.FreeText = []string{}
		}
		if len(got.Filters) == 0 && len(tt.expected.Filters) == 0 {
			got.Filters = []Filter{}
			tt.expected.Filters = []Filter{}
		}

		if !reflect.DeepEqual(got, tt.expected) {
			t.Errorf("Parse(%q)\ngot  %#v\nwant %#v", tt.input, got, tt.expected)
		}
	}
}
