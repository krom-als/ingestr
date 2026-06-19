package fluxx

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFluxxURI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		uri          string
		wantInstance string
		wantClientID string
		wantSecret   string
		wantErr      bool
		errSubstr    string
	}{
		{
			name:         "valid uri",
			uri:          "fluxx://mycompany.preprod?client_id=abc&client_secret=xyz",
			wantInstance: "mycompany.preprod",
			wantClientID: "abc",
			wantSecret:   "xyz",
		},
		{
			name:         "instance with dots",
			uri:          "fluxx://company.fluxx.io?client_id=id1&client_secret=s1",
			wantInstance: "company.fluxx.io",
			wantClientID: "id1",
			wantSecret:   "s1",
		},
		{
			name:      "missing fluxx scheme",
			uri:       "http://mycompany?client_id=abc&client_secret=xyz",
			wantErr:   true,
			errSubstr: "http://",
		},
		{
			name:      "https rejected",
			uri:       "https://mycompany?client_id=abc&client_secret=xyz",
			wantErr:   true,
			errSubstr: "https://",
		},
		{
			name:      "wrong scheme",
			uri:       "postgres://mycompany?client_id=abc&client_secret=xyz",
			wantErr:   true,
			errSubstr: "must start with fluxx://",
		},
		{
			name:      "empty instance",
			uri:       "fluxx://?client_id=abc&client_secret=xyz",
			wantErr:   true,
			errSubstr: "instance is required",
		},
		{
			name:      "bare fluxx scheme",
			uri:       "fluxx://",
			wantErr:   true,
			errSubstr: "instance is required",
		},
		{
			name:      "instance only, no query string",
			uri:       "fluxx://mycompany",
			wantErr:   true,
			errSubstr: "client_id and client_secret are required",
		},
		{
			name:      "missing client_id",
			uri:       "fluxx://mycompany?client_secret=xyz",
			wantErr:   true,
			errSubstr: "client_id",
		},
		{
			name:      "missing client_secret",
			uri:       "fluxx://mycompany?client_id=abc",
			wantErr:   true,
			errSubstr: "client_secret",
		},
		{
			name:      "empty client_id",
			uri:       "fluxx://mycompany?client_id=&client_secret=xyz",
			wantErr:   true,
			errSubstr: "client_id",
		},
		{
			name:      "empty client_secret",
			uri:       "fluxx://mycompany?client_id=abc&client_secret=",
			wantErr:   true,
			errSubstr: "client_secret",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			inst, cid, sec, err := parseFluxxURI(tt.uri)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errSubstr != "" {
					assert.Contains(t, err.Error(), tt.errSubstr)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantInstance, inst)
			assert.Equal(t, tt.wantClientID, cid)
			assert.Equal(t, tt.wantSecret, sec)
		})
	}
}

func TestParseTableSpec(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		table           string
		wantResources   []string
		wantCustomField map[string][]string
		wantErr         bool
		errSubstr       string
	}{
		{
			name:            "single resource",
			table:           "grant_request",
			wantResources:   []string{"grant_request"},
			wantCustomField: map[string][]string{},
		},
		{
			name:            "comma-separated resources",
			table:           "grant_request,user",
			wantResources:   []string{"grant_request", "user"},
			wantCustomField: map[string][]string{},
		},
		{
			name:            "three resources",
			table:           "grant_request,user,organization",
			wantResources:   []string{"grant_request", "user", "organization"},
			wantCustomField: map[string][]string{},
		},
		{
			name:            "resource with whitespace trimmed",
			table:           " grant_request , user ",
			wantResources:   []string{"grant_request", "user"},
			wantCustomField: map[string][]string{},
		},
		{
			name:          "resource with custom fields",
			table:         "grant_request:amount_requested,project_title",
			wantResources: []string{"grant_request"},
			wantCustomField: map[string][]string{
				"grant_request": {"amount_requested", "project_title"},
			},
		},
		{
			name:          "custom fields with whitespace trimmed",
			table:         "grant_request: amount_requested , project_title ",
			wantResources: []string{"grant_request"},
			wantCustomField: map[string][]string{
				"grant_request": {"amount_requested", "project_title"},
			},
		},
		{
			name:          "custom fields single field",
			table:         "grant_request:amount_requested",
			wantResources: []string{"grant_request"},
			wantCustomField: map[string][]string{
				"grant_request": {"amount_requested"},
			},
		},
		{
			name:      "empty table string",
			table:     "",
			wantErr:   true,
			errSubstr: "table specification is required",
		},
		{
			name:      "empty resource name in colon form",
			table:     ":amount_requested",
			wantErr:   true,
			errSubstr: "resource name is required",
		},
		{
			name:      "empty fields in colon form",
			table:     "grant_request:",
			wantErr:   true,
			errSubstr: "at least one field is required",
		},
		{
			name:      "only commas",
			table:     ",,",
			wantErr:   true,
			errSubstr: "at least one resource is required",
		},
		{
			name:      "whitespace only",
			table:     "   ",
			wantErr:   true,
			errSubstr: "at least one resource is required",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseTableSpec(tt.table)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errSubstr != "" {
					assert.Contains(t, err.Error(), tt.errSubstr)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantResources, got.resources)
			assert.Equal(t, tt.wantCustomField, got.customFields)
		})
	}
}

