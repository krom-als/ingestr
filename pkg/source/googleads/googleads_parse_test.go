package googleads

import (
	"context"
	"testing"

	"github.com/bruin-data/ingestr/pkg/source"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetTableDispatch validates how the table-name string is parsed to select
// a path (gaql_query, daily, builtin, builtin+customer_ids) and what primary
// keys / incremental key come out. No network calls are made — GetTable only
// builds the DynamicSourceTable metadata; it does not connect.
func TestGetTableDispatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		tableName          string
		wantIncrementalKey string
		wantPKsContain     []string
		wantErr            bool
		errSubstr          string
	}{
		{
			name:               "gaql_query prefix produces empty pks and no incremental key",
			tableName:          "gaql_query:SELECT campaign.id FROM campaign",
			wantIncrementalKey: "",
			wantPKsContain:     nil,
		},
		{
			name:               "builtin account_report_daily",
			tableName:          "account_report_daily",
			wantIncrementalKey: "segments_date",
			wantPKsContain:     []string{"customer_id"},
		},
		{
			name:               "builtin campaign_report_daily",
			tableName:          "campaign_report_daily",
			wantIncrementalKey: "segments_date",
			wantPKsContain:     []string{"customer_id", "campaign_resource_name"},
		},
		{
			name:               "builtin with colon customer_id suffix",
			tableName:          "campaign_report_daily:123,456",
			wantIncrementalKey: "segments_date",
			wantPKsContain:     []string{"customer_id"},
		},
		{
			name:               "unknown builtin with colon suffix is not an error from GetTable",
			tableName:          "not_a_report:123",
			wantIncrementalKey: "",
			wantPKsContain:     nil,
		},
		{
			name:               "unknown bare name is not an error from GetTable",
			tableName:          "totally_unknown",
			wantIncrementalKey: "",
			wantPKsContain:     nil,
		},
		{
			name:      "daily prefix with invalid spec returns error",
			tableName: "daily:only_one_colon_segment",
			wantErr:   true,
			errSubstr: "invalid daily report spec",
		},
		{
			name:               "daily prefix with valid spec",
			tableName:          "daily:campaign:campaign.id:metrics.clicks",
			wantIncrementalKey: "segments_date",
			wantPKsContain:     []string{"customer_id"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			src := NewGoogleAdsSource()
			tbl, err := src.GetTable(context.Background(), source.TableRequest{Name: tt.tableName})
			if tt.wantErr {
				require.Error(t, err)
				if tt.errSubstr != "" {
					assert.Contains(t, err.Error(), tt.errSubstr)
				}
				return
			}
			require.NoError(t, err)
			require.NotNil(t, tbl)
			assert.Equal(t, tt.wantIncrementalKey, tbl.IncrementalKey())
			for _, pk := range tt.wantPKsContain {
				assert.Contains(t, tbl.PrimaryKeys(), pk, "expected primary keys to contain %q", pk)
			}
		})
	}
}

func TestGetTableDispatch_GaqlQueryPreserved(t *testing.T) {
	t.Parallel()
	// The table name for gaql_query is stored verbatim (the whole "gaql_query:..." string).
	src := NewGoogleAdsSource()
	q := "gaql_query:SELECT campaign.id FROM campaign WHERE segments.date BETWEEN '2024-01-01' AND '2024-01-31'"
	tbl, err := src.GetTable(context.Background(), source.TableRequest{Name: q})
	require.NoError(t, err)
	assert.Equal(t, q, tbl.Name())
}

func TestGetTableDispatch_BuiltinColonStripsCustomerIDs(t *testing.T) {
	t.Parallel()
	// When a builtin name is followed by ":customer_ids", the table name stored
	// in DynamicSourceTable is the full original string (not stripped).
	src := NewGoogleAdsSource()
	full := "account_report_daily:111,222"
	tbl, err := src.GetTable(context.Background(), source.TableRequest{Name: full})
	require.NoError(t, err)
	assert.Equal(t, full, tbl.Name())
}

