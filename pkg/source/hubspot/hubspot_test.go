package hubspot

import (
	"reflect"
	"testing"
)

// TestParseHubspotTableSpec_LegacyForms verifies that all legacy colon-delimited
// forms still parse to the same internal hubspotTableSpec as before.
func TestParseHubspotTableSpec_LegacyForms(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		wantBase  string
		wantProps []string
		wantAssoc []string
	}{
		{
			name:     "plain table",
			input:    "contacts",
			wantBase: "contacts",
		},
		{
			name:      "table with assoc override single",
			input:     "contacts:companies",
			wantBase:  "contacts",
			wantAssoc: []string{"companies"},
		},
		{
			name:      "table with assoc override multiple",
			input:     "contacts:companies,deals,tickets",
			wantBase:  "contacts",
			wantAssoc: []string{"companies", "deals", "tickets"},
		},
		{
			name:      "table with empty assoc override",
			input:     "contacts:",
			wantBase:  "contacts",
			wantAssoc: []string{},
		},
		{
			name:     "builtin history no props",
			input:    "property_history:contacts",
			wantBase: "property_history:contacts",
		},
		{
			name:      "builtin history with props",
			input:     "property_history:contacts:email,firstname",
			wantBase:  "property_history:contacts",
			wantProps: []string{"email", "firstname"},
		},
		{
			name:     "custom object no assoc",
			input:    "custom:myObj",
			wantBase: "custom:myObj",
		},
		{
			name:     "custom object with assoc",
			input:    "custom:myObj:assoc1,assoc2",
			wantBase: "custom:myObj:assoc1,assoc2",
		},
		{
			name:     "custom history no props",
			input:    "property_history:custom:myObj",
			wantBase: "property_history:custom:myObj",
		},
		{
			name:      "custom history with props",
			input:     "property_history:custom:myObj:p1,p2",
			wantBase:  "property_history:custom:myObj",
			wantProps: []string{"p1", "p2"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseHubspotTableSpec(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.base != tc.wantBase {
				t.Errorf("base: got %q, want %q", got.base, tc.wantBase)
			}
			if !reflect.DeepEqual(got.historyProps, tc.wantProps) {
				t.Errorf("historyProps: got %#v, want %#v", got.historyProps, tc.wantProps)
			}
			if !reflect.DeepEqual(got.assocOverride, tc.wantAssoc) {
				t.Errorf("assocOverride: got %#v, want %#v", got.assocOverride, tc.wantAssoc)
			}
		})
	}
}

// TestParseHubspotTableSpec_QueryForms verifies the URL-style query form.
func TestParseHubspotTableSpec_QueryForms(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		wantBase  string
		wantProps []string
		wantAssoc []string
		wantErr   bool
	}{
		{
			name:     "plain table no params",
			input:    "contacts",
			wantBase: "contacts",
		},
		{
			name:      "assoc override single",
			input:     "contacts?associations=companies",
			wantBase:  "contacts",
			wantAssoc: []string{"companies"},
		},
		{
			name:      "assoc override repeated",
			input:     "contacts?associations=companies&associations=deals",
			wantBase:  "contacts",
			wantAssoc: []string{"companies", "deals"},
		},
		{
			name:      "assoc override comma-joined",
			input:     "contacts?associations=companies,deals",
			wantBase:  "contacts",
			wantAssoc: []string{"companies", "deals"},
		},
		{
			name:     "history no props",
			input:    "property_history:contacts",
			wantBase: "property_history:contacts",
		},
		{
			name:      "history with props repeated",
			input:     "property_history:contacts?properties=email&properties=firstname",
			wantBase:  "property_history:contacts",
			wantProps: []string{"email", "firstname"},
		},
		{
			name:      "history with props comma-joined",
			input:     "property_history:contacts?properties=email,firstname",
			wantBase:  "property_history:contacts",
			wantProps: []string{"email", "firstname"},
		},
		{
			name:     "custom object",
			input:    "custom?object=myObj",
			wantBase: "custom:myObj",
		},
		{
			name:     "custom object with assoc repeated",
			input:    "custom?object=myObj&associations=assoc1&associations=assoc2",
			wantBase: "custom:myObj:assoc1,assoc2",
		},
		{
			name:     "custom history no props",
			input:    "property_history:custom?object=myObj",
			wantBase: "property_history:custom:myObj",
		},
		{
			name:      "custom history with props",
			input:     "property_history:custom?object=myObj&properties=p1&properties=p2",
			wantBase:  "property_history:custom:myObj",
			wantProps: []string{"p1", "p2"},
		},
		{
			name:    "custom missing object param",
			input:   "custom?associations=foo",
			wantErr: true,
		},
		{
			name:    "custom history missing object param",
			input:   "property_history:custom?properties=p1",
			wantErr: true,
		},
		{
			name:    "unknown param",
			input:   "contacts?typo=val",
			wantErr: true,
		},
		{
			name:    "object param on standard table",
			input:   "contacts?object=something",
			wantErr: true,
		},
		{
			name:    "properties param on standard table rejected",
			input:   "contacts?properties=email",
			wantErr: true,
		},
		{
			name:    "properties param on standard table with assoc also rejected",
			input:   "deals?associations=companies&properties=amount",
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseHubspotTableSpec(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (base=%q)", got.base)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.base != tc.wantBase {
				t.Errorf("base: got %q, want %q", got.base, tc.wantBase)
			}
			if !reflect.DeepEqual(got.historyProps, tc.wantProps) {
				t.Errorf("historyProps: got %#v, want %#v", got.historyProps, tc.wantProps)
			}
			if !reflect.DeepEqual(got.assocOverride, tc.wantAssoc) {
				t.Errorf("assocOverride: got %#v, want %#v", got.assocOverride, tc.wantAssoc)
			}
		})
	}
}

