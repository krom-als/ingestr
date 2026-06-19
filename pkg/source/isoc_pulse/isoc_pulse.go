package isoc_pulse

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/bruin-data/ingestr/internal/config"
	"github.com/bruin-data/ingestr/pkg/arrowconv"
	httpclient "github.com/bruin-data/ingestr/pkg/http"
	"github.com/bruin-data/ingestr/pkg/schema"
	"github.com/bruin-data/ingestr/pkg/source"
	"github.com/bruin-data/ingestr/pkg/tablespec"
)

const (
	baseURL        = "https://pulse.internetsociety.org/api"
	rateLimit      = 2.0
	rateLimitBurst = 5
)

var metrics = map[string]string{
	"dnssec_adoption":     "dnssec/adoption",
	"dnssec_tld_adoption": "dnssec/adoption",
	"dnssec_validation":   "dnssec/validation",
	"http":                "http",
	"http3":               "http3",
	"https":               "https",
	"ipv6":                "ipv6",
	"net_loss":            "net-loss",
	"resilience":          "resilience",
	"roa":                 "roa",
	"rov":                 "rov",
	"tls":                 "tls",
	"tls13":               "tls13",
}

var noOptionMetrics = map[string]bool{
	"http":  true,
	"http3": true,
	"tls":   true,
	"tls13": true,
	"rov":   true,
}

type IsocPulseSource struct {
	token  string
	client *httpclient.Client
}

func NewIsocPulseSource() *IsocPulseSource {
	return &IsocPulseSource{}
}

func (s *IsocPulseSource) HandlesIncrementality() bool {
	return true
}

func (s *IsocPulseSource) Schemes() []string {
	return []string{"isoc-pulse"}
}

func (s *IsocPulseSource) Connect(ctx context.Context, uri string) error {
	token, err := parseURI(uri)
	if err != nil {
		return err
	}
	s.token = token

	s.client = httpclient.New(
		httpclient.WithBaseURL(baseURL),
		httpclient.WithTimeout(60*time.Second),
		httpclient.WithRateLimiter(rateLimit, rateLimitBurst),
		httpclient.WithDebug(config.DebugMode),
		httpclient.WithHeader("Authorization", "Bearer "+s.token),
	)
	config.Debug("[ISOC-PULSE] Connected successfully")
	return nil
}

func (s *IsocPulseSource) Close(ctx context.Context) error {
	if s.client != nil {
		return s.client.Close()
	}
	return nil
}

func parseURI(uri string) (string, error) {
	if !strings.HasPrefix(uri, "isoc-pulse://") {
		return "", fmt.Errorf("invalid isoc-pulse URI: must start with isoc-pulse://")
	}

	rest := strings.TrimPrefix(uri, "isoc-pulse://")
	if rest == "" || rest == "?" {
		return "", fmt.Errorf("token is required in isoc-pulse URI")
	}

	rest = strings.TrimPrefix(rest, "?")

	values, err := url.ParseQuery(rest)
	if err != nil {
		return "", fmt.Errorf("failed to parse isoc-pulse URI query: %w", err)
	}

	token := values.Get("token")
	if token == "" {
		return "", fmt.Errorf("token is required in isoc-pulse URI")
	}

	return token, nil
}

func (s *IsocPulseSource) GetTable(ctx context.Context, req source.TableRequest) (source.SourceTable, error) {
	metric, opts, err := parseTableName(req.Name)
	if err != nil {
		return nil, err
	}

	if !isValidTable(metric) {
		supportedList := make([]string, 0, len(metrics))
		for k := range metrics {
			supportedList = append(supportedList, k)
		}
		return nil, fmt.Errorf("unsupported table: %s (supported: %s)", req.Name, strings.Join(supportedList, ", "))
	}

	if err := validateOptions(metric, opts); err != nil {
		return nil, err
	}

	return &source.DynamicSourceTable{
		TableName:           metric,
		TablePrimaryKeys:    []string{"date"},
		TableIncrementalKey: "date",
		TableStrategy:       config.StrategyMerge,
		KnownSchema:         false,
		SchemaFn: func(ctx context.Context) (*schema.TableSchema, error) {
			return nil, fmt.Errorf("isoc-pulse source does not have a predefined schema; schema inference is required")
		},
		ReadFn: func(ctx context.Context, readOpts source.ReadOptions) (<-chan source.RecordBatchResult, error) {
			return s.read(ctx, metric, opts, readOpts)
		},
	}, nil
}

var isocPulseParamKeys = []string{"country", "topsites", "shutdown_type", "ip_version"}

