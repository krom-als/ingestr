package applovin

import (
	"context"
	"testing"
	"time"

	"github.com/bruin-data/ingestr/pkg/source"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAppLovinURI(t *testing.T) {
	tests := []struct {
		name      string
		uri       string
		wantKey   string
		wantError bool
	}{
		{
			name:    "valid URI with api_key",
			uri:     "applovin://?api_key=test_key_123",
			wantKey: "test_key_123",
		},
		{
			name:    "valid URI with extra params",
			uri:     "applovin://?api_key=my_key&other=value",
			wantKey: "my_key",
		},
		{
			name:      "missing api_key",
			uri:       "applovin://?other=value",
			wantError: true,
		},
		{
			name:      "empty api_key",
			uri:       "applovin://?api_key=",
			wantError: true,
		},
		{
			name:      "invalid URI",
			uri:       "://invalid",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := parseAppLovinURI(tt.uri)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantKey, key)
			}
		})
	}
}

func TestParseTimeInterval(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		start     interface{}
		end       interface{}
		wantError bool
	}{
		{
			name:  "valid time.Time values",
			start: now,
			end:   now.AddDate(0, 0, 7),
		},
		{
			name:  "valid *time.Time values",
			start: &now,
			end:   func() *time.Time { t := now.AddDate(0, 0, 7); return &t }(),
		},
		{
			name:      "nil start",
			start:     nil,
			end:       now,
			wantError: true,
		},
		{
			name:      "nil end",
			start:     now,
			end:       nil,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end, err := parseTimeInterval(tt.start, tt.end)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.False(t, start.IsZero())
				assert.False(t, end.IsZero())
			}
		})
	}
}

