package google_analytics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildReportConfig_QueryForm_Custom verifies new URL-style query form for custom reports.
func TestBuildReportConfig_QueryForm_Custom(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		table       string
		wantType    string
		wantDims    []string
		wantMets    []string
		wantDateDim string
	}{
		{
			name:        "custom query form basic",
			table:       "custom?dimensions=date&dimensions=country&metrics=sessions&metrics=users",
			wantType:    "custom",
			wantDims:    []string{"date", "country"},
			wantMets:    []string{"sessions", "users"},
			wantDateDim: "date",
		},
		{
			name:        "custom query form comma-joined dimensions",
			table:       "custom?dimensions=date,country&metrics=sessions",
			wantType:    "custom",
			wantDims:    []string{"date", "country"},
			wantMets:    []string{"sessions"},
			wantDateDim: "date",
		},
		{
			name:        "custom dateHour dimension",
			table:       "custom?dimensions=dateHour&dimensions=city&metrics=sessions",
			wantType:    "custom",
			wantDims:    []string{"dateHour", "city"},
			wantMets:    []string{"sessions"},
			wantDateDim: "dateHour",
		},
		{
			name:        "custom multiple metrics via repeated keys",
			table:       "custom?dimensions=date&dimensions=sessionSource&dimensions=sessionMedium&metrics=sessions&metrics=totalUsers&metrics=newUsers",
			wantType:    "custom",
			wantDims:    []string{"date", "sessionSource", "sessionMedium"},
			wantMets:    []string{"sessions", "totalUsers", "newUsers"},
			wantDateDim: "date",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg, err := buildReportConfig(tt.table)
			require.NoError(t, err)
			assert.Equal(t, tt.wantType, cfg.reportType)
			assert.Equal(t, tt.wantDims, cfg.dimensions)
			assert.Equal(t, tt.wantMets, cfg.metrics)
			assert.Equal(t, tt.wantDateDim, cfg.datetime)
			assert.Nil(t, cfg.minuteRanges)
		})
	}
}

// TestBuildReportConfig_QueryForm_Realtime verifies new URL-style query form for realtime reports.
func TestBuildReportConfig_QueryForm_Realtime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		table         string
		wantDims      []string
		wantMets      []string
		wantRangeLen  int
		wantRangeData [][2]int64 // [StartMinutesAgo, EndMinutesAgo]
	}{
		{
			name:         "realtime minimal no ranges",
			table:        "realtime?dimensions=city&dimensions=country&metrics=activeUsers",
			wantDims:     []string{"city", "country"},
			wantMets:     []string{"activeUsers"},
			wantRangeLen: 0,
		},
		{
			name:          "realtime with minute ranges repeated keys",
			table:         "realtime?dimensions=city&metrics=activeUsers&minute_ranges=1-5&minute_ranges=6-10",
			wantDims:      []string{"city"},
			wantMets:      []string{"activeUsers"},
			wantRangeLen:  2,
			wantRangeData: [][2]int64{{5, 1}, {10, 6}},
		},
		{
			name:         "realtime comma-joined metrics",
			table:        "realtime?dimensions=city&metrics=activeUsers,screenPageViews",
			wantDims:     []string{"city"},
			wantMets:     []string{"activeUsers", "screenPageViews"},
			wantRangeLen: 0,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg, err := buildReportConfig(tt.table)
			require.NoError(t, err)
			assert.Equal(t, "realtime", cfg.reportType)
			assert.Equal(t, tt.wantDims, cfg.dimensions)
			assert.Equal(t, tt.wantMets, cfg.metrics)
			assert.Empty(t, cfg.datetime)
			require.Len(t, cfg.minuteRanges, tt.wantRangeLen)
			for i, rd := range tt.wantRangeData {
				assert.Equal(t, rd[0], cfg.minuteRanges[i].StartMinutesAgo)
				assert.Equal(t, rd[1], cfg.minuteRanges[i].EndMinutesAgo)
			}
		})
	}
}

// TestBuildReportConfig_QueryForm_Errors verifies error cases specific to the query form.
func TestBuildReportConfig_QueryForm_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		table     string
		errSubstr string
	}{
		{
			name:      "unknown param key",
			table:     "custom?dimensions=date&metrics=sessions&extra=foo",
			errSubstr: "unknown table parameter",
		},
		{
			name:      "invalid report type",
			table:     "unknown?dimensions=date&metrics=sessions",
			errSubstr: "invalid report type",
		},
		{
			name:      "missing dimensions",
			table:     "custom?metrics=sessions",
			errSubstr: "dimensions parameter is required",
		},
		{
			name:      "missing metrics",
			table:     "realtime?dimensions=city",
			errSubstr: "metrics parameter is required",
		},
		{
			name:      "custom missing datetime dimension",
			table:     "custom?dimensions=city&metrics=sessions",
			errSubstr: "custom reports must include at least one datetime dimension",
		},
		{
			name:      "minute_ranges on custom report",
			table:     "custom?dimensions=date&metrics=sessions&minute_ranges=1-5",
			errSubstr: "minute_ranges parameter is only valid for realtime reports",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := buildReportConfig(tt.table)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errSubstr)
		})
	}
}

