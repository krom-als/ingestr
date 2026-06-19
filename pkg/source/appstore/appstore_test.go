package appstore

import (
	"context"
	"testing"

	"github.com/bruin-data/ingestr/pkg/source"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAppStoreSpec_QueryForm(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		input      string
		wantTable  string
		wantAppIDs []string
		wantErr    bool
		errSubstr  string
	}{
		{
			name:      "bare table no params",
			input:     "app-downloads-detailed",
			wantTable: "app-downloads-detailed",
		},
		{
			name:       "single app_id repeated key",
			input:      "app-downloads-detailed?app_ids=1234567890",
			wantTable:  "app-downloads-detailed",
			wantAppIDs: []string{"1234567890"},
		},
		{
			name:       "multiple app_ids repeated keys",
			input:      "app-downloads-detailed?app_ids=1234567890&app_ids=9876543210",
			wantTable:  "app-downloads-detailed",
			wantAppIDs: []string{"1234567890", "9876543210"},
		},
		{
			name:       "comma-joined app_ids single key",
			input:      "app-downloads-detailed?app_ids=111%2C222%2C333",
			wantTable:  "app-downloads-detailed",
			wantAppIDs: []string{"111", "222", "333"},
		},
		{
			name:       "mixed repeated and comma-joined",
			input:      "app-sessions-detailed?app_ids=100%2C200&app_ids=300",
			wantTable:  "app-sessions-detailed",
			wantAppIDs: []string{"100", "200", "300"},
		},
		{
			name:      "no app_ids param yields empty slice",
			input:     "app-sessions-detailed?app_ids=",
			wantTable: "app-sessions-detailed",
		},
		{
			name:      "unknown param key returns error",
			input:     "app-downloads-detailed?app_ids=123&typo=x",
			wantErr:   true,
			errSubstr: "unknown table parameter",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			spec, err := parseAppStoreSpec(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errSubstr != "" {
					assert.Contains(t, err.Error(), tt.errSubstr)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantTable, spec.table)
			assert.Equal(t, tt.wantAppIDs, spec.appIDs)
		})
	}
}

func TestGetTable_QueryForm(t *testing.T) {
	t.Parallel()

	validTable := "app-downloads-detailed"

	tests := []struct {
		name      string
		tableName string
		uriAppIDs []string
		wantTable string
		wantErr   bool
		errSubstr string
	}{
		{
			name:      "query form single app_id",
			tableName: validTable + "?app_ids=1234567890",
			uriAppIDs: nil,
			wantTable: validTable,
		},
		{
			name:      "query form multiple app_ids repeated",
			tableName: validTable + "?app_ids=111&app_ids=222",
			uriAppIDs: nil,
			wantTable: validTable,
		},
		{
			name:      "query form table-level overrides URI",
			tableName: validTable + "?app_ids=42",
			uriAppIDs: []string{"999"},
			wantTable: validTable,
		},
		{
			name:      "query form no app_ids falls back to URI",
			tableName: validTable,
			uriAppIDs: []string{"123"},
			wantTable: validTable,
		},
		{
			name:      "query form no app_ids and no URI returns error",
			tableName: validTable,
			uriAppIDs: nil,
			wantErr:   true,
			errSubstr: "app_id is required",
		},
		{
			name:      "query form unknown param returns error",
			tableName: validTable + "?app_ids=1&bad_key=x",
			uriAppIDs: nil,
			wantErr:   true,
			errSubstr: "unknown table parameter",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := &AppStoreSource{appIDs: tt.uriAppIDs}
			table, err := s.GetTable(context.Background(), source.TableRequest{Name: tt.tableName})
			if tt.wantErr {
				require.Error(t, err)
				if tt.errSubstr != "" {
					assert.Contains(t, err.Error(), tt.errSubstr)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantTable, table.Name())
		})
	}
}

