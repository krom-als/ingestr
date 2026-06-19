package salesforce

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bruin-data/ingestr/internal/config"
	"github.com/bruin-data/ingestr/pkg/source"
	"github.com/simpleforce/simpleforce"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSalesforceURIWithPasswordAuth(t *testing.T) {
	cfg, err := parseSalesforceURI("salesforce://?username=user&password=pass&token=tok&domain=login")
	if err != nil {
		t.Fatalf("parseSalesforceURI returned error: %v", err)
	}

	if cfg.authMethod != salesforceAuthPassword {
		t.Fatalf("authMethod = %q, want %q", cfg.authMethod, salesforceAuthPassword)
	}
	if cfg.username != "user" || cfg.password != "pass" || cfg.token != "tok" || cfg.domain != "login" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestParseSalesforceURIWithClientCredentialsAuth(t *testing.T) {
	cfg, err := parseSalesforceURI("salesforce://?client_id=id&client_secret=secret&domain=my-domain.my&grant_type=client_credentials")
	if err != nil {
		t.Fatalf("parseSalesforceURI returned error: %v", err)
	}

	if cfg.authMethod != salesforceAuthClientCredentials {
		t.Fatalf("authMethod = %q, want %q", cfg.authMethod, salesforceAuthClientCredentials)
	}
	if cfg.clientID != "id" || cfg.clientSecret != "secret" || cfg.domain != "my-domain.my" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestParseSalesforceURIInfersClientCredentialsAuth(t *testing.T) {
	cfg, err := parseSalesforceURI("salesforce://?client_id=id&client_secret=secret&domain=test")
	if err != nil {
		t.Fatalf("parseSalesforceURI returned error: %v", err)
	}

	if cfg.authMethod != salesforceAuthClientCredentials {
		t.Fatalf("authMethod = %q, want %q", cfg.authMethod, salesforceAuthClientCredentials)
	}
}

func TestParseSalesforceURIInfersAccessTokenAuth(t *testing.T) {
	cfg, err := parseSalesforceURI("salesforce://?access_token=access-token&domain=https://company.my.salesforce.com")
	if err != nil {
		t.Fatalf("parseSalesforceURI returned error: %v", err)
	}

	if cfg.authMethod != salesforceAuthAccessToken {
		t.Fatalf("authMethod = %q, want %q", cfg.authMethod, salesforceAuthAccessToken)
	}
	if cfg.accessToken != "access-token" || cfg.domain != "https://company.my.salesforce.com" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestParseSalesforceURIRequiresClientSecretForClientCredentials(t *testing.T) {
	_, err := parseSalesforceURI("salesforce://?client_id=id&domain=test&grant_type=client_credentials")
	if err == nil {
		t.Fatal("parseSalesforceURI returned nil error, want validation error")
	}
}

func TestParseSalesforceURIRequiresAccessTokenForAccessTokenAuth(t *testing.T) {
	_, err := parseSalesforceURI("salesforce://?auth_method=access_token&domain=test")
	if err == nil {
		t.Fatal("parseSalesforceURI returned nil error, want validation error")
	}
}

func TestSalesforceBaseURL(t *testing.T) {
	tests := []struct {
		name   string
		domain string
		want   string
	}{
		{name: "login domain", domain: "login", want: "https://login.salesforce.com"},
		{name: "my domain", domain: "company.my", want: "https://company.my.salesforce.com"},
		{name: "salesforce host", domain: "company.my.salesforce.com", want: "https://company.my.salesforce.com"},
		{name: "explicit URL", domain: "http://127.0.0.1:8080", want: "http://127.0.0.1:8080"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := salesforceBaseURL(tt.domain)
			if got != tt.want {
				t.Fatalf("salesforceBaseURL(%q) = %q, want %q", tt.domain, got, tt.want)
			}
		})
	}
}

func TestConnectWithAccessTokenAuth(t *testing.T) {
	src := NewSalesforceSource()

	if err := src.Connect(context.Background(), "salesforce://?access_token=access-token&domain=https://company.my.salesforce.com"); err != nil {
		t.Fatalf("Connect returned error: %v", err)
	}
	defer func() { _ = src.Close(context.Background()) }()

	if got := src.sessionID; got != "access-token" {
		t.Fatalf("sessionID = %q, want %q", got, "access-token")
	}
	if got := src.instanceURL; got != "https://company.my.salesforce.com" {
		t.Fatalf("instanceURL = %q, want %q", got, "https://company.my.salesforce.com")
	}
}

func TestLoginClientCredentials(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != salesforceOAuthTokenPath {
			t.Fatalf("path = %q, want %q", r.URL.Path, salesforceOAuthTokenPath)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("method = %q, want %q", r.Method, http.MethodPost)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm returned error: %v", err)
		}
		if got := r.Form.Get("grant_type"); got != string(salesforceAuthClientCredentials) {
			t.Fatalf("grant_type = %q, want %q", got, salesforceAuthClientCredentials)
		}
		if got := r.Form.Get("client_id"); got != "client-id" {
			t.Fatalf("client_id = %q, want %q", got, "client-id")
		}
		if got := r.Form.Get("client_secret"); got != "client-secret" {
			t.Fatalf("client_secret = %q, want %q", got, "client-secret")
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"access-token","instance_url":"` + server.URL + `","token_type":"Bearer"}`))
	}))
	defer server.Close()

	src := &salesforceSource{
		client:         simpleforce.NewClient(server.URL, "client-id", defaultAPIVersion),
		sfUrl:          server.URL,
		sfClientID:     "client-id",
		sfClientSecret: "client-secret",
	}

	if err := src.loginClientCredentials(context.Background()); err != nil {
		t.Fatalf("loginClientCredentials returned error: %v", err)
	}
	if got := src.client.GetSid(); got != "access-token" {
		t.Fatalf("sid = %q, want %q", got, "access-token")
	}
	if got := src.client.GetLoc(); got != server.URL {
		t.Fatalf("instance URL = %q, want %q", got, server.URL)
	}
}

func TestSalesforceGetTableTableNameParsing(t *testing.T) {
	t.Parallel()

	s := NewSalesforceSource()

	tests := []struct {
		name          string
		req           source.TableRequest
		wantErr       bool
		errSubstr     string
		wantTableName string
		wantPKs       []string
		wantStrategy  config.IncrementalStrategy
		wantIncrKey   string
	}{
		{
			name:          "known standard table account",
			req:           source.TableRequest{Name: "account"},
			wantTableName: "account",
			wantPKs:       []string{"Id"},
			wantStrategy:  config.StrategyMerge,
			wantIncrKey:   "SystemModstamp",
		},
		{
			name:          "known replace-strategy table campaign",
			req:           source.TableRequest{Name: "campaign"},
			wantTableName: "campaign",
			wantPKs:       []string{"Id"},
			wantStrategy:  config.StrategyReplace,
			wantIncrKey:   "",
		},
		{
			name:          "custom object legacy colon form",
			req:           source.TableRequest{Name: "custom:MyObject__c"},
			wantTableName: "custom:MyObject__c",
			wantPKs:       nil,
			wantStrategy:  config.StrategyReplace,
			wantIncrKey:   "",
		},
		{
			name:          "custom object with incremental key override",
			req:           source.TableRequest{Name: "custom:MyObject__c", IncrementalKey: "UpdatedAt__c"},
			wantTableName: "custom:MyObject__c",
			wantPKs:       []string{"Id"},
			wantStrategy:  config.StrategyMerge,
			wantIncrKey:   "UpdatedAt__c",
		},
		{
			name:          "standard table incremental key override switches to merge",
			req:           source.TableRequest{Name: "account", IncrementalKey: "CreatedDate"},
			wantTableName: "account",
			wantPKs:       []string{"Id"},
			wantStrategy:  config.StrategyMerge,
			wantIncrKey:   "CreatedDate",
		},
		{
			name:          "standard table with caller-supplied PKs preserved",
			req:           source.TableRequest{Name: "user", PrimaryKeys: []string{"Username"}},
			wantTableName: "user",
			wantPKs:       []string{"Username"},
			wantStrategy:  config.StrategyReplace,
			wantIncrKey:   "",
		},
		{
			name:      "empty table name",
			req:       source.TableRequest{Name: ""},
			wantErr:   true,
			errSubstr: "table name is required",
		},
		{
			name:      "unknown table without custom prefix",
			req:       source.TableRequest{Name: "not_a_real_table"},
			wantErr:   true,
			errSubstr: "unsupported table",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tbl, err := s.GetTable(context.Background(), tt.req)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errSubstr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantTableName, tbl.Name())
			assert.Equal(t, tt.wantPKs, tbl.PrimaryKeys())
			assert.Equal(t, tt.wantStrategy, tbl.Strategy())
			assert.Equal(t, tt.wantIncrKey, tbl.IncrementalKey())
		})
	}
}

func TestParseSalesforceTableSpec(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		want      string
		wantErr   bool
		errSubstr string
	}{
		// Legacy forms — returned verbatim.
		{name: "standard table", input: "account", want: "account"},
		{name: "custom legacy colon", input: "custom:MyObject__c", want: "custom:MyObject__c"},
		{name: "custom legacy colon complex", input: "custom:My_Namespace__MyObj__c", want: "custom:My_Namespace__MyObj__c"},
		// Query form — normalized to legacy.
		{name: "query form basic", input: "custom?object=MyObject__c", want: "custom:MyObject__c"},
		{name: "query form complex name", input: "custom?object=My_Namespace__MyObj__c", want: "custom:My_Namespace__MyObj__c"},
		// Error cases for query form.
		{name: "query form wrong path", input: "account?object=Foo__c", wantErr: true, errSubstr: `path must be "custom"`},
		{name: "query form missing object", input: "custom?object=", wantErr: true, errSubstr: "object parameter is required"},
		{name: "query form unknown key", input: "custom?object=Foo__c&typo=x", wantErr: true, errSubstr: "unknown table parameter"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseSalesforceTableSpec(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errSubstr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetTableQueryForm(t *testing.T) {
	t.Parallel()

	s := NewSalesforceSource()

	tests := []struct {
		name          string
		req           source.TableRequest
		wantTableName string
		wantPKs       []string
		wantStrategy  config.IncrementalStrategy
		wantIncrKey   string
		wantErr       bool
		errSubstr     string
	}{
		{
			name:          "query form custom object",
			req:           source.TableRequest{Name: "custom?object=My_Object__c"},
			wantTableName: "custom:My_Object__c",
			wantPKs:       nil,
			wantStrategy:  config.StrategyReplace,
			wantIncrKey:   "",
		},
		{
			name:          "query form custom object with incremental key",
			req:           source.TableRequest{Name: "custom?object=My_Object__c", IncrementalKey: "UpdatedAt__c"},
			wantTableName: "custom:My_Object__c",
			wantPKs:       []string{"Id"},
			wantStrategy:  config.StrategyMerge,
			wantIncrKey:   "UpdatedAt__c",
		},
		{
			// Equivalent: custom?object=My_Object__c == custom:My_Object__c
			name:          "query form equivalent to legacy colon form",
			req:           source.TableRequest{Name: "custom?object=My_Object__c"},
			wantTableName: "custom:My_Object__c",
			wantStrategy:  config.StrategyReplace,
		},
		{
			name:      "query form missing object value",
			req:       source.TableRequest{Name: "custom?object="},
			wantErr:   true,
			errSubstr: "object parameter is required",
		},
		{
			name:      "query form wrong path",
			req:       source.TableRequest{Name: "account?object=Foo__c"},
			wantErr:   true,
			errSubstr: `path must be "custom"`,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tbl, err := s.GetTable(context.Background(), tt.req)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errSubstr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantTableName, tbl.Name())
			assert.Equal(t, tt.wantPKs, tbl.PrimaryKeys())
			assert.Equal(t, tt.wantStrategy, tbl.Strategy())
			assert.Equal(t, tt.wantIncrKey, tbl.IncrementalKey())
		})
	}
}
