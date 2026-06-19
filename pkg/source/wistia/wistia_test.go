package wistia

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/bruin-data/ingestr/pkg/source"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseWistiaURI(t *testing.T) {
	t.Run("query token", func(t *testing.T) {
		creds, err := parseWistiaURI("wistia://?access_token=abc&api_version=2026-03&base_url=https%3A%2F%2Fexample.com%2Fmodern%2F")
		require.NoError(t, err)
		assert.Equal(t, "abc", creds.accessToken)
		assert.Equal(t, "2026-03", creds.apiVersion)
		assert.Equal(t, "https://example.com/modern", creds.apiURL)
	})

	t.Run("api key alias", func(t *testing.T) {
		creds, err := parseWistiaURI("wistia://?api_key=abc")
		require.NoError(t, err)
		assert.Equal(t, "abc", creds.accessToken)
		assert.Equal(t, defaultAPIVersion, creds.apiVersion)
		assert.Equal(t, defaultBaseURL, creds.apiURL)
	})

	t.Run("bare token", func(t *testing.T) {
		creds, err := parseWistiaURI("wistia://abc123")
		require.NoError(t, err)
		assert.Equal(t, "abc123", creds.accessToken)
	})

	t.Run("missing token", func(t *testing.T) {
		_, err := parseWistiaURI("wistia://?api_version=2026-03")
		require.Error(t, err)
	})
}

func TestGetTable(t *testing.T) {
	src := NewWistiaSource()

	table, err := src.GetTable(context.Background(), source.TableRequest{Name: "stats_media_by_date:abc123"})
	require.NoError(t, err)
	assert.Equal(t, "stats_media_by_date:abc123", table.Name())
	assert.Equal(t, []string{"media_id", "date"}, table.PrimaryKeys())
	assert.Equal(t, "date", table.IncrementalKey())
	assert.Equal(t, "date", table.(source.PartitionedTable).PartitionBy())

	table, err = src.GetTable(context.Background(), source.TableRequest{Name: "captions"})
	require.NoError(t, err)
	assert.Equal(t, []string{"id"}, table.PrimaryKeys())

	_, err = src.GetTable(context.Background(), source.TableRequest{Name: "stats_media_by_date"})
	require.Error(t, err)

	_, err = src.GetTable(context.Background(), source.TableRequest{Name: "unknown"})
	require.Error(t, err)
}

func TestReadPaginated(t *testing.T) {
	var pages []int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer secret", r.Header.Get("Authorization"))
		assert.Equal(t, "2026-03", r.Header.Get(apiVersionHeader))
		assert.Equal(t, "/medias", r.URL.Path)
		assert.Equal(t, "2", r.URL.Query().Get("per_page"))

		page, err := strconv.Atoi(r.URL.Query().Get("page"))
		require.NoError(t, err)
		pages = append(pages, page)

		w.Header().Set("Content-Type", "application/json")
		switch page {
		case 1:
			require.NoError(t, json.NewEncoder(w).Encode([]map[string]interface{}{
				{"hashed_id": "a", "name": "A"},
				{"hashed_id": "b", "name": "B"},
			}))
		case 2:
			require.NoError(t, json.NewEncoder(w).Encode([]map[string]interface{}{
				{"hashed_id": "c", "name": "C"},
			}))
		default:
			require.NoError(t, json.NewEncoder(w).Encode([]map[string]interface{}{}))
		}
	}))
	defer server.Close()

	src := NewWistiaSource()
	err := src.Connect(context.Background(), "wistia://?access_token=secret&base_url="+url.QueryEscape(server.URL))
	require.NoError(t, err)
	defer func() { require.NoError(t, src.Close(context.Background())) }()

	table, err := src.GetTable(context.Background(), source.TableRequest{Name: "medias"})
	require.NoError(t, err)

	records, err := table.Read(context.Background(), source.ReadOptions{PageSize: 2})
	require.NoError(t, err)

	var rows int64
	for result := range records {
		require.NoError(t, result.Err)
		rows += result.Batch.NumRows()
		result.Batch.Release()
	}

	assert.Equal(t, int64(3), rows)
	assert.Equal(t, []int{1, 2}, pages)
}

