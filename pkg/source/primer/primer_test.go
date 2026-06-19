package primer

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sortedStatuses returns a sorted copy of ss for order-independent comparison.
func sortedStatuses(ss []string) []string {
	out := make([]string, len(ss))
	copy(out, ss)
	sort.Strings(out)
	return out
}

func TestPrimerParseURI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		uri        string
		wantAPIKey string
		wantErr    bool
		errSubstr  string
	}{
		{
			name:       "valid URI",
			uri:        "primer://?api_key=test-key-123",
			wantAPIKey: "test-key-123",
		},
		{
			name:       "valid URI with extra params",
			uri:        "primer://?api_key=abc&other=ignored",
			wantAPIKey: "abc",
		},
		{
			name:      "wrong scheme",
			uri:       "https://?api_key=mykey",
			wantErr:   true,
			errSubstr: "must start with primer://",
		},
		{
			name:      "missing api_key",
			uri:       "primer://?other=value",
			wantErr:   true,
			errSubstr: "api_key is required",
		},
		{
			name:      "empty after scheme",
			uri:       "primer://",
			wantErr:   true,
			errSubstr: "api_key is required",
		},
		{
			name:      "only question mark",
			uri:       "primer://?",
			wantErr:   true,
			errSubstr: "api_key is required",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parsePrimerURI(tt.uri)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errSubstr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantAPIKey, got)
		})
	}
}

func TestPrimerParseTableName(t *testing.T) {
	t.Parallel()

	allStatuses := allStatuses()

	tests := []struct {
		name         string
		table        string
		wantTable    string
		wantStatuses []string
		wantErr      bool
		errSubstr    string
	}{
		{
			name:         "bare payments returns all statuses",
			table:        "payments",
			wantTable:    "payments",
			wantStatuses: allStatuses,
		},
		{
			name:         "payments with single status",
			table:        "payments:SETTLED",
			wantTable:    "payments",
			wantStatuses: []string{"SETTLED"},
		},
		{
			name:         "payments with multiple statuses",
			table:        "payments:SETTLED,FAILED",
			wantTable:    "payments",
			wantStatuses: []string{"SETTLED", "FAILED"},
		},
		{
			name:         "status is normalised to uppercase",
			table:        "payments:settled",
			wantTable:    "payments",
			wantStatuses: []string{"SETTLED"},
		},
		{
			name:         "mixed case normalised",
			table:        "payments:Settled,FAILED",
			wantTable:    "payments",
			wantStatuses: []string{"SETTLED", "FAILED"},
		},
		{
			name:         "whitespace around statuses trimmed",
			table:        "payments:SETTLED , FAILED",
			wantTable:    "payments",
			wantStatuses: []string{"SETTLED", "FAILED"},
		},
		{
			name:         "duplicate statuses de-duplicated",
			table:        "payments:SETTLED,SETTLED",
			wantTable:    "payments",
			wantStatuses: []string{"SETTLED"},
		},
		{
			name:      "invalid status",
			table:     "payments:BADSTATUS",
			wantErr:   true,
			errSubstr: "invalid payment status",
		},
		{
			name:      "empty statuses after colon",
			table:     "payments:",
			wantErr:   true,
			errSubstr: "no payment status provided",
		},
		{
			name:      "only commas after colon",
			table:     "payments:,",
			wantErr:   true,
			errSubstr: "no payment status provided",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tbl, statuses, err := parseTableName(tt.table)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errSubstr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantTable, tbl)

			got := make([]string, len(statuses))
			copy(got, statuses)
			sort.Strings(got)

			want := make([]string, len(tt.wantStatuses))
			copy(want, tt.wantStatuses)
			sort.Strings(want)

			assert.Equal(t, want, got)
		})
	}
}

func TestParseTableNameQueryForm(t *testing.T) {
	t.Parallel()

	allSt := sortedStatuses(allStatuses())

	tests := []struct {
		name         string
		table        string
		wantTable    string
		wantStatuses []string
		wantErr      bool
		errSubstr    string
	}{
		{
			name:         "single status via query param",
			table:        "payments?statuses=SETTLED",
			wantTable:    "payments",
			wantStatuses: []string{"SETTLED"},
		},
		{
			name:         "multiple statuses via repeated key",
			table:        "payments?statuses=SETTLED&statuses=AUTHORIZED",
			wantTable:    "payments",
			wantStatuses: []string{"SETTLED", "AUTHORIZED"},
		},
		{
			name:         "multiple statuses via comma-joined value",
			table:        "payments?statuses=SETTLED,AUTHORIZED",
			wantTable:    "payments",
			wantStatuses: []string{"SETTLED", "AUTHORIZED"},
		},
		{
			name:         "mixed repeated and comma-joined",
			table:        "payments?statuses=SETTLED,FAILED&statuses=PENDING",
			wantTable:    "payments",
			wantStatuses: []string{"SETTLED", "FAILED", "PENDING"},
		},
		{
			name:         "statuses normalised to uppercase",
			table:        "payments?statuses=settled",
			wantTable:    "payments",
			wantStatuses: []string{"SETTLED"},
		},
		{
			name:         "duplicate statuses de-duplicated",
			table:        "payments?statuses=SETTLED&statuses=SETTLED",
			wantTable:    "payments",
			wantStatuses: []string{"SETTLED"},
		},
		{
			name:         "all statuses via query param",
			table:        "payments?statuses=SETTLED,AUTHORIZED,DECLINED,FAILED,PARTIALLY_SETTLED,PENDING,SETTLING,CANCELLED",
			wantTable:    "payments",
			wantStatuses: allSt,
		},
		{
			name:      "invalid status via query param",
			table:     "payments?statuses=BADSTATUS",
			wantErr:   true,
			errSubstr: "invalid payment status",
		},
		{
			name:      "unknown query parameter rejected",
			table:     "payments?status=SETTLED",
			wantErr:   true,
			errSubstr: "unknown table parameter",
		},
		{
			name:      "statuses only valid for payments table",
			table:     "orders?statuses=SETTLED",
			wantErr:   true,
			errSubstr: "statuses parameter is only valid for the payments table",
		},
		{
			name:      "non-payments table query form without statuses rejected early",
			table:     "orders?statuses=",
			wantErr:   true,
			errSubstr: "statuses parameter is only valid for the payments table",
		},
		{
			name:      "unknown table query form rejected early",
			table:     "refunds?statuses=",
			wantErr:   true,
			errSubstr: "statuses parameter is only valid for the payments table",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tbl, statuses, err := parseTableName(tt.table)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errSubstr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantTable, tbl)
			assert.Equal(t, sortedStatuses(tt.wantStatuses), sortedStatuses(statuses))
		})
	}
}
