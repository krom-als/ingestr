package isoc_pulse

import (
	"net/url"
	"testing"
)

func TestParseURI(t *testing.T) {
	tests := []struct {
		name    string
		uri     string
		want    string
		wantErr bool
	}{
		{
			name: "valid URI",
			uri:  "isoc-pulse://?token=abc123",
			want: "abc123",
		},
		{
			name: "valid URI with special characters",
			uri:  "isoc-pulse://?token=abc-123_XYZ.test",
			want: "abc-123_XYZ.test",
		},
		{
			name:    "missing token",
			uri:     "isoc-pulse://",
			wantErr: true,
		},
		{
			name:    "empty token",
			uri:     "isoc-pulse://?token=",
			wantErr: true,
		},
		{
			name:    "wrong scheme",
			uri:     "postgres://?token=abc123",
			wantErr: true,
		},
		{
			name:    "empty URI",
			uri:     "",
			wantErr: true,
		},
		{
			name:    "just question mark",
			uri:     "isoc-pulse://?",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseURI(tt.uri)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseURI() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseURI() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsValidTable(t *testing.T) {
	validTables := []string{
		"dnssec_adoption", "dnssec_tld_adoption", "dnssec_validation",
		"http", "http3", "https", "ipv6", "net_loss", "resilience",
		"roa", "rov", "tls", "tls13",
	}

	for _, table := range validTables {
		if !isValidTable(table) {
			t.Errorf("isValidTable(%q) = false, want true", table)
		}
	}

	invalidTables := []string{"", "unknown", "HTTP", "Https", "dns", "ipv4"}
	for _, table := range invalidTables {
		if isValidTable(table) {
			t.Errorf("isValidTable(%q) = true, want false", table)
		}
	}
}

func TestParseTableName(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantMetric string
		wantOpts   []string
		wantErr    bool
	}{
		{
			name:       "simple metric",
			input:      "https",
			wantMetric: "https",
			wantOpts:   nil,
		},
		{
			name:       "metric with country",
			input:      "https:US",
			wantMetric: "https",
			wantOpts:   []string{"US"},
		},
		{
			name:       "metric with topsites and country",
			input:      "https:topsites:US",
			wantMetric: "https",
			wantOpts:   []string{"topsites", "US"},
		},
		{
			name:       "net_loss with shutdown type and country",
			input:      "net_loss:shutdown:IN",
			wantMetric: "net_loss",
			wantOpts:   []string{"shutdown", "IN"},
		},
		{
			name:       "roa with ip version",
			input:      "roa:4",
			wantMetric: "roa",
			wantOpts:   []string{"4"},
		},
		{
			name:       "roa with ip version and country",
			input:      "roa:6:BR",
			wantMetric: "roa",
			wantOpts:   []string{"6", "BR"},
		},
		{
			name:       "resilience with empty option and country",
			input:      "resilience::FR",
			wantMetric: "resilience",
			wantOpts:   []string{"", "FR"},
		},
		{
			name:    "empty name",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metric, opts, err := parseTableName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseTableName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if metric != tt.wantMetric {
				t.Errorf("parseTableName() metric = %v, want %v", metric, tt.wantMetric)
			}
			if len(opts) != len(tt.wantOpts) {
				t.Errorf("parseTableName() opts = %v, want %v", opts, tt.wantOpts)
				return
			}
			for i := range opts {
				if opts[i] != tt.wantOpts[i] {
					t.Errorf("parseTableName() opts[%d] = %v, want %v", i, opts[i], tt.wantOpts[i])
				}
			}
		})
	}
}

func TestValidateOptions(t *testing.T) {
	tests := []struct {
		name    string
		metric  string
		opts    []string
		wantErr bool
	}{
		{name: "http no options", metric: "http", opts: nil, wantErr: false},
		{name: "http with options", metric: "http", opts: []string{"US"}, wantErr: true},
		{name: "tls no options", metric: "tls", opts: nil, wantErr: false},
		{name: "tls with options", metric: "tls", opts: []string{"US"}, wantErr: true},
		{name: "rov no options", metric: "rov", opts: nil, wantErr: false},
		{name: "rov with options", metric: "rov", opts: []string{"US"}, wantErr: true},
		{name: "net_loss valid", metric: "net_loss", opts: []string{"shutdown", "IN"}, wantErr: false},
		{name: "net_loss missing country", metric: "net_loss", opts: []string{"shutdown"}, wantErr: true},
		{name: "net_loss no options", metric: "net_loss", opts: nil, wantErr: true},
		{name: "https with country", metric: "https", opts: []string{"US"}, wantErr: false},
		{name: "https no options", metric: "https", opts: nil, wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateOptions(tt.metric, tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateOptions(%q, %v) error = %v, wantErr %v", tt.metric, tt.opts, err, tt.wantErr)
			}
		})
	}
}