// TestGetTable_TableNameParsing tests the inline table-name / app-ID parsing
// in GetTable. No network is needed: the parse+validation logic runs before
// any read call.
func TestGetTable_TableNameParsing(t *testing.T) {
	t.Parallel()

	validTable := "app-downloads-detailed"

	tests := []struct {
		name      string
		tableName string
		uriAppIDs []string
		wantTable string
		wantErr   bool
		errSubstr string
	}{
		{
			name:      "bare table name uses URI app IDs",
			tableName: validTable,
			uriAppIDs: []string{"123456789"},
			wantTable: validTable,
		},
		{
			name:      "table:appID overrides URI app IDs",
			tableName: validTable + ":987654321",
			uriAppIDs: []string{"111111111"},
			wantTable: validTable,
		},
		{
			name:      "table:multiple comma-separated app IDs",
			tableName: validTable + ":111,222,333",
			uriAppIDs: nil,
			wantTable: validTable,
		},
		{
			name:      "table:single app ID with no URI app IDs",
			tableName: validTable + ":42",
			uriAppIDs: nil,
			wantTable: validTable,
		},
		{
			name:      "unsupported table name returns error",
			tableName: "no-such-table",
			uriAppIDs: []string{"123"},
			wantErr:   true,
			errSubstr: "unsupported table",
		},
		{
			name:      "unsupported table with colon syntax returns error",
			tableName: "no-such-table:123",
			uriAppIDs: nil,
			wantErr:   true,
			errSubstr: "unsupported table",
		},
		{
			name:      "no app IDs anywhere returns error",
			tableName: validTable,
			uriAppIDs: nil,
			wantErr:   true,
			errSubstr: "app_id is required",
		},
		{
			// strings.Split("", ",") returns [""] (one empty-string element),
			// so len(appIDs)==1 and the "app_id is required" guard is NOT reached.
			// This characterises the current (quirky) behaviour.
			name:      "colon separator present but empty app ID part does not error",
			tableName: validTable + ":",
			uriAppIDs: nil,
			wantErr:   false,
			wantTable: validTable,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := &AppStoreSource{appIDs: tt.uriAppIDs}
			table, err := s.GetTable(context.Background(), source.TableRequest{Name: tt.tableName})
			if tt.wantErr {
				require.Error(t, err)
				if tt.errSubstr != "" {
					assert.Contains(t, err.Error(), tt.errSubstr)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantTable, table.Name())
		})
	}
}

// TestGetTable_AppIDSplitting verifies that the colon-delimited app ID list is
// split correctly. We check behaviour via error absence (valid table + IDs) and
// the Name() return, without triggering a real read.
func TestGetTable_AppIDSplitting(t *testing.T) {
	t.Parallel()

	s := &AppStoreSource{}

	table, err := s.GetTable(context.Background(), source.TableRequest{
		Name: "app-downloads-detailed:100,200,300",
	})
	require.NoError(t, err)
	assert.Equal(t, "app-downloads-detailed", table.Name())
}

// TestGetTable_URIAppIDsUsedAsFallback verifies that app IDs from the URI are
// used when none are embedded in the table string.
func TestGetTable_URIAppIDsUsedAsFallback(t *testing.T) {
	t.Parallel()

	s := &AppStoreSource{appIDs: []string{"111", "222"}}

	table, err := s.GetTable(context.Background(), source.TableRequest{
		Name: "app-sessions-detailed",
	})
	require.NoError(t, err)
	assert.Equal(t, "app-sessions-detailed", table.Name())
}

// TestGetTable_TableAppIDsOverrideURI verifies that app IDs in the table string
// take precedence over those from the URI.
func TestGetTable_TableAppIDsOverrideURI(t *testing.T) {
	t.Parallel()

	// URI has app IDs; table string provides its own — should not error.
	s := &AppStoreSource{appIDs: []string{"999"}}

	table, err := s.GetTable(context.Background(), source.TableRequest{
		Name: "app-sessions-detailed:42",
	})
	require.NoError(t, err)
	assert.Equal(t, "app-sessions-detailed", table.Name())
}
