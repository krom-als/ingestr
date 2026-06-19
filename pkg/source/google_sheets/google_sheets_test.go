package google_sheets

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTableName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		input         string
		wantID        string
		wantSheet     string
		wantErrSubstr string
	}{
		{
			name:      "simple dot form",
			input:     "fkdUQ2bjdNfUq2CA.Sheet1",
			wantID:    "fkdUQ2bjdNfUq2CA",
			wantSheet: "Sheet1",
		},
		{
			name:      "sheet name contains dot",
			input:     "abc123.My.Sheet.Name",
			wantID:    "abc123",
			wantSheet: "My.Sheet.Name",
		},
		{
			name:          "missing dot separator",
			input:         "fkdUQ2bjdNfUq2CA",
			wantErrSubstr: "invalid table name",
		},
		{
			name:          "empty spreadsheet id",
			input:         ".Sheet1",
			wantErrSubstr: "invalid table name",
		},
		{
			name:          "empty sheet name",
			input:         "fkdUQ2bjdNfUq2CA.",
			wantErrSubstr: "invalid table name",
		},
		{
			name:          "empty string",
			input:         "",
			wantErrSubstr: "invalid table name",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			id, sheet, err := parseTableName(tt.input)
			if tt.wantErrSubstr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrSubstr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantID, id)
			assert.Equal(t, tt.wantSheet, sheet)
		})
	}
}

func TestParseTableName_QueryForm(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		input         string
		wantID        string
		wantSheet     string
		wantErrSubstr string
	}{
		{
			name:      "simple query form",
			input:     "fkdUQ2bjdNfUq2CA?sheet=Sheet1",
			wantID:    "fkdUQ2bjdNfUq2CA",
			wantSheet: "Sheet1",
		},
		{
			name:      "sheet name with spaces",
			input:     "fkdUQ2bjdNfUq2CA?sheet=My Sheet",
			wantID:    "fkdUQ2bjdNfUq2CA",
			wantSheet: "My Sheet",
		},
		{
			name:          "missing sheet param value",
			input:         "fkdUQ2bjdNfUq2CA?sheet=",
			wantErrSubstr: "invalid table name",
		},
		{
			name:          "unknown param key rejected",
			input:         "fkdUQ2bjdNfUq2CA?sheett=Sheet1",
			wantErrSubstr: "unknown table parameter(s)",
		},
		{
			name:          "extra unknown param rejected",
			input:         "fkdUQ2bjdNfUq2CA?sheet=Sheet1&skip=2",
			wantErrSubstr: "unknown table parameter(s)",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			id, sheet, err := parseTableName(tt.input)
			if tt.wantErrSubstr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrSubstr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantID, id)
			assert.Equal(t, tt.wantSheet, sheet)
		})
	}
}

// TestParseTableName_Equivalence asserts that the legacy dot form and the
// URL-style query form produce identical (spreadsheetID, sheetName) results.
func TestParseTableName_Equivalence(t *testing.T) {
	cases := []struct {
		legacy   string
		queryFrm string
	}{
		{
			legacy:   "fkdUQ2bjdNfUq2CA.Sheet1",
			queryFrm: "fkdUQ2bjdNfUq2CA?sheet=Sheet1",
		},
		{
			legacy:   "abc123.Q1 Data",
			queryFrm: "abc123?sheet=Q1 Data",
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.legacy, func(t *testing.T) {
			legacyID, legacySheet, legacyErr := parseTableName(c.legacy)
			newID, newSheet, newErr := parseTableName(c.queryFrm)
			require.NoError(t, legacyErr)
			require.NoError(t, newErr)
			assert.Equal(t, legacyID, newID, "spreadsheetID mismatch")
			assert.Equal(t, legacySheet, newSheet, "sheetName mismatch")
		})
	}
}

// TestParseTableName_QuestionMarkInLegacySheet verifies that a legacy sheet
// name containing a literal "?" (no "=" after it) stays on the legacy dot path
// and is not mistaken for a query parameter block.
func TestParseTableName_QuestionMarkInLegacySheet(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantID    string
		wantSheet string
	}{
		{
			name:      "question mark in sheet name no equals",
			input:     "fkdUQ2bjdNfUq2CA.Sheet?",
			wantID:    "fkdUQ2bjdNfUq2CA",
			wantSheet: "Sheet?",
		},
		{
			name:      "question mark mid sheet name",
			input:     "fkdUQ2bjdNfUq2CA.Q1?Data",
			wantID:    "fkdUQ2bjdNfUq2CA",
			wantSheet: "Q1?Data",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			id, sheet, err := parseTableName(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.wantID, id)
			assert.Equal(t, tt.wantSheet, sheet)
		})
	}
}