func TestReadDateFilteredParameterizedTable(t *testing.T) {
	var gotQuery url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		assert.Equal(t, "/stats/medias/abc123/by_date", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode([]map[string]interface{}{
			{"date": "2026-01-02", "play_count": 3},
		}))
	}))
	defer server.Close()

	src := NewWistiaSource()
	err := src.Connect(context.Background(), "wistia://?access_token=secret&base_url="+url.QueryEscape(server.URL))
	require.NoError(t, err)
	defer func() { require.NoError(t, src.Close(context.Background())) }()

	table, err := src.GetTable(context.Background(), source.TableRequest{Name: "stats_media_by_date:abc123"})
	require.NoError(t, err)

	start := time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 3, 10, 0, 0, 0, time.UTC)
	records, err := table.Read(context.Background(), source.ReadOptions{
		IntervalStart: &start,
		IntervalEnd:   &end,
	})
	require.NoError(t, err)

	var batchCount int
	for result := range records {
		require.NoError(t, result.Err)
		batchCount++
		assert.Equal(t, int64(1), result.Batch.NumRows())
		indices := result.Batch.Schema().FieldIndices("media_id")
		require.NotEmpty(t, indices)
		result.Batch.Release()
	}

	assert.Equal(t, 1, batchCount)
	assert.Equal(t, "2026-01-02", gotQuery.Get("start_date"))
	assert.Equal(t, "2026-01-03", gotQuery.Get("end_date"))
}

func TestWistiaDateRange(t *testing.T) {
	defaultingCfg := tableConfigs["stats_account_by_date"]
	nonDefaultingCfg := tableConfigs["stats_media_by_date"]

	start, end := wistiaDateRange(defaultingCfg, source.ReadOptions{})
	require.NotEmpty(t, start)
	require.NotEmpty(t, end)

	startDate, err := time.Parse("2006-01-02", start)
	require.NoError(t, err)
	endDate, err := time.Parse("2006-01-02", end)
	require.NoError(t, err)
	assert.Equal(t, 24*time.Hour, endDate.Sub(startDate))

	start, end = wistiaDateRange(nonDefaultingCfg, source.ReadOptions{})
	assert.Empty(t, start)
	assert.Empty(t, end)

	onlyEnd := time.Date(2026, 1, 3, 10, 0, 0, 0, time.UTC)
	start, end = wistiaDateRange(nonDefaultingCfg, source.ReadOptions{IntervalEnd: &onlyEnd})
	assert.Equal(t, "2026-01-02", start)
	assert.Equal(t, "2026-01-03", end)

	onlyStart := time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC)
	start, end = wistiaDateRange(nonDefaultingCfg, source.ReadOptions{IntervalStart: &onlyStart})
	assert.Equal(t, "2026-01-02", start)
	require.NotEmpty(t, end)
}

func TestParseTableName(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input     string
		wantBase  string
		wantParam string
	}{
		{"medias", "medias", ""},
		{"captions", "captions", ""},
		{"captions:abc123", "captions", "abc123"},
		{"stats_media_by_date:xyz", "stats_media_by_date", "xyz"},
		{"folder:", "folder", ""},
		{"a:b:c", "a", "b:c"},
		{"", "", ""},
	}

	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			t.Parallel()
			base, param := parseTableName(c.input)
			assert.Equal(t, c.wantBase, base)
			assert.Equal(t, c.wantParam, param)
		})
	}
}

func TestResolveTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		// no-param tables
		{name: "medias no param", input: "medias", wantErr: false},
		{name: "account no param", input: "account", wantErr: false},
		// no-param table with unexpected param → error
		{name: "medias with param rejected", input: "medias:abc", wantErr: true},
		// requiresParam tables — param provided → ok
		{name: "folder with param", input: "folder:abc123", wantErr: false},
		{name: "media with param", input: "media:abc123", wantErr: false},
		// requiresParam tables — param missing → error
		{name: "folder without param", input: "folder", wantErr: true},
		{name: "stats_media_by_date without param", input: "stats_media_by_date", wantErr: true},
		// allowsParam tables — param present → ok
		{name: "captions with param", input: "captions:m123", wantErr: false},
		{name: "stats_events with param", input: "stats_events:m456", wantErr: false},
		// allowsParam tables — no param → ok
		{name: "captions without param", input: "captions", wantErr: false},
		{name: "stats_events without param", input: "stats_events", wantErr: false},
		// allowsParam table with empty param (bare colon) — treated as param="" which is allowed
		{name: "captions empty param", input: "captions:", wantErr: false},
		// unknown table → error
		{name: "unknown table", input: "nonexistent_table", wantErr: true},
		{name: "empty string", input: "", wantErr: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			_, got, err := resolveTable(c.input)
			if c.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.input, got)
		})
	}
}

// TestResolveTableQueryForm covers the URL-style ?id= form added by the
// tablespec migration. Legacy cases are in TestResolveTable above.
func TestResolveTableQueryForm(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		input    string
		wantNorm string // normalized (legacy-form) table name returned
		wantErr  bool
	}{
		// requiresParam tables with ?id=
		{name: "folder with query id", input: "folder?id=abc123", wantNorm: "folder:abc123"},
		{name: "media with query id", input: "media?id=xyz456", wantNorm: "media:xyz456"},
		{name: "stats_media_by_date with query id", input: "stats_media_by_date?id=m999", wantNorm: "stats_media_by_date:m999"},
		// requiresParam missing id → error
		{name: "folder query form no id", input: "folder?id=", wantErr: true},
		{name: "media query form no id", input: "media?id=", wantErr: true},
		// allowsParam tables with ?id= → ok
		{name: "captions with query id", input: "captions?id=m123", wantNorm: "captions:m123"},
		{name: "stats_events with query id", input: "stats_events?id=media_hash", wantNorm: "stats_events:media_hash"},
		// allowsParam tables without ?id= → ok (no id param in query, but hasQuery=false → legacy path, already covered)
		// no-param tables with ?id= → error
		{name: "medias rejects query id", input: "medias?id=abc", wantErr: true},
		{name: "account rejects query id", input: "account?id=foo", wantErr: true},
		// unknown table in query form → error
		{name: "unknown table query form", input: "nonexistent?id=x", wantErr: true},
		// unknown key → error
		{name: "unknown param key", input: "media?bogus=x", wantErr: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			_, got, err := resolveTable(c.input)
			if c.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.wantNorm, got)
		})
	}
}

// TestResolveTableQuirkBareColonAllowsParam verifies that the legacy quirk
// "captions:" (bare colon, empty param) continues to pass after the migration.
func TestResolveTableQuirkBareColonAllowsParam(t *testing.T) {
	t.Parallel()
	_, got, err := resolveTable("captions:")
	require.NoError(t, err)
	assert.Equal(t, "captions:", got)
}

func TestResponseItems(t *testing.T) {
	t.Run("rejects unexpected object for array response", func(t *testing.T) {
		items, err := responseItems([]byte(`{"pagination":{"next":2}}`), true)
		require.Error(t, err)
		assert.Nil(t, items)
		assert.Contains(t, err.Error(), "expected array response")
	})

	t.Run("keeps object for single item response", func(t *testing.T) {
		items, err := responseItems([]byte(`{"id":"abc123","name":"A"}`), false)
		require.NoError(t, err)
		require.Len(t, items, 1)
		assert.Equal(t, "abc123", items[0]["id"])
	})

	t.Run("unwraps recognized array envelope", func(t *testing.T) {
		items, err := responseItems([]byte(`{"data":[{"id":"a"},{"id":"b"}]}`), true)
		require.NoError(t, err)
		require.Len(t, items, 2)
		assert.Equal(t, "a", items[0]["id"])
		assert.Equal(t, "b", items[1]["id"])
	})
}