// Two colons means only one colon → goes through comma-split path.
// The current code checks strings.Count(table, ":") == 1 for the colon form.
// A table string with two colons (e.g. "a:b:c") is treated as comma-split,
// producing a single resource named "a:b:c" (which will later fail resource validation).
func TestParseTableSpec_MultipleColonsGoesToCommaPath(t *testing.T) {
	t.Parallel()
	got, err := parseTableSpec("grant_request:field1:field2")
	require.NoError(t, err)
	assert.Equal(t, []string{"grant_request:field1:field2"}, got.resources)
	assert.Empty(t, got.customFields)
}

func TestParseFluxxTableSpec_QueryForm(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		table           string
		wantResources   []string
		wantCustomField map[string][]string
		wantErr         bool
		errSubstr       string
	}{
		{
			name:          "query form with fields",
			table:         "grant_request?fields=id,amount_requested,status",
			wantResources: []string{"grant_request"},
			wantCustomField: map[string][]string{
				"grant_request": {"id", "amount_requested", "status"},
			},
		},
		{
			name:          "query form single field",
			table:         "grant_request?fields=id",
			wantResources: []string{"grant_request"},
			wantCustomField: map[string][]string{
				"grant_request": {"id"},
			},
		},
		{
			name:      "query form no fields param",
			table:     "grant_request?fields=",
			wantErr:   true,
			errSubstr: "at least one field is required",
		},
		{
			name:      "query form unknown param",
			table:     "grant_request?resources=foo",
			wantErr:   true,
			errSubstr: "unknown table parameter",
		},
		{
			name:      "query form empty resource",
			table:     "?fields=id",
			wantErr:   true,
			errSubstr: "resource name is required",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseFluxxTableSpec(tt.table)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errSubstr != "" {
					assert.Contains(t, err.Error(), tt.errSubstr)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantResources, got.resources)
			assert.Equal(t, tt.wantCustomField, got.customFields)
		})
	}
}

func TestParseFluxxTableSpec_LegacyPassthrough(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		table           string
		wantResources   []string
		wantCustomField map[string][]string
		wantErr         bool
		errSubstr       string
	}{
		{
			name:            "single resource no query",
			table:           "grant_request",
			wantResources:   []string{"grant_request"},
			wantCustomField: map[string][]string{},
		},
		{
			name:            "comma-separated resources",
			table:           "grant_request,user,organization",
			wantResources:   []string{"grant_request", "user", "organization"},
			wantCustomField: map[string][]string{},
		},
		{
			name:          "colon form legacy",
			table:         "grant_request:amount_requested,project_title",
			wantResources: []string{"grant_request"},
			wantCustomField: map[string][]string{
				"grant_request": {"amount_requested", "project_title"},
			},
		},
		{
			name:            "two-colon quirk via parseFluxxTableSpec",
			table:           "grant_request:field1:field2",
			wantResources:   []string{"grant_request:field1:field2"},
			wantCustomField: map[string][]string{},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseFluxxTableSpec(tt.table)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errSubstr != "" {
					assert.Contains(t, err.Error(), tt.errSubstr)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantResources, got.resources)
			assert.Equal(t, tt.wantCustomField, got.customFields)
		})
	}
}
