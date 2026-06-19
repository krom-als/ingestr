package slack

import (
	"context"
	"strings"
	"testing"

	"github.com/bruin-data/ingestr/pkg/source"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSlackURI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		uri     string
		want    string
		wantErr string
	}{
		{
			name: "valid api_key",
			uri:  "slack://?api_key=xoxb-abc123",
			want: "xoxb-abc123",
		},
		{
			name:    "wrong scheme",
			uri:     "https://slack.com/?api_key=tok",
			wantErr: "must start with slack://",
		},
		{
			name:    "missing api_key param",
			uri:     "slack://?other=val",
			wantErr: "api_key is required",
		},
		{
			name:    "empty URI body",
			uri:     "slack://",
			wantErr: "api_key is required",
		},
		{
			name:    "bare question mark only",
			uri:     "slack://?",
			wantErr: "api_key is required",
		},
		{
			name: "api_key with special characters URL-encoded",
			uri:  "slack://?api_key=xoxb-123%2F456",
			want: "xoxb-123/456",
		},
		{
			name: "extra params ignored",
			uri:  "slack://?api_key=mykey&foo=bar",
			want: "mykey",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseSlackURI(tt.uri)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetTable_SupportedPlainTables(t *testing.T) {
	t.Parallel()

	plainTables := []struct {
		name      string
		wantPKs   []string
		wantStrat string
	}{
		{"channels", []string{"id"}, "replace"},
		{"users", []string{"id"}, "replace"},
		{"access_logs", []string{"user_id"}, "append"},
	}

	s := &SlackSource{}
	ctx := context.Background()

	for _, tt := range plainTables {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tbl, err := s.GetTable(ctx, source.TableRequest{Name: tt.name})
			require.NoError(t, err)
			assert.Equal(t, tt.name, tbl.Name())
			assert.Equal(t, tt.wantPKs, tbl.PrimaryKeys())
		})
	}
}

func TestGetTable_MessagesPrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		table     string
		wantPKs   []string
		wantError bool
	}{
		{
			name:    "single channel id",
			table:   "messages:C12345678",
			wantPKs: []string{"ts", "channel"},
		},
		{
			name:    "multiple channel ids",
			table:   "messages:C111,C222,C333",
			wantPKs: []string{"ts", "channel"},
		},
		{
			name:    "single channel no comma",
			table:   "messages:CABCDEFGH",
			wantPKs: []string{"ts", "channel"},
		},
	}

	s := &SlackSource{}
	ctx := context.Background()

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tbl, err := s.GetTable(ctx, source.TableRequest{Name: tt.table})
			require.NoError(t, err)
			assert.Equal(t, tt.table, tbl.Name())
			assert.Equal(t, tt.wantPKs, tbl.PrimaryKeys())
		})
	}
}

func TestGetTable_UnsupportedTable(t *testing.T) {
	t.Parallel()

	unsupported := []string{
		"unknown_table",
		"",
		"message",  // no colon prefix
		"messages", // prefix without colon
	}

	s := &SlackSource{}
	ctx := context.Background()

	for _, table := range unsupported {
		table := table
		t.Run("table="+table, func(t *testing.T) {
			t.Parallel()
			_, err := s.GetTable(ctx, source.TableRequest{Name: table})
			require.Error(t, err)
			assert.True(t, strings.Contains(err.Error(), "unsupported table"), "expected 'unsupported table' in: %v", err)
		})
	}
}

func TestGetTable_MessagesTableName_PreservedAsIs(t *testing.T) {
	t.Parallel()

	s := &SlackSource{}
	ctx := context.Background()

	tableName := "messages:C1,C2"
	tbl, err := s.GetTable(ctx, source.TableRequest{Name: tableName})
	require.NoError(t, err)
	assert.Equal(t, tableName, tbl.Name(), "table Name() should equal the full original string")
}

func TestParseSlackTableSpec_QueryForm(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		input      string
		wantTable  string
		wantIDs    []string
		wantErrStr string
	}{
		{
			name:      "repeated channel_ids",
			input:     "messages?channel_ids=C012AB3CD&channel_ids=general",
			wantTable: "messages",
			wantIDs:   []string{"C012AB3CD", "general"},
		},
		{
			name:      "comma-joined channel_ids",
			input:     "messages?channel_ids=C012AB3CD,general",
			wantTable: "messages",
			wantIDs:   []string{"C012AB3CD", "general"},
		},
		{
			name:      "single channel_id",
			input:     "messages?channel_ids=C012AB3CD",
			wantTable: "messages",
			wantIDs:   []string{"C012AB3CD"},
		},
		{
			name:       "messages query form without channel_ids",
			input:      "messages?channel_ids=",
			wantErrStr: "messages table requires at least one channel_ids value",
		},
		{
			name:       "non-messages table with channel_ids rejected",
			input:      "channels?channel_ids=C1",
			wantErrStr: "channels table does not accept a channel_ids parameter",
		},
		{
			name:       "unknown query param",
			input:      "messages?channel_ids=C1&bad_param=x",
			wantErrStr: "unknown table parameter(s): bad_param",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			spec, err := parseSlackTableSpec(tt.input)
			if tt.wantErrStr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrStr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantTable, spec.table)
			assert.Equal(t, tt.wantIDs, spec.channelIDs)
			assert.Equal(t, tt.input, spec.rawName)
		})
	}
}

func TestGetTable_QueryForm_Messages(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		table   string
		wantPKs []string
	}{
		{
			name:    "repeated channel_ids",
			table:   "messages?channel_ids=C012AB3CD&channel_ids=general",
			wantPKs: []string{"ts", "channel"},
		},
		{
			name:    "comma-joined channel_ids",
			table:   "messages?channel_ids=C012AB3CD,general",
			wantPKs: []string{"ts", "channel"},
		},
		{
			name:    "single channel_id",
			table:   "messages?channel_ids=C012AB3CD",
			wantPKs: []string{"ts", "channel"},
		},
	}

	s := &SlackSource{}
	ctx := context.Background()

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tbl, err := s.GetTable(ctx, source.TableRequest{Name: tt.table})
			require.NoError(t, err)
			assert.Equal(t, tt.table, tbl.Name())
			assert.Equal(t, tt.wantPKs, tbl.PrimaryKeys())
		})
	}
}

func TestGetTable_QueryForm_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		table      string
		wantErrStr string
	}{
		{
			name:       "messages without channel_ids value",
			table:      "messages?channel_ids=",
			wantErrStr: "messages table requires at least one channel_ids value",
		},
		{
			name:       "non-messages table with channel_ids",
			table:      "channels?channel_ids=C1",
			wantErrStr: "channels table does not accept a channel_ids parameter",
		},
		{
			name:       "unknown param",
			table:      "messages?channel_ids=C1&unknown=x",
			wantErrStr: "unknown table parameter(s): unknown",
		},
	}

	s := &SlackSource{}
	ctx := context.Background()

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := s.GetTable(ctx, source.TableRequest{Name: tt.table})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErrStr)
		})
	}
}