func parseTableName(name string) (string, []string, error) {
	if name == "" {
		return "", nil, fmt.Errorf("table name is required")
	}

	path, params, hasQuery, err := tablespec.Split(name)
	if err != nil {
		return "", nil, err
	}

	if hasQuery {
		if err := tablespec.ValidateKeys(params, isocPulseParamKeys...); err != nil {
			return "", nil, err
		}
		metric := strings.TrimSpace(path)
		opts, err := buildOptsFromParams(metric, params)
		if err != nil {
			return "", nil, err
		}
		return metric, opts, nil
	}

	// Legacy colon form, preserved exactly.
	parts := strings.Split(name, ":")
	metric := parts[0]
	var opts []string
	if len(parts) > 1 {
		opts = parts[1:]
	}
	return metric, opts, nil
}

// buildOptsFromParams converts query parameters to the positional opts slice that
// buildMetricConfig expects. Each metric has its own positional contract; the
// named params map cleanly onto that contract:
//
//   - net_loss:  opts[0]=shutdown_type  opts[1]=country  (both required)
//   - roa:       opts[0]=ip_version  [opts[1]=country]
//   - https/ipv6: ["topsites"] if topsites=true, then [country] if present
//   - dnssec_*:  [country]
//   - resilience: [country]
//   - no-option metrics: no params accepted (validated by validateOptions)
func buildOptsFromParams(metric string, params url.Values) ([]string, error) {
	country := strings.TrimSpace(params.Get("country"))
	topsites := strings.TrimSpace(params.Get("topsites"))
	shutdownType := strings.TrimSpace(params.Get("shutdown_type"))
	ipVersion := strings.TrimSpace(params.Get("ip_version"))

	switch metric {
	case "net_loss":
		if shutdownType == "" || country == "" {
			return nil, fmt.Errorf("metric 'net_loss' requires both 'shutdown_type' and 'country' parameters to be non-empty")
		}
		return []string{shutdownType, country}, nil

	case "roa":
		if ipVersion != "" && country != "" {
			return []string{ipVersion, country}, nil
		}
		if ipVersion != "" {
			return []string{ipVersion}, nil
		}
		if country != "" {
			// Legacy-consistent: roa:US (country only) maps country into the ip_version slot.
			return []string{country}, nil
		}
		return nil, nil

	case "https", "ipv6":
		var opts []string
		if topsites != "" {
			ts, err := strconv.ParseBool(topsites)
			if err != nil {
				return nil, fmt.Errorf("invalid value for 'topsites': %q (expected true/false)", topsites)
			}
			if ts {
				opts = append(opts, "topsites")
			}
		}
		if country != "" {
			opts = append(opts, country)
		}
		return opts, nil

	case "dnssec_validation", "dnssec_tld_adoption", "dnssec_adoption":
		if country != "" {
			return []string{country}, nil
		}
		return nil, nil

	case "resilience":
		if country != "" {
			return []string{country}, nil
		}
		return nil, nil

	default:
		// no-option metrics: reject any supplied named param.
		if country != "" || topsites != "" || shutdownType != "" || ipVersion != "" {
			return nil, fmt.Errorf("metric %q does not accept any parameters", metric)
		}
		return nil, nil
	}
}

func isValidTable(metric string) bool {
	_, ok := metrics[metric]
	return ok
}

func validateOptions(metric string, opts []string) error {
	if len(opts) > 0 && noOptionMetrics[metric] {
		return fmt.Errorf("metric '%s' does not support options", metric)
	}
	if metric == "net_loss" && len(opts) != 2 {
		return fmt.Errorf("for 'net_loss' metric, two options are required: shutdown_type and country (e.g., net_loss:shutdown:US)")
	}
	return nil
}

type metricConfig struct {
	path   string
	params map[string]string
}