func TestParseTableNameQueryForm(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantMetric string
		wantOpts   []string
		wantErr    bool
	}{
		{
			name:       "net_loss with shutdown_type and country",
			input:      "net_loss?shutdown_type=shutdown&country=US",
			wantMetric: "net_loss",
			wantOpts:   []string{"shutdown", "US"},
		},
		{
			name:       "https with topsites and country",
			input:      "https?topsites=true&country=DE",
			wantMetric: "https",
			wantOpts:   []string{"topsites", "DE"},
		},
		{
			name:       "https with country only",
			input:      "https?country=US",
			wantMetric: "https",
			wantOpts:   []string{"US"},
		},
		{
			name:       "https with topsites only",
			input:      "https?topsites=true",
			wantMetric: "https",
			wantOpts:   []string{"topsites"},
		},
		{
			name:       "https no params",
			input:      "https?topsites=false",
			wantMetric: "https",
			wantOpts:   nil,
		},
		{
			name:       "ipv6 with country",
			input:      "ipv6?country=FR",
			wantMetric: "ipv6",
			wantOpts:   []string{"FR"},
		},
		{
			name:       "ipv6 with topsites and country",
			input:      "ipv6?topsites=true&country=JP",
			wantMetric: "ipv6",
			wantOpts:   []string{"topsites", "JP"},
		},
		{
			name:       "roa with ip_version and country",
			input:      "roa?ip_version=4&country=US",
			wantMetric: "roa",
			wantOpts:   []string{"4", "US"},
		},
		{
			name:       "roa with ip_version only",
			input:      "roa?ip_version=6",
			wantMetric: "roa",
			wantOpts:   []string{"6"},
		},
		{
			name:       "dnssec_validation with country",
			input:      "dnssec_validation?country=SE",
			wantMetric: "dnssec_validation",
			wantOpts:   []string{"SE"},
		},
		{
			name:       "dnssec_adoption with country",
			input:      "dnssec_adoption?country=BR",
			wantMetric: "dnssec_adoption",
			wantOpts:   []string{"BR"},
		},
		{
			name:       "resilience with country",
			input:      "resilience?country=IN",
			wantMetric: "resilience",
			wantOpts:   []string{"IN"},
		},
		{
			name:       "http no params (no-option metric)",
			input:      "http",
			wantMetric: "http",
			wantOpts:   nil,
		},
		{
			name:    "unknown param key",
			input:   "https?region=US",
			wantErr: true,
		},
		{
			name:       "https topsites=1 treated as true",
			input:      "https?topsites=1",
			wantMetric: "https",
			wantOpts:   []string{"topsites"},
		},
		{
			name:    "http with country param rejected",
			input:   "http?country=US",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metric, opts, err := parseTableName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseTableName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if metric != tt.wantMetric {
				t.Errorf("parseTableName() metric = %q, want %q", metric, tt.wantMetric)
			}
			if len(opts) != len(tt.wantOpts) {
				t.Errorf("parseTableName() opts = %v, want %v", opts, tt.wantOpts)
				return
			}
			for i := range opts {
				if opts[i] != tt.wantOpts[i] {
					t.Errorf("parseTableName() opts[%d] = %q, want %q", i, opts[i], tt.wantOpts[i])
				}
			}
		})
	}
}

