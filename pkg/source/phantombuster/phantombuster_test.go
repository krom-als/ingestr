package phantombuster

import (
	"context"
	"testing"

	"github.com/bruin-data/ingestr/pkg/source"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePhantombusterSpec(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		wantTable string
		wantAgent string
		wantErr   bool
		errSubstr string
	}{
		// Legacy colon form — unchanged behavior.
		{
			name:      "legacy completed_phantoms with agent_id",
			input:     "completed_phantoms:agent123",
			wantTable: "completed_phantoms",
			wantAgent: "agent123",
		},
		{
			name:      "legacy completed_phantoms with numeric agent_id",
			input:     "completed_phantoms:42",
			wantTable: "completed_phantoms",
			wantAgent: "42",
		},
		{
			name:      "legacy missing agent_id",
			input:     "completed_phantoms:",
			wantErr:   true,
			errSubstr: "agent_id is required",
		},
		{
			name:      "legacy bare table name",
			input:     "completed_phantoms",
			wantErr:   true,
			errSubstr: "unsupported table",
		},
		{
			name:      "legacy unknown table",
			input:     "other_table:123",
			wantErr:   true,
			errSubstr: "unsupported table",
		},
		// URL query-param form.
		{
			name:      "query form completed_phantoms",
			input:     "completed_phantoms?agent_id=1234567890",
			wantTable: "completed_phantoms",
			wantAgent: "1234567890",
		},
		{
			name:      "query form numeric agent_id",
			input:     "completed_phantoms?agent_id=99",
			wantTable: "completed_phantoms",
			wantAgent: "99",
		},
		{
			name:      "query form missing agent_id",
			input:     "completed_phantoms?other=value",
			wantErr:   true,
			errSubstr: "unknown table parameter",
		},
		{
			name:      "query form empty agent_id",
			input:     "completed_phantoms?agent_id=",
			wantErr:   true,
			errSubstr: "agent_id is required",
		},
		{
			name:      "query form unknown param",
			input:     "completed_phantoms?agent_id=123&typo=x",
			wantErr:   true,
			errSubstr: "unknown table parameter",
		},
		{
			name:      "query form unsupported table",
			input:     "other_table?agent_id=123",
			wantErr:   true,
			errSubstr: "unsupported table",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parsePhantombusterSpec(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errSubstr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantTable, got.table)
			assert.Equal(t, tt.wantAgent, got.agentID)
		})
	}
}

func TestGetTableQueryForm(t *testing.T) {
	t.Parallel()

	s := NewPhantombusterSource()

	tests := []struct {
		name         string
		table        string
		wantErr      bool
		errSubstr    string
		wantPKs      []string
		wantIncrKey  string
		wantStrategy string
	}{
		{
			name:         "query form completed_phantoms",
			table:        "completed_phantoms?agent_id=1234567890",
			wantPKs:      []string{"container_id"},
			wantIncrKey:  "ended_at",
			wantStrategy: "merge",
		},
		{
			name:      "query form missing agent_id value",
			table:     "completed_phantoms?agent_id=",
			wantErr:   true,
			errSubstr: "agent_id is required",
		},
		{
			name:      "query form unknown param",
			table:     "completed_phantoms?agentid=123",
			wantErr:   true,
			errSubstr: "unknown table parameter",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tbl, err := s.GetTable(context.Background(), source.TableRequest{Name: tt.table})
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errSubstr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.table, tbl.Name())
			assert.Equal(t, tt.wantPKs, tbl.PrimaryKeys())
			assert.Equal(t, tt.wantIncrKey, tbl.IncrementalKey())
			assert.Equal(t, tt.wantStrategy, string(tbl.Strategy()))
		})
	}
}

func TestParsePhantombusterURI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		uri       string
		wantKey   string
		wantErr   bool
		errSubstr string
	}{
		{
			name:    "valid URI",
			uri:     "phantombuster://?api_key=mykey123",
			wantKey: "mykey123",
		},
		{
			name:    "valid URI with extra params",
			uri:     "phantombuster://?api_key=abc&other=ignored",
			wantKey: "abc",
		},
		{
			name:      "wrong scheme",
			uri:       "https://?api_key=mykey",
			wantErr:   true,
			errSubstr: "must start with phantombuster://",
		},
		{
			name:      "missing api_key",
			uri:       "phantombuster://?other=value",
			wantErr:   true,
			errSubstr: "api_key is required",
		},
		{
			name:      "empty after scheme",
			uri:       "phantombuster://",
			wantErr:   true,
			errSubstr: "api_key is required",
		},
		{
			name:      "only question mark",
			uri:       "phantombuster://?",
			wantErr:   true,
			errSubstr: "api_key is required",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parsePhantombusterURI(tt.uri)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errSubstr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantKey, got)
		})
	}
}

func TestPhantombusterGetTable(t *testing.T) {
	t.Parallel()

	s := NewPhantombusterSource()

	tests := []struct {
		name         string
		table        string
		wantErr      bool
		errSubstr    string
		wantPKs      []string
		wantIncrKey  string
		wantStrategy string
	}{
		{
			name:         "completed_phantoms with agent_id",
			table:        "completed_phantoms:agent123",
			wantPKs:      []string{"container_id"},
			wantIncrKey:  "ended_at",
			wantStrategy: "merge",
		},
		{
			name:         "completed_phantoms with numeric agent_id",
			table:        "completed_phantoms:42",
			wantPKs:      []string{"container_id"},
			wantIncrKey:  "ended_at",
			wantStrategy: "merge",
		},
		{
			name:      "completed_phantoms without agent_id",
			table:     "completed_phantoms:",
			wantErr:   true,
			errSubstr: "agent_id is required",
		},
		{
			name:      "completed_phantoms bare without colon",
			table:     "completed_phantoms",
			wantErr:   true,
			errSubstr: "unsupported table",
		},
		{
			name:      "unknown table",
			table:     "unknown_table",
			wantErr:   true,
			errSubstr: "unsupported table",
		},
		{
			name:      "empty table name",
			table:     "",
			wantErr:   true,
			errSubstr: "unsupported table",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tbl, err := s.GetTable(context.Background(), source.TableRequest{Name: tt.table})
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errSubstr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.table, tbl.Name())
			assert.Equal(t, tt.wantPKs, tbl.PrimaryKeys())
			assert.Equal(t, tt.wantIncrKey, tbl.IncrementalKey())
			assert.Equal(t, tt.wantStrategy, string(tbl.Strategy()))
		})
	}
}