// TestGetTableDispatch_QueryForm exercises the new URL-style query forms.
func TestGetTableDispatch_QueryForm(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		tableName          string
		wantIncrementalKey string
		wantPKsContain     []string
		wantErr            bool
		errSubstr          string
	}{
		{
			name:               "daily query form basic",
			tableName:          "daily?resource=campaign&dimensions=campaign.id&metrics=metrics.clicks",
			wantIncrementalKey: "segments_date",
			wantPKsContain:     []string{"customer_id", "campaign_id"},
		},
		{
			name:               "daily query form with customer_ids",
			tableName:          "daily?resource=campaign&dimensions=campaign.id&metrics=metrics.clicks&customer_ids=111&customer_ids=222",
			wantIncrementalKey: "segments_date",
			wantPKsContain:     []string{"customer_id", "campaign_id"},
		},
		{
			name:               "daily query form customer_ids comma-joined",
			tableName:          "daily?resource=campaign&dimensions=campaign.id&metrics=metrics.clicks&customer_ids=111,222",
			wantIncrementalKey: "segments_date",
			wantPKsContain:     []string{"customer_id"},
		},
		{
			name:               "builtin query form with customer_ids",
			tableName:          "campaign_report_daily?customer_ids=1234567890",
			wantIncrementalKey: "segments_date",
			wantPKsContain:     []string{"customer_id", "campaign_resource_name"},
		},
		{
			name:               "builtin query form unknown builtin",
			tableName:          "not_a_report?customer_ids=123",
			wantIncrementalKey: "",
			wantPKsContain:     nil,
		},
		{
			name:      "daily query form missing resource",
			tableName: "daily?dimensions=campaign.id&metrics=metrics.clicks",
			wantErr:   true,
			errSubstr: "resource is required",
		},
		{
			name:      "daily query form missing dimensions",
			tableName: "daily?resource=campaign&metrics=metrics.clicks",
			wantErr:   true,
			errSubstr: "dimensions are required",
		},
		{
			name:      "daily query form missing metrics",
			tableName: "daily?resource=campaign&dimensions=campaign.id",
			wantErr:   true,
			errSubstr: "metrics are required",
		},
		{
			name:      "daily query form unknown param",
			tableName: "daily?resource=campaign&dimensions=campaign.id&metrics=metrics.clicks&typo=x",
			wantErr:   true,
			errSubstr: "unknown table parameter",
		},
		{
			name:      "builtin query form unknown param",
			tableName: "campaign_report_daily?typo=x",
			wantErr:   true,
			errSubstr: "unknown table parameter",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			src := NewGoogleAdsSource()
			tbl, err := src.GetTable(context.Background(), source.TableRequest{Name: tt.tableName})
			if tt.wantErr {
				require.Error(t, err)
				if tt.errSubstr != "" {
					assert.Contains(t, err.Error(), tt.errSubstr)
				}
				return
			}
			require.NoError(t, err)
			require.NotNil(t, tbl)
			assert.Equal(t, tt.wantIncrementalKey, tbl.IncrementalKey())
			for _, pk := range tt.wantPKsContain {
				assert.Contains(t, tbl.PrimaryKeys(), pk, "expected primary keys to contain %q", pk)
			}
		})
	}
}