// TestParseHubspotTableSpec_Equivalence proves that legacy and query forms
// produce byte-identical internal state for every representative input.
func TestParseHubspotTableSpec_Equivalence(t *testing.T) {
	pairs := []struct {
		desc   string
		legacy string
		query  string
	}{
		{
			desc:   "contacts plain",
			legacy: "contacts",
			query:  "contacts",
		},
		{
			desc:   "assoc override single",
			legacy: "contacts:companies",
			query:  "contacts?associations=companies",
		},
		{
			desc:   "assoc override multiple",
			legacy: "contacts:companies,deals",
			query:  "contacts?associations=companies&associations=deals",
		},
		{
			desc:   "assoc override comma-joined value",
			legacy: "contacts:companies,deals",
			query:  "contacts?associations=companies,deals",
		},
		{
			desc:   "history no props",
			legacy: "property_history:contacts",
			query:  "property_history:contacts",
		},
		{
			desc:   "history with props",
			legacy: "property_history:contacts:email,firstname",
			query:  "property_history:contacts?properties=email&properties=firstname",
		},
		{
			desc:   "custom object no assoc",
			legacy: "custom:myObj",
			query:  "custom?object=myObj",
		},
		{
			desc:   "custom object with assoc",
			legacy: "custom:myObj:assoc1,assoc2",
			query:  "custom?object=myObj&associations=assoc1&associations=assoc2",
		},
		{
			desc:   "custom history no props",
			legacy: "property_history:custom:myObj",
			query:  "property_history:custom?object=myObj",
		},
		{
			desc:   "custom history with props",
			legacy: "property_history:custom:myObj:p1,p2",
			query:  "property_history:custom?object=myObj&properties=p1&properties=p2",
		},
	}

	for _, p := range pairs {
		t.Run(p.desc, func(t *testing.T) {
			legacySpec, err := parseHubspotTableSpec(p.legacy)
			if err != nil {
				t.Fatalf("legacy %q: unexpected error: %v", p.legacy, err)
			}
			querySpec, err := parseHubspotTableSpec(p.query)
			if err != nil {
				t.Fatalf("query %q: unexpected error: %v", p.query, err)
			}
			if legacySpec.base != querySpec.base {
				t.Errorf("base mismatch: legacy %q → %q, query %q → %q",
					p.legacy, legacySpec.base, p.query, querySpec.base)
			}
			if !reflect.DeepEqual(legacySpec.historyProps, querySpec.historyProps) {
				t.Errorf("historyProps mismatch: legacy %#v, query %#v",
					legacySpec.historyProps, querySpec.historyProps)
			}
			if !reflect.DeepEqual(legacySpec.assocOverride, querySpec.assocOverride) {
				t.Errorf("assocOverride mismatch: legacy %#v, query %#v",
					legacySpec.assocOverride, querySpec.assocOverride)
			}
		})
	}
}

