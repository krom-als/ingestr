package stripe

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTableName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		wantTable string
		wantMode  loadingMode
	}{
		// plain table names — async mode
		{
			name:      "simple table async",
			input:     "charge",
			wantTable: "charge",
			wantMode:  modeAsync,
		},
		{
			name:      "payment_intent alias via underscore removal",
			input:     "payment_intent",
			wantTable: "payment_intent",
			wantMode:  modeAsync,
		},
		{
			name:      "balance_transaction alias",
			input:     "balance_transaction",
			wantTable: "balance_transaction",
			wantMode:  modeAsync,
		},
		// :sync modifier
		{
			name:      "table with :sync",
			input:     "charge:sync",
			wantTable: "charge",
			wantMode:  modeSync,
		},
		{
			name:      "payment_intent:sync",
			input:     "payment_intent:sync",
			wantTable: "payment_intent",
			wantMode:  modeSync,
		},
		// :sync:incremental modifier
		{
			name:      "table with :sync:incremental",
			input:     "charge:sync:incremental",
			wantTable: "charge",
			wantMode:  modeSyncIncremental,
		},
		{
			name:      "balance_transaction:sync:incremental",
			input:     "balance_transaction:sync:incremental",
			wantTable: "balance_transaction",
			wantMode:  modeSyncIncremental,
		},
		// unknown second segment — stays async (no sync keyword)
		{
			name:      "second segment not sync stays async",
			input:     "charge:other",
			wantTable: "charge",
			wantMode:  modeAsync,
		},
		// :sync present but third part not "incremental" — stays modeSync
		{
			name:      "sync without incremental keyword",
			input:     "charge:sync:other",
			wantTable: "charge",
			wantMode:  modeSync,
		},
		// normalizeTableName alias rewriting triggered via parseTableName
		{
			name:      "checkoutsession alias",
			input:     "checkout_session",
			wantTable: "checkout_session",
			wantMode:  modeAsync,
		},
		{
			name:      "invoicelineitem alias with underscores stripped then mapped",
			input:     "invoice_line_item",
			wantTable: "invoice_line_item",
			wantMode:  modeAsync,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotTable, gotMode := parseTableName(tt.input)
			assert.Equal(t, tt.wantTable, gotTable)
			assert.Equal(t, tt.wantMode, gotMode)
		})
	}
}

func TestNormalizeTableName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		// names that are already canonical — pass through unchanged
		{"charge unchanged", "charge", "charge"},
		{"customer unchanged", "customer", "customer"},
		{"invoice unchanged", "invoice", "invoice"},
		{"event unchanged", "event", "event"},

		// underscore-free alias forms
		{"checkoutsession", "checkoutsession", "checkout_session"},
		{"paymentintent", "paymentintent", "payment_intent"},
		{"paymentlink", "paymentlink", "payment_link"},
		{"paymentmethod", "paymentmethod", "payment_method"},
		{"paymentmethoddomain", "paymentmethoddomain", "payment_method_domain"},
		{"promotioncode", "promotioncode", "promotion_code"},
		{"setupattempt", "setupattempt", "setup_attempt"},
		{"setupintent", "setupintent", "setup_intent"},
		{"shippingrate", "shippingrate", "shipping_rate"},
		{"subscriptionitem", "subscriptionitem", "subscription_item"},
		{"subscriptionschedule", "subscriptionschedule", "subscription_schedule"},
		{"taxcode", "taxcode", "tax_code"},
		{"taxid", "taxid", "tax_id"},
		{"taxrate", "taxrate", "tax_rate"},
		{"topup", "topup", "top_up"},
		{"webhookendpoint", "webhookendpoint", "webhook_endpoint"},
		{"applepaydomain", "applepaydomain", "apple_pay_domain"},
		{"applicationfee", "applicationfee", "application_fee"},
		{"balancetransaction", "balancetransaction", "balance_transaction"},
		{"creditnote", "creditnote", "credit_note"},
		{"invoiceitem", "invoiceitem", "invoice_item"},
		{"invoicelineitem", "invoicelineitem", "invoice_line_item"},

		// underscore-containing alias forms: underscores are stripped before lookup
		{"checkout_session strips and maps", "checkout_session", "checkout_session"},
		{"payment_intent strips and maps", "payment_intent", "payment_intent"},
		{"balance_transaction strips and maps", "balance_transaction", "balance_transaction"},
		{"invoice_line_item strips and maps", "invoice_line_item", "invoice_line_item"},

		// completely unknown name — returned as-is (no alias, no stripping visible)
		{"unknown name passes through", "foobar", "foobar"},
		{"unknown with underscores passes through as-is", "my_custom_table", "my_custom_table"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := normalizeTableName(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseStripeSpec_QueryForm(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		wantTable string
		wantMode  loadingMode
		wantErr   bool
	}{
		// mode=sync
		{
			name:      "mode=sync",
			input:     "charge?mode=sync",
			wantTable: "charge",
			wantMode:  modeSync,
		},
		// mode=sync + incremental=true -> sync incremental
		{
			name:      "mode=sync incremental=true",
			input:     "charge?mode=sync&incremental=true",
			wantTable: "charge",
			wantMode:  modeSyncIncremental,
		},
		// alias normalization works through query form
		{
			name:      "payment_intent alias via query form",
			input:     "payment_intent?mode=sync",
			wantTable: "payment_intent",
			wantMode:  modeSync,
		},
		// underscore-free alias via query form
		{
			name:      "paymentintent alias via query form",
			input:     "paymentintent?mode=sync",
			wantTable: "payment_intent",
			wantMode:  modeSync,
		},
		// incremental without mode=sync — still treated as async (incremental only meaningful with mode=sync)
		{
			name:      "incremental without mode is async",
			input:     "charge?incremental=true",
			wantTable: "charge",
			wantMode:  modeAsync,
		},
		// unknown key
		{
			name:    "unknown param key",
			input:   "charge?typo=sync",
			wantErr: true,
		},
		// legacy colon forms still work through parseStripeSpec (legacy path)
		{
			name:      "legacy colon sync via parseStripeSpec",
			input:     "charge:sync",
			wantTable: "charge",
			wantMode:  modeSync,
		},
		{
			name:      "legacy colon sync incremental via parseStripeSpec",
			input:     "charge:sync:incremental",
			wantTable: "charge",
			wantMode:  modeSyncIncremental,
		},
		{
			name:      "legacy plain table via parseStripeSpec",
			input:     "balance_transaction",
			wantTable: "balance_transaction",
			wantMode:  modeAsync,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotTable, gotMode, err := parseStripeSpec(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantTable, gotTable)
			assert.Equal(t, tt.wantMode, gotMode)
		})
	}
}