func TestBuildOptsFromParams(t *testing.T) {
	makeParams := func(kv ...string) url.Values {
		v := url.Values{}
		for i := 0; i+1 < len(kv); i += 2 {
			v.Set(kv[i], kv[i+1])
		}
		return v
	}

	tests := []struct {
		name     string
		metric   string
		params   url.Values
		wantOpts []string
		wantErr  bool
	}{
		{
			name:     "net_loss shutdown and country",
			metric:   "net_loss",
			params:   makeParams("shutdown_type", "shutdown", "country", "IN"),
			wantOpts: []string{"shutdown", "IN"},
		},
		{
			name:     "net_loss blackout and country",
			metric:   "net_loss",
			params:   makeParams("shutdown_type", "blackout", "country", "RU"),
			wantOpts: []string{"blackout", "RU"},
		},
		{
			name:     "roa ip_version and country",
			metric:   "roa",
			params:   makeParams("ip_version", "6", "country", "BR"),
			wantOpts: []string{"6", "BR"},
		},
		{
			name:     "roa ip_version only",
			metric:   "roa",
			params:   makeParams("ip_version", "4"),
			wantOpts: []string{"4"},
		},
		{
			name:     "https topsites and country",
			metric:   "https",
			params:   makeParams("topsites", "true", "country", "US"),
			wantOpts: []string{"topsites", "US"},
		},
		{
			name:     "https country only",
			metric:   "https",
			params:   makeParams("country", "US"),
			wantOpts: []string{"US"},
		},
		{
			name:     "https topsites false no country",
			metric:   "https",
			params:   makeParams("topsites", "false"),
			wantOpts: nil,
		},
		{
			name:     "ipv6 topsites and country",
			metric:   "ipv6",
			params:   makeParams("topsites", "true", "country", "DE"),
			wantOpts: []string{"topsites", "DE"},
		},
		{
			name:     "dnssec_validation country",
			metric:   "dnssec_validation",
			params:   makeParams("country", "SE"),
			wantOpts: []string{"SE"},
		},
		{
			name:     "resilience country",
			metric:   "resilience",
			params:   makeParams("country", "CN"),
			wantOpts: []string{"CN"},
		},
		{
			name:     "no-option metric no params",
			metric:   "http",
			params:   url.Values{},
			wantOpts: nil,
		},
		{
			name:    "net_loss empty shutdown_type and country",
			metric:  "net_loss",
			params:  makeParams("shutdown_type", "", "country", ""),
			wantErr: true,
		},
		{
			name:    "net_loss missing shutdown_type",
			metric:  "net_loss",
			params:  makeParams("country", "US"),
			wantErr: true,
		},
		{
			name:    "no-option metric with country param",
			metric:  "http",
			params:  makeParams("country", "US"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, err := buildOptsFromParams(tt.metric, tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("buildOptsFromParams() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if len(opts) != len(tt.wantOpts) {
				t.Errorf("buildOptsFromParams() = %v, want %v", opts, tt.wantOpts)
				return
			}
			for i := range opts {
				if opts[i] != tt.wantOpts[i] {
					t.Errorf("buildOptsFromParams()[%d] = %q, want %q", i, opts[i], tt.wantOpts[i])
				}
			}
		})
	}
}

func TestQueryFormEquivalence(t *testing.T) {
	tests := []struct {
		name   string
		legacy string
		query  string
	}{
		{
			name:   "net_loss shutdown US",
			legacy: "net_loss:shutdown:US",
			query:  "net_loss?shutdown_type=shutdown&country=US",
		},
		{
			name:   "https topsites DE",
			legacy: "https:topsites:DE",
			query:  "https?topsites=true&country=DE",
		},
		{
			name:   "https country only",
			legacy: "https:US",
			query:  "https?country=US",
		},
		{
			name:   "roa ip_version and country",
			legacy: "roa:6:BR",
			query:  "roa?ip_version=6&country=BR",
		},
		{
			name:   "roa ip_version only",
			legacy: "roa:4",
			query:  "roa?ip_version=4",
		},
		{
			name:   "dnssec_validation country",
			legacy: "dnssec_validation:SE",
			query:  "dnssec_validation?country=SE",
		},
		{
			name:   "ipv6 country",
			legacy: "ipv6:DE",
			query:  "ipv6?country=DE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lMetric, lOpts, err := parseTableName(tt.legacy)
			if err != nil {
				t.Fatalf("legacy parseTableName(%q) error: %v", tt.legacy, err)
			}
			qMetric, qOpts, err := parseTableName(tt.query)
			if err != nil {
				t.Fatalf("query parseTableName(%q) error: %v", tt.query, err)
			}

			if lMetric != qMetric {
				t.Errorf("metric mismatch: legacy=%q query=%q", lMetric, qMetric)
			}

			lCfg := buildMetricConfig(lMetric, lOpts, nil)
			qCfg := buildMetricConfig(qMetric, qOpts, nil)

			if lCfg.path != qCfg.path {
				t.Errorf("path mismatch: legacy=%q query=%q", lCfg.path, qCfg.path)
			}
			for k, lv := range lCfg.params {
				qv, ok := qCfg.params[k]
				if !ok {
					t.Errorf("query config missing param %q (value=%q)", k, lv)
					continue
				}
				if lv != qv {
					t.Errorf("param %q: legacy=%q query=%q", k, lv, qv)
				}
			}
			for k := range qCfg.params {
				if _, ok := lCfg.params[k]; !ok {
					t.Errorf("query config has extra param %q", k)
				}
			}
		})
	}
}