func TestParseHistoryTableName(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		wantBase  string
		wantProps []string
	}{
		{
			name:      "non-history unchanged",
			input:     "contacts",
			wantBase:  "contacts",
			wantProps: nil,
		},
		{
			name:      "non-history custom unchanged",
			input:     "custom:myObj:assoc1,assoc2",
			wantBase:  "custom:myObj:assoc1,assoc2",
			wantProps: nil,
		},
		{
			name:      "builtin history no suffix",
			input:     "property_history:contacts",
			wantBase:  "property_history:contacts",
			wantProps: nil,
		},
		{
			name:      "builtin history single prop",
			input:     "property_history:contacts:email",
			wantBase:  "property_history:contacts",
			wantProps: []string{"email"},
		},
		{
			name:      "builtin history multiple props",
			input:     "property_history:contacts:email,firstname,lastname",
			wantBase:  "property_history:contacts",
			wantProps: []string{"email", "firstname", "lastname"},
		},
		{
			name:      "builtin history trailing comma",
			input:     "property_history:contacts:email,firstname,",
			wantBase:  "property_history:contacts",
			wantProps: []string{"email", "firstname"},
		},
		{
			name:      "builtin history whitespace",
			input:     "property_history:contacts: email , firstname ",
			wantBase:  "property_history:contacts",
			wantProps: []string{"email", "firstname"},
		},
		{
			name:      "builtin history empty suffix",
			input:     "property_history:contacts:",
			wantBase:  "property_history:contacts",
			wantProps: nil,
		},
		{
			name:      "custom history no suffix",
			input:     "property_history:custom:myObj",
			wantBase:  "property_history:custom:myObj",
			wantProps: nil,
		},
		{
			name:      "custom history with props",
			input:     "property_history:custom:myObj:p1,p2",
			wantBase:  "property_history:custom:myObj",
			wantProps: []string{"p1", "p2"},
		},
		{
			name:      "custom history only commas",
			input:     "property_history:custom:myObj:,,,",
			wantBase:  "property_history:custom:myObj",
			wantProps: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotBase, gotProps := parseHistoryTableName(tc.input)
			if gotBase != tc.wantBase {
				t.Errorf("base: got %q, want %q", gotBase, tc.wantBase)
			}
			if !reflect.DeepEqual(gotProps, tc.wantProps) {
				t.Errorf("props: got %#v, want %#v", gotProps, tc.wantProps)
			}
		})
	}
}

func TestParseHubspotURI(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "api_key",
			input: "hubspot://?api_key=pat_test_12345",
			want:  "pat_test_12345",
		},
		{
			name:  "service_key",
			input: "hubspot://?service_key=sk_test_67890",
			want:  "sk_test_67890",
		},
		{
			name:  "both equal",
			input: "hubspot://?api_key=tok&service_key=tok",
			want:  "tok",
		},
		{
			name:    "both differ",
			input:   "hubspot://?api_key=a&service_key=b",
			wantErr: true,
		},
		{
			name:    "missing credential",
			input:   "hubspot://?",
			wantErr: true,
		},
		{
			name:    "empty",
			input:   "hubspot://",
			wantErr: true,
		},
		{
			name:    "wrong scheme",
			input:   "postgres://?api_key=x",
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseHubspotURI(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (value %q)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParseTableAssocOverride(t *testing.T) {
	cases := []struct {
		name         string
		input        string
		wantBase     string
		wantOverride []string
		wantOK       bool
	}{
		{
			name:         "no colon",
			input:        "contacts",
			wantBase:     "contacts",
			wantOverride: nil,
			wantOK:       false,
		},
		{
			name:         "single override",
			input:        "contacts:companies",
			wantBase:     "contacts",
			wantOverride: []string{"companies"},
			wantOK:       true,
		},
		{
			name:         "multiple overrides",
			input:        "contacts:companies,deals,tickets",
			wantBase:     "contacts",
			wantOverride: []string{"companies", "deals", "tickets"},
			wantOK:       true,
		},
		{
			name:         "empty override means no associations",
			input:        "contacts:",
			wantBase:     "contacts",
			wantOverride: []string{},
			wantOK:       true,
		},
		{
			name:         "whitespace trimmed",
			input:        "contacts: companies , deals ",
			wantBase:     "contacts",
			wantOverride: []string{"companies", "deals"},
			wantOK:       true,
		},
		{
			name:         "trailing comma",
			input:        "contacts:companies,deals,",
			wantBase:     "contacts",
			wantOverride: []string{"companies", "deals"},
			wantOK:       true,
		},
		{
			name:         "only commas",
			input:        "contacts:,,,",
			wantBase:     "contacts",
			wantOverride: []string{},
			wantOK:       true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotBase, gotOverride, gotOK := parseTableAssocOverride(tc.input)
			if gotBase != tc.wantBase {
				t.Errorf("base: got %q, want %q", gotBase, tc.wantBase)
			}
			if !reflect.DeepEqual(gotOverride, tc.wantOverride) {
				t.Errorf("override: got %#v, want %#v", gotOverride, tc.wantOverride)
			}
			if gotOK != tc.wantOK {
				t.Errorf("ok: got %v, want %v", gotOK, tc.wantOK)
			}
		})
	}
}
