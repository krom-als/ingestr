package attio

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTableName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		wantTable string
		wantParam string
	}{
		{
			name:      "bare table name returns empty param",
			input:     "objects",
			wantTable: "objects",
			wantParam: "",
		},
		{
			name:      "records with slug",
			input:     "records:companies",
			wantTable: "records",
			wantParam: "companies",
		},
		{
			name:      "list_entries with UUID",
			input:     "list_entries:8abc-123-456-789d-123",
			wantTable: "list_entries",
			wantParam: "8abc-123-456-789d-123",
		},
		{
			name:      "all_list_entries with slug",
			input:     "all_list_entries:companies",
			wantTable: "all_list_entries",
			wantParam: "companies",
		},
		{
			name:      "lists bare returns empty param",
			input:     "lists",
			wantTable: "lists",
			wantParam: "",
		},
		{
			name:      "only colon present yields empty param",
			input:     "records:",
			wantTable: "records",
			wantParam: "",
		},
		{
			name:      "param contains colon only first split applied",
			input:     "records:a:b:c",
			wantTable: "records",
			wantParam: "a:b:c",
		},
		{
			name:      "empty string returns empty table and param",
			input:     "",
			wantTable: "",
			wantParam: "",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			table, param := parseTableName(tt.input)
			assert.Equal(t, tt.wantTable, table)
			assert.Equal(t, tt.wantParam, param)
		})
	}
}

func TestParseAttioTableSpec_QueryForm(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		wantTable string
		wantParam string
		wantErr   string
	}{
		{
			name:      "records with object param",
			input:     "records?object=companies",
			wantTable: "records",
			wantParam: "companies",
		},
		{
			name:      "all_list_entries with object param",
			input:     "all_list_entries?object=companies",
			wantTable: "all_list_entries",
			wantParam: "companies",
		},
		{
			name:      "list_entries with list_id param",
			input:     "list_entries?list_id=8abc-123",
			wantTable: "list_entries",
			wantParam: "8abc-123",
		},
		{
			name:      "objects with no params",
			input:     "objects",
			wantTable: "objects",
			wantParam: "",
		},
		{
			name:      "lists with no params",
			input:     "lists",
			wantTable: "lists",
			wantParam: "",
		},
		{
			name:    "unknown param key rejected",
			input:   "records?slug=companies",
			wantErr: "unknown table parameter(s)",
		},
		{
			name:    "objects rejects object param",
			input:   "objects?object=companies",
			wantErr: "objects does not accept parameters",
		},
		{
			name:    "lists rejects list_id param",
			input:   "lists?list_id=abc",
			wantErr: "lists does not accept parameters",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			spec, err := parseAttioTableSpec(tt.input)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantTable, spec.table)
			assert.Equal(t, tt.wantParam, spec.param)
		})
	}
}

func TestParseAttioTableSpec_LegacyForm(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		wantTable string
		wantParam string
	}{
		{
			name:      "records legacy colon form",
			input:     "records:companies",
			wantTable: "records",
			wantParam: "companies",
		},
		{
			name:      "list_entries legacy colon form",
			input:     "list_entries:8abc-123-456-789d-123",
			wantTable: "list_entries",
			wantParam: "8abc-123-456-789d-123",
		},
		{
			name:      "all_list_entries legacy colon form",
			input:     "all_list_entries:companies",
			wantTable: "all_list_entries",
			wantParam: "companies",
		},
		{
			name:      "objects bare",
			input:     "objects",
			wantTable: "objects",
			wantParam: "",
		},
		{
			name:      "lists bare",
			input:     "lists",
			wantTable: "lists",
			wantParam: "",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			spec, err := parseAttioTableSpec(tt.input)
			assert.NoError(t, err)
			assert.Equal(t, tt.wantTable, spec.table)
			assert.Equal(t, tt.wantParam, spec.param)
		})
	}
}