func buildMetricConfig(metric string, opts []string, startDate *time.Time) metricConfig {
	basePath := metrics[metric]
	params := make(map[string]string)

	if len(opts) == 0 {
		return metricConfig{path: basePath, params: params}
	}

	switch metric {
	case "https":
		hasTopsites := slices.Contains(opts, "topsites")
		if hasTopsites {
			params["topsites"] = "true"
		} else {
			params["topsites"] = "false"
		}
		country := lastNonKeyword(opts, "topsites")
		if country != "" {
			return metricConfig{path: basePath + "/country/" + country, params: params}
		}

	case "dnssec_validation", "dnssec_tld_adoption":
		return metricConfig{path: basePath + "/country/" + opts[len(opts)-1], params: params}

	case "dnssec_adoption":
		return metricConfig{path: basePath + "/domains/" + opts[len(opts)-1], params: params}

	case "ipv6":
		if slices.Contains(opts, "topsites") {
			params["topsites"] = "true"
		}
		country := lastNonKeyword(opts, "topsites")
		if country != "" {
			return metricConfig{path: basePath + "/country/" + country, params: params}
		}

	case "roa":
		if len(opts) > 1 {
			params["ip_version"] = opts[0]
			return metricConfig{path: basePath + "/country/" + opts[len(opts)-1], params: params}
		}
		params["ip_version"] = opts[0]

	case "net_loss":
		params["shutdown_type"] = opts[0]
		params["country"] = opts[1]

	case "resilience":
		params["country"] = opts[len(opts)-1]
		if startDate != nil {
			params["year"] = fmt.Sprintf("%d", startDate.Year())
			params["quarter"] = fmt.Sprintf("%d", int(math.Floor(float64(startDate.Month())/4))+1)
		}
	}

	return metricConfig{path: basePath, params: params}
}

func lastNonKeyword(opts []string, keywords ...string) string {
	for i := len(opts) - 1; i >= 0; i-- {
		if !slices.Contains(keywords, opts[i]) {
			return opts[i]
		}
	}
	return ""
}

func (s *IsocPulseSource) read(ctx context.Context, metric string, opts []string, readOpts source.ReadOptions) (<-chan source.RecordBatchResult, error) {
	results := make(chan source.RecordBatchResult, 8)

	go func() {
		defer close(results)

		err := s.fetchMetric(ctx, metric, opts, readOpts, results)
		if err != nil {
			results <- source.RecordBatchResult{Err: err}
		}
	}()

	return results, nil
}

func (s *IsocPulseSource) fetchMetric(ctx context.Context, metric string, opts []string, readOpts source.ReadOptions, results chan<- source.RecordBatchResult) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if readOpts.IntervalStart == nil {
		defaultStart := time.Now().AddDate(0, 0, -30)
		readOpts.IntervalStart = &defaultStart
	}

	cfg := buildMetricConfig(metric, opts, readOpts.IntervalStart)

	config.Debug("[ISOC-PULSE] Fetching metric=%s path=%s", metric, cfg.path)

	req := s.client.R(ctx)

	for k, v := range cfg.params {
		req.SetQueryParam(k, v)
	}

	req.SetQueryParam("start_date", readOpts.IntervalStart.Format("2006-01-02"))
	if readOpts.IntervalEnd != nil {
		req.SetQueryParam("end_date", readOpts.IntervalEnd.Format("2006-01-02"))
	}

	resp, err := req.Get("/" + cfg.path)
	if err != nil {
		return fmt.Errorf("failed to fetch %s: %w", metric, err)
	}

	if !resp.IsSuccess() {
		return fmt.Errorf("isoc-pulse API /%s returned status %d: %s", cfg.path, resp.StatusCode(), resp.String())
	}

	var items []map[string]any

	decoder := json.NewDecoder(strings.NewReader(string(resp.Body())))
	decoder.UseNumber()

	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := decoder.Decode(&envelope); err == nil && len(envelope.Data) > 0 {
		dec := json.NewDecoder(strings.NewReader(string(envelope.Data)))
		dec.UseNumber()
		if err := dec.Decode(&items); err != nil {
			return fmt.Errorf("failed to parse %s data array: %w", metric, err)
		}
	} else {
		decoder2 := json.NewDecoder(strings.NewReader(string(resp.Body())))
		decoder2.UseNumber()
		if err := decoder2.Decode(&items); err != nil {
			var single map[string]any
			decoder3 := json.NewDecoder(strings.NewReader(string(resp.Body())))
			decoder3.UseNumber()
			if err2 := decoder3.Decode(&single); err2 != nil {
				return fmt.Errorf("failed to parse %s response: %w", metric, err)
			}
			items = []map[string]any{single}
		}
	}

	if metric == "net_loss" {
		for i := range items {
			if readOpts.IntervalStart != nil {
				items[i]["date"] = readOpts.IntervalStart.Format("2006-01-02")
			}
		}
	}

	if len(items) == 0 {
		config.Debug("[ISOC-PULSE] No data returned for %s", metric)
		return nil
	}

	record, err := arrowconv.ItemsToArrowRecordWithSchema(items, nil, readOpts.ExcludeColumns)
	if err != nil {
		return fmt.Errorf("failed to convert %s data to Arrow: %w", metric, err)
	}

	results <- source.RecordBatchResult{Batch: record}
	config.Debug("[ISOC-PULSE] Sent %d records for %s", len(items), metric)
	return nil
}