func TestBuildMetricConfig(t *testing.T) {
	tests := []struct {
		name     string
		metric   string
		opts     []string
		wantPath string
		wantKeys []string
	}{
		{
			name:     "https global",
			metric:   "https",
			opts:     nil,
			wantPath: "https",
		},
		{
			name:     "https country",
			metric:   "https",
			opts:     []string{"US"},
			wantPath: "https/country/US",
		},
		{
			name:     "https topsites",
			metric:   "https",
			opts:     []string{"topsites"},
			wantPath: "https",
			wantKeys: []string{"topsites"},
		},
		{
			name:     "https topsites country",
			metric:   "https",
			opts:     []string{"topsites", "US"},
			wantPath: "https/country/US",
			wantKeys: []string{"topsites"},
		},
		{
			name:     "ipv6 global",
			metric:   "ipv6",
			opts:     nil,
			wantPath: "ipv6",
		},
		{
			name:     "ipv6 country",
			metric:   "ipv6",
			opts:     []string{"DE"},
			wantPath: "ipv6/country/DE",
		},
		{
			name:     "ipv6 topsites",
			metric:   "ipv6",
			opts:     []string{"topsites"},
			wantPath: "ipv6",
			wantKeys: []string{"topsites"},
		},
		{
			name:     "dnssec_validation global",
			metric:   "dnssec_validation",
			opts:     nil,
			wantPath: "dnssec/validation",
		},
		{
			name:     "dnssec_validation country",
			metric:   "dnssec_validation",
			opts:     []string{"SE"},
			wantPath: "dnssec/validation/country/SE",
		},
		{
			name:     "dnssec_tld_adoption country",
			metric:   "dnssec_tld_adoption",
			opts:     []string{"JP"},
			wantPath: "dnssec/adoption/country/JP",
		},
		{
			name:     "dnssec_adoption domain",
			metric:   "dnssec_adoption",
			opts:     []string{"BR"},
			wantPath: "dnssec/adoption/domains/BR",
		},
		{
			name:     "roa ip version",
			metric:   "roa",
			opts:     []string{"4"},
			wantPath: "roa",
			wantKeys: []string{"ip_version"},
		},
		{
			name:     "roa ip version and country",
			metric:   "roa",
			opts:     []string{"6", "BR"},
			wantPath: "roa/country/BR",
			wantKeys: []string{"ip_version"},
		},
		{
			name:     "net_loss",
			metric:   "net_loss",
			opts:     []string{"shutdown", "IN"},
			wantPath: "net-loss",
			wantKeys: []string{"shutdown_type", "country"},
		},
		{
			name:     "rov global",
			metric:   "rov",
			opts:     nil,
			wantPath: "rov",
		},
		{
			name:     "tls global",
			metric:   "tls",
			opts:     nil,
			wantPath: "tls",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := buildMetricConfig(tt.metric, tt.opts, nil)
			if cfg.path != tt.wantPath {
				t.Errorf("buildMetricConfig() path = %q, want %q", cfg.path, tt.wantPath)
			}
			for _, key := range tt.wantKeys {
				if _, ok := cfg.params[key]; !ok {
					t.Errorf("buildMetricConfig() missing param key %q", key)
				}
			}
		})
	}
}