// TestReportFromQueryParams_Equivalence asserts that the query-form parser
// produces a Report structurally identical to the legacy colon-form parser for
// representative inputs.
func TestReportFromQueryParams_Equivalence(t *testing.T) {
	t.Parallel()

	t.Run("daily form equivalence — no customer_ids", func(t *testing.T) {
		t.Parallel()
		// Legacy: daily:campaign:campaign.id,customer.id:metrics.clicks,metrics.impressions
		legacyReport, legacyCIDs, err := reportFromSpec("campaign:campaign.id,customer.id:metrics.clicks,metrics.impressions")
		require.NoError(t, err)

		// Query form: daily?resource=campaign&dimensions=campaign.id&dimensions=customer.id&metrics=metrics.clicks&metrics=metrics.impressions
		params := map[string][]string{
			"resource":   {"campaign"},
			"dimensions": {"campaign.id", "customer.id"},
			"metrics":    {"metrics.clicks", "metrics.impressions"},
		}
		queryReport, queryCIDs, err := reportFromQueryParams(params)
		require.NoError(t, err)

		assert.Equal(t, legacyReport.Resource, queryReport.Resource)
		assert.Equal(t, legacyReport.Dimensions, queryReport.Dimensions)
		assert.Equal(t, legacyReport.Metrics, queryReport.Metrics)
		assert.Equal(t, legacyReport.Segments, queryReport.Segments)
		assert.Equal(t, legacyCIDs, queryCIDs)
	})

	t.Run("daily form equivalence — with customer_ids", func(t *testing.T) {
		t.Parallel()
		// Legacy: daily:campaign:campaign.id:clicks:123,456
		legacyReport, legacyCIDs, err := reportFromSpec("campaign:campaign.id:clicks:123,456")
		require.NoError(t, err)

		// Query form: daily?resource=campaign&dimensions=campaign.id&metrics=clicks&customer_ids=123&customer_ids=456
		params := map[string][]string{
			"resource":     {"campaign"},
			"dimensions":   {"campaign.id"},
			"metrics":      {"clicks"},
			"customer_ids": {"123", "456"},
		}
		queryReport, queryCIDs, err := reportFromQueryParams(params)
		require.NoError(t, err)

		assert.Equal(t, legacyReport.Resource, queryReport.Resource)
		assert.Equal(t, legacyReport.Dimensions, queryReport.Dimensions)
		assert.Equal(t, legacyReport.Metrics, queryReport.Metrics)
		assert.Equal(t, legacyReport.Segments, queryReport.Segments)
		assert.Equal(t, legacyCIDs, queryCIDs)
	})

	t.Run("builtin customer_ids equivalence via GetTable", func(t *testing.T) {
		t.Parallel()
		src := NewGoogleAdsSource()

		// Legacy form
		legacyTbl, err := src.GetTable(context.Background(), source.TableRequest{Name: "campaign_report_daily:1234567890"})
		require.NoError(t, err)

		// Query form
		queryTbl, err := src.GetTable(context.Background(), source.TableRequest{Name: "campaign_report_daily?customer_ids=1234567890"})
		require.NoError(t, err)

		assert.Equal(t, legacyTbl.IncrementalKey(), queryTbl.IncrementalKey())
		assert.Equal(t, legacyTbl.PrimaryKeys(), queryTbl.PrimaryKeys())
	})
}

// TestReportFromQueryParams_DimensionCommaJoined tests that comma-joined
// dimension/metric values in a single repeated key are treated the same as
// repeated keys (list semantics).
func TestReportFromQueryParams_DimensionCommaJoined(t *testing.T) {
	t.Parallel()
	params := map[string][]string{
		"resource":   {"campaign"},
		"dimensions": {"campaign.id,customer.id"},
		"metrics":    {"metrics.clicks,metrics.impressions"},
	}
	report, _, err := reportFromQueryParams(params)
	require.NoError(t, err)
	assert.Equal(t, []string{"campaign.id", "customer.id"}, report.Dimensions)
	assert.Equal(t, []string{"metrics.clicks", "metrics.impressions"}, report.Metrics)
}

// TestGaqlQueryStaysLegacy confirms that gaql_query: inputs are never routed
// through the query-form parser, even if they happen to contain "?".
func TestGaqlQueryStaysLegacy(t *testing.T) {
	t.Parallel()
	src := NewGoogleAdsSource()
	// A GAQL query may contain "?" characters; this must not trigger the tablespec guard.
	q := "gaql_query:SELECT campaign.id FROM campaign WHERE segments.date = '2024-01-01'"
	tbl, err := src.GetTable(context.Background(), source.TableRequest{Name: q})
	require.NoError(t, err)
	assert.Equal(t, q, tbl.Name())
	assert.Equal(t, "", tbl.IncrementalKey())
}
