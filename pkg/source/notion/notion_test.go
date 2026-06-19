package notion

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseNotionTableSpec(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		wantID      string
		wantAll     bool
		wantErr     bool
		errContains string
	}{
		// Legacy forms — must be preserved byte-for-byte.
		{
			name:    "legacy star wildcard",
			input:   "*",
			wantID:  "",
			wantAll: true,
		},
		{
			name:    "legacy UUID",
			input:   "abc123ef-dead-beef-cafe-000000000000",
			wantID:  "abc123ef-dead-beef-cafe-000000000000",
			wantAll: false,
		},
		{
			name:    "legacy plain id no hyphens",
			input:   "abc123",
			wantID:  "abc123",
			wantAll: false,
		},
		// New URL-style form.
		{
			name:    "all=true triggers discover",
			input:   "?all=true",
			wantID:  "",
			wantAll: true,
		},
		{
			name:    "all=1 triggers discover",
			input:   "?all=1",
			wantID:  "",
			wantAll: true,
		},
		{
			name:    "all=false keeps path as database id",
			input:   "abc123?all=false",
			wantID:  "abc123",
			wantAll: false,
		},
		{
			name:    "all=0 keeps path as database id",
			input:   "abc123?all=0",
			wantID:  "abc123",
			wantAll: false,
		},
		{
			name:    "all key present but empty value treated as false",
			input:   "abc123?all=",
			wantID:  "abc123",
			wantAll: false,
		},
		// Error cases.
		{
			name:        "unknown param rejected",
			input:       "abc123?database=xyz",
			wantErr:     true,
			errContains: "unknown table parameter",
		},
		{
			name:        "invalid all value rejected",
			input:       "?all=yes",
			wantErr:     true,
			errContains: "invalid all parameter",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotID, gotAll, err := parseNotionTableSpec(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantID, gotID)
			assert.Equal(t, tt.wantAll, gotAll)
		})
	}
}

func TestParseNotionURI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		uri     string
		want    string
		wantErr bool
	}{
		{
			name: "valid URI with api_key",
			uri:  "notion://?api_key=secret_abc123",
			want: "secret_abc123",
		},
		{
			name: "valid URI with api_key containing special chars",
			uri:  "notion://?api_key=secret_ABC-xyz_123",
			want: "secret_ABC-xyz_123",
		},
		{
			name: "valid URI with api_key and extra params ignored",
			uri:  "notion://?api_key=mykey&other=ignored",
			want: "mykey",
		},
		{
			name:    "wrong scheme",
			uri:     "postgres://?api_key=secret",
			wantErr: true,
		},
		{
			name:    "missing api_key param",
			uri:     "notion://?other=value",
			wantErr: true,
		},
		{
			name:    "empty api_key value",
			uri:     "notion://?api_key=",
			wantErr: true,
		},
		{
			name:    "bare notion:// with no query string",
			uri:     "notion://",
			wantErr: true,
		},
		{
			name:    "notion:// with only question mark",
			uri:     "notion://?",
			wantErr: true,
		},
		{
			name:    "empty string",
			uri:     "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseNotionURI(tt.uri)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