func TestParseTimestamp(t *testing.T) {
	now := time.Now()
	defaultTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		value    interface{}
		defVal   time.Time
		expected time.Time
	}{
		{
			name:     "time.Time value",
			value:    now,
			defVal:   defaultTime,
			expected: now,
		},
		{
			name:     "*time.Time value",
			value:    &now,
			defVal:   defaultTime,
			expected: now,
		},
		{
			name:     "nil returns default",
			value:    nil,
			defVal:   defaultTime,
			expected: defaultTime,
		},
		{
			name:     "nil pointer returns default",
			value:    (*time.Time)(nil),
			defVal:   defaultTime,
			expected: defaultTime,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseTimestamp(tt.value, tt.defVal)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExcludeColumns(t *testing.T) {
	tests := []struct {
		name     string
		columns  []string
		exclude  map[string]bool
		expected []string
	}{
		{
			name:     "exclude some columns",
			columns:  []string{"a", "b", "c", "d"},
			exclude:  map[string]bool{"b": true, "d": true},
			expected: []string{"a", "c"},
		},
		{
			name:     "exclude none",
			columns:  []string{"a", "b", "c"},
			exclude:  map[string]bool{},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "exclude all",
			columns:  []string{"a", "b"},
			exclude:  map[string]bool{"a": true, "b": true},
			expected: []string{},
		},
		{
			name:     "exclude non-existent",
			columns:  []string{"a", "b"},
			exclude:  map[string]bool{"x": true, "y": true},
			expected: []string{"a", "b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := excludeColumns(tt.columns, tt.exclude)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetDimensionColumns(t *testing.T) {
	tests := []struct {
		name     string
		columns  []string
		expected []string
	}{
		{
			name:     "day in middle moves to first",
			columns:  []string{"country", "day", "platform"},
			expected: []string{"day", "country", "platform"},
		},
		{
			name:     "day already first",
			columns:  []string{"day", "country", "platform"},
			expected: []string{"day", "country", "platform"},
		},
		{
			name:     "no day column",
			columns:  []string{"country", "platform"},
			expected: []string{"country", "platform"},
		},
		{
			name:     "filters out non-dimensions",
			columns:  []string{"day", "clicks", "country", "impressions"},
			expected: []string{"day", "country"},
		},
		{
			name:     "empty columns",
			columns:  []string{},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getDimensionColumns(tt.columns)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCreateCustomReportTable(t *testing.T) {
	source := NewAppLovinSource()

	tests := []struct {
		name      string
		spec      string
		wantError bool
	}{
		{
			name:      "valid spec",
			spec:      "custom:report:publisher:day,country,clicks",
			wantError: false,
		},
		{
			name:      "valid advertiser spec",
			spec:      "custom:probabilisticReport:advertiser:day,campaign,impressions",
			wantError: false,
		},
		{
			name:      "invalid format - missing parts",
			spec:      "custom:report",
			wantError: true,
		},
		{
			name:      "invalid report_type",
			spec:      "custom:report:invalid_type:day,country",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			table, err := source.createCustomReportTable(tt.spec)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, table)
			}
		})
	}
}

func TestCreateCustomReportTableDetails(t *testing.T) {
	t.Parallel()

	s := NewAppLovinSource()

	t.Run("table name is always custom_report", func(t *testing.T) {
		t.Parallel()
		table, err := s.createCustomReportTable("custom:report:publisher:day,country")
		require.NoError(t, err)
		assert.Equal(t, "custom_report", table.Name())
	})

	t.Run("day auto-added when missing from dimensions", func(t *testing.T) {
		t.Parallel()
		// country is a dimension; clicks is a metric — getDimensionColumns filters to dimensions only.
		// day is not in the spec but must be appended so it appears in TablePrimaryKeys.
		table, err := s.createCustomReportTable("custom:report:publisher:country,clicks")
		require.NoError(t, err)
		pks := table.PrimaryKeys()
		assert.Contains(t, pks, "day", "day should be auto-added to primary keys when absent")
		assert.Contains(t, pks, "country")
	})

	t.Run("day not duplicated when already present", func(t *testing.T) {
		t.Parallel()
		table, err := s.createCustomReportTable("custom:report:publisher:day,country")
		require.NoError(t, err)
		pks := table.PrimaryKeys()
		dayCount := 0
		for _, pk := range pks {
			if pk == "day" {
				dayCount++
			}
		}
		assert.Equal(t, 1, dayCount, "day should appear exactly once in primary keys")
	})

	t.Run("metric-only columns excluded from primary keys", func(t *testing.T) {
		t.Parallel()
		// clicks, impressions, revenue are metrics (not in the dimensions map).
		table, err := s.createCustomReportTable("custom:report:publisher:day,country,clicks,impressions,revenue")
		require.NoError(t, err)
		pks := table.PrimaryKeys()
		assert.NotContains(t, pks, "clicks")
		assert.NotContains(t, pks, "impressions")
		assert.NotContains(t, pks, "revenue")
		assert.Contains(t, pks, "day")
		assert.Contains(t, pks, "country")
	})

	t.Run("whitespace trimmed from dimensions", func(t *testing.T) {
		t.Parallel()
		table, err := s.createCustomReportTable("custom:report:publisher: day , country , clicks ")
		require.NoError(t, err)
		pks := table.PrimaryKeys()
		assert.Contains(t, pks, "day")
		assert.Contains(t, pks, "country")
	})

	t.Run("too few parts returns error", func(t *testing.T) {
		t.Parallel()
		_, err := s.createCustomReportTable("custom:report:publisher")
		assert.Error(t, err)
	})

	t.Run("too many parts returns error", func(t *testing.T) {
		t.Parallel()
		_, err := s.createCustomReportTable("custom:report:publisher:day:extra")
		assert.Error(t, err)
	})

	t.Run("publisher report type accepted", func(t *testing.T) {
		t.Parallel()
		_, err := s.createCustomReportTable("custom:report:publisher:day,country")
		assert.NoError(t, err)
	})

	t.Run("advertiser report type accepted", func(t *testing.T) {
		t.Parallel()
		_, err := s.createCustomReportTable("custom:report:advertiser:day,campaign")
		assert.NoError(t, err)
	})

	t.Run("unknown report type rejected", func(t *testing.T) {
		t.Parallel()
		_, err := s.createCustomReportTable("custom:report:unknown:day,country")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid report_type")
	})
}

func TestGetTableNonCustomQueryPath(t *testing.T) {
	t.Parallel()

	s := NewAppLovinSource()
	s.tables = s.getTables()

	t.Run("non-custom path with query params returns clear error", func(t *testing.T) {
		t.Parallel()
		_, err := s.GetTable(context.Background(), source.TableRequest{
			Name: "publisher-report?endpoint=report&report_type=publisher&dimensions=day",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported table")
		assert.Contains(t, err.Error(), "publisher-report")
	})
}

func TestCreateCustomReportTableFromParams(t *testing.T) {
	t.Parallel()

	s := NewAppLovinSource()

	t.Run("query form basic", func(t *testing.T) {
		t.Parallel()
		table, err := s.GetTable(context.Background(), source.TableRequest{Name: "custom?endpoint=report&report_type=publisher&dimensions=day,country"})
		require.NoError(t, err)
		assert.Equal(t, "custom_report", table.Name())
	})

	t.Run("query form advertiser", func(t *testing.T) {
		t.Parallel()
		table, err := s.GetTable(context.Background(), source.TableRequest{Name: "custom?endpoint=probabilisticReport&report_type=advertiser&dimensions=day,campaign,impressions"})
		require.NoError(t, err)
		assert.Equal(t, "custom_report", table.Name())
	})

	t.Run("query form repeated dimensions key", func(t *testing.T) {
		t.Parallel()
		table, err := s.GetTable(context.Background(), source.TableRequest{Name: "custom?endpoint=report&report_type=publisher&dimensions=day&dimensions=country"})
		require.NoError(t, err)
		pks := table.PrimaryKeys()
		assert.Contains(t, pks, "day")
		assert.Contains(t, pks, "country")
	})

	t.Run("query form day auto-added", func(t *testing.T) {
		t.Parallel()
		table, err := s.GetTable(context.Background(), source.TableRequest{Name: "custom?endpoint=report&report_type=publisher&dimensions=country"})
		require.NoError(t, err)
		pks := table.PrimaryKeys()
		assert.Contains(t, pks, "day")
	})

	t.Run("query form invalid report_type", func(t *testing.T) {
		t.Parallel()
		_, err := s.GetTable(context.Background(), source.TableRequest{Name: "custom?endpoint=report&report_type=bad&dimensions=day,country"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid report_type")
	})

	t.Run("query form missing endpoint", func(t *testing.T) {
		t.Parallel()
		_, err := s.GetTable(context.Background(), source.TableRequest{Name: "custom?report_type=publisher&dimensions=day,country"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "endpoint")
	})

	t.Run("query form missing dimensions", func(t *testing.T) {
		t.Parallel()
		_, err := s.GetTable(context.Background(), source.TableRequest{Name: "custom?endpoint=report&report_type=publisher"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "dimension")
	})

	t.Run("query form unknown key rejected", func(t *testing.T) {
		t.Parallel()
		_, err := s.GetTable(context.Background(), source.TableRequest{Name: "custom?endpoint=report&report_type=publisher&dimensions=day&typo=x"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown table parameter")
	})

	t.Run("legacy colon form still works via GetTable", func(t *testing.T) {
		t.Parallel()
		table, err := s.GetTable(context.Background(), source.TableRequest{Name: "custom:report:publisher:day,country,clicks"})
		require.NoError(t, err)
		assert.Equal(t, "custom_report", table.Name())
	})

	t.Run("query form produces same primary keys as legacy form", func(t *testing.T) {
		t.Parallel()
		legacy, err := s.GetTable(context.Background(), source.TableRequest{Name: "custom:report:publisher:day,country"})
		require.NoError(t, err)
		query, err := s.GetTable(context.Background(), source.TableRequest{Name: "custom?endpoint=report&report_type=publisher&dimensions=day,country"})
		require.NoError(t, err)
		assert.Equal(t, legacy.PrimaryKeys(), query.PrimaryKeys())
		assert.Equal(t, legacy.Name(), query.Name())
	})
}