// TestBuildReportConfig_Equivalence asserts that the query form and legacy colon form
// produce identical reportConfig structs for representative inputs.
func TestBuildReportConfig_Equivalence(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		legacy string
		query  string
	}{
		{
			name:   "custom basic",
			legacy: "custom:date,country:sessions,users",
			query:  "custom?dimensions=date&dimensions=country&metrics=sessions&metrics=users",
		},
		{
			name:   "custom dateHour dimension",
			legacy: "custom:dateHour,city:sessions",
			query:  "custom?dimensions=dateHour&dimensions=city&metrics=sessions",
		},
		{
			name:   "custom multiple metrics",
			legacy: "custom:date,sessionSource,sessionMedium:sessions,totalUsers,newUsers",
			query:  "custom?dimensions=date&dimensions=sessionSource&dimensions=sessionMedium&metrics=sessions&metrics=totalUsers&metrics=newUsers",
		},
		{
			name:   "realtime no ranges",
			legacy: "realtime:city,country:activeUsers",
			query:  "realtime?dimensions=city&dimensions=country&metrics=activeUsers",
		},
		{
			name:   "realtime multiple metrics",
			legacy: "realtime:city:activeUsers,screenPageViews",
			query:  "realtime?dimensions=city&metrics=activeUsers&metrics=screenPageViews",
		},
		{
			name:   "realtime with minute ranges",
			legacy: "realtime:city:activeUsers:1-5,6-10",
			query:  "realtime?dimensions=city&metrics=activeUsers&minute_ranges=1-5&minute_ranges=6-10",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			legacyCfg, err := buildReportConfig(tc.legacy)
			require.NoError(t, err, "legacy parse failed")

			queryCfg, err := buildReportConfig(tc.query)
			require.NoError(t, err, "query form parse failed")

			assert.Equal(t, legacyCfg.reportType, queryCfg.reportType, "reportType mismatch")
			assert.Equal(t, legacyCfg.dimensions, queryCfg.dimensions, "dimensions mismatch")
			assert.Equal(t, legacyCfg.metrics, queryCfg.metrics, "metrics mismatch")
			assert.Equal(t, legacyCfg.datetime, queryCfg.datetime, "datetime mismatch")
			require.Len(t, queryCfg.minuteRanges, len(legacyCfg.minuteRanges), "minuteRanges length mismatch")
			for i := range legacyCfg.minuteRanges {
				assert.Equal(t, legacyCfg.minuteRanges[i].StartMinutesAgo, queryCfg.minuteRanges[i].StartMinutesAgo, "minuteRange[%d].StartMinutesAgo mismatch", i)
				assert.Equal(t, legacyCfg.minuteRanges[i].EndMinutesAgo, queryCfg.minuteRanges[i].EndMinutesAgo, "minuteRange[%d].EndMinutesAgo mismatch", i)
				assert.Equal(t, legacyCfg.minuteRanges[i].Name, queryCfg.minuteRanges[i].Name, "minuteRange[%d].Name mismatch", i)
			}
		})
	}
}

func TestBuildReportConfig_ValidCustom(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		table       string
		wantType    string
		wantDims    []string
		wantMets    []string
		wantDateDim string
	}{
		{
			name:        "custom report with date dimension",
			table:       "custom:date,country:sessions,users",
			wantType:    "custom",
			wantDims:    []string{"date", "country"},
			wantMets:    []string{"sessions", "users"},
			wantDateDim: "date",
		},
		{
			name:        "custom report dateHour dimension",
			table:       "custom:dateHour,city:sessions",
			wantType:    "custom",
			wantDims:    []string{"dateHour", "city"},
			wantMets:    []string{"sessions"},
			wantDateDim: "dateHour",
		},
		{
			name:        "custom report dateHourMinute dimension",
			table:       "custom:dateHourMinute,region:pageViews",
			wantType:    "custom",
			wantDims:    []string{"dateHourMinute", "region"},
			wantMets:    []string{"pageViews"},
			wantDateDim: "dateHourMinute",
		},
		{
			name:        "custom report multiple metrics",
			table:       "custom:date,sessionSource,sessionMedium:sessions,totalUsers,newUsers",
			wantType:    "custom",
			wantDims:    []string{"date", "sessionSource", "sessionMedium"},
			wantMets:    []string{"sessions", "totalUsers", "newUsers"},
			wantDateDim: "date",
		},
		{
			name:        "custom report spaces around dimensions",
			table:       "custom: date , country : sessions , users",
			wantType:    "custom",
			wantDims:    []string{"date", "country"},
			wantMets:    []string{"sessions", "users"},
			wantDateDim: "date",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg, err := buildReportConfig(tt.table)
			require.NoError(t, err)
			assert.Equal(t, tt.wantType, cfg.reportType)
			assert.Equal(t, tt.wantDims, cfg.dimensions)
			assert.Equal(t, tt.wantMets, cfg.metrics)
			assert.Equal(t, tt.wantDateDim, cfg.datetime)
			assert.Nil(t, cfg.minuteRanges)
		})
	}
}

func TestBuildReportConfig_ValidRealtime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		table    string
		wantDims []string
		wantMets []string
	}{
		{
			name:     "realtime minimal",
			table:    "realtime:city,country:activeUsers",
			wantDims: []string{"city", "country"},
			wantMets: []string{"activeUsers"},
		},
		{
			name:     "realtime multiple metrics",
			table:    "realtime:city:activeUsers,screenPageViews",
			wantDims: []string{"city"},
			wantMets: []string{"activeUsers", "screenPageViews"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg, err := buildReportConfig(tt.table)
			require.NoError(t, err)
			assert.Equal(t, "realtime", cfg.reportType)
			assert.Equal(t, tt.wantDims, cfg.dimensions)
			assert.Equal(t, tt.wantMets, cfg.metrics)
			// realtime reports do not require a datetime dimension
			assert.Empty(t, cfg.datetime)
		})
	}
}

func TestBuildReportConfig_WithMinuteRanges(t *testing.T) {
	t.Parallel()

	cfg, err := buildReportConfig("realtime:city:activeUsers:1-5,6-10")
	require.NoError(t, err)
	assert.Equal(t, "realtime", cfg.reportType)
	require.Len(t, cfg.minuteRanges, 2)
	assert.Equal(t, int64(5), cfg.minuteRanges[0].StartMinutesAgo)
	assert.Equal(t, int64(1), cfg.minuteRanges[0].EndMinutesAgo)
	assert.Equal(t, int64(10), cfg.minuteRanges[1].StartMinutesAgo)
	assert.Equal(t, int64(6), cfg.minuteRanges[1].EndMinutesAgo)
}

func TestBuildReportConfig_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		table     string
		errSubstr string
	}{
		{
			name:      "too few parts",
			table:     "custom:date",
			errSubstr: "invalid table format",
		},
		{
			name:      "too many parts",
			table:     "custom:date:sessions:1-5:extra",
			errSubstr: "invalid table format",
		},
		{
			name:      "invalid report type",
			table:     "unknown:date:sessions",
			errSubstr: "invalid report type",
		},
		{
			name:      "custom without datetime dimension",
			table:     "custom:country,city:sessions",
			errSubstr: "custom reports must include at least one datetime dimension",
		},
		{
			name:      "empty table string",
			table:     "",
			errSubstr: "invalid table format",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := buildReportConfig(tt.table)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errSubstr)
		})
	}
}

func TestBuildMinuteRanges(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		raw        string
		wantLen    int
		wantNames  []string
		wantStarts []int64
		wantEnds   []int64
		wantErr    bool
		errSubstr  string
	}{
		{
			name:       "single range",
			raw:        "1-5",
			wantLen:    1,
			wantNames:  []string{"5_to_1_minutes_ago"},
			wantStarts: []int64{5},
			wantEnds:   []int64{1},
		},
		{
			name:       "two ranges",
			raw:        "1-5,6-10",
			wantLen:    2,
			wantNames:  []string{"5_to_1_minutes_ago", "10_to_6_minutes_ago"},
			wantStarts: []int64{5, 10},
			wantEnds:   []int64{1, 6},
		},
		{
			name:       "spaces stripped",
			raw:        " 1-5 , 6-10 ",
			wantLen:    2,
			wantStarts: []int64{5, 10},
			wantEnds:   []int64{1, 6},
		},
		{
			name:      "no hyphen separator",
			raw:       "15",
			wantErr:   true,
			errSubstr: "start-end format",
		},
		{
			name:      "non-numeric start",
			raw:       "abc-5",
			wantErr:   true,
			errSubstr: "values must be numeric",
		},
		{
			name:      "non-numeric end",
			raw:       "1-abc",
			wantErr:   true,
			errSubstr: "values must be numeric",
		},
		{
			name:      "empty string",
			raw:       "",
			wantErr:   true,
			errSubstr: "start-end format",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := buildMinuteRanges(tt.raw)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errSubstr != "" {
					assert.Contains(t, err.Error(), tt.errSubstr)
				}
				return
			}
			require.NoError(t, err)
			require.Len(t, got, tt.wantLen)
			for i := range got {
				assert.Equal(t, tt.wantStarts[i], got[i].StartMinutesAgo)
				assert.Equal(t, tt.wantEnds[i], got[i].EndMinutesAgo)
				if tt.wantNames != nil {
					assert.Equal(t, tt.wantNames[i], got[i].Name)
				}
			}
		})
	}
}
