// Copyright 2026 Appvia Ltd.
// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"context"
	"errors"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/appvia/ipam"
	"github.com/appvia/ipam/storage"
	"github.com/appvia/ipam/storage/memory"
)

// azuriteConnectionString is the public, well-known Azurite development
// account connection string (not a secret).
const azuriteConnectionString = "DefaultEndpointsProtocol=http;AccountName=devstoreaccount1;" +
	"AccountKey=Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw==;" +
	"BlobEndpoint=http://127.0.0.1:10000/devstoreaccount1;"

type unknown struct{}

// isolateEnv clears every variable Configure or the backends read, so tests
// never pick up the developer's credentials or backend settings.
func isolateEnv(t *testing.T) {
	t.Helper()

	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(name, "ARM_") || strings.HasPrefix(name, "AZURE_") ||
			strings.HasPrefix(name, "AWS_") || strings.HasPrefix(name, "IPAM_") ||
			strings.HasPrefix(name, "ACTIONS_ID_TOKEN_") || strings.HasPrefix(name, "SYSTEM_") {
			t.Setenv(name, "")
		}
	}

	empty := filepath.Join(t.TempDir(), "empty")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("AWS_CONFIG_FILE", empty)
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", empty)
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
}

func providerSchema(t *testing.T) schema.Schema {
	t.Helper()

	var resp provider.SchemaResponse

	(&ipamProvider{}).Schema(context.Background(), provider.SchemaRequest{}, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("schema: %v", resp.Diagnostics)
	}

	return resp.Schema
}

// configValue builds a value of typ from attrs: strings, bools, nested
// map[string]any objects and unknown{}; missing attributes are null.
func configValue(t *testing.T, typ tftypes.Type, attrs map[string]any) tftypes.Value {
	t.Helper()

	obj, ok := typ.(tftypes.Object)
	if !ok {
		t.Fatalf("type %s is not an object", typ)
	}

	values := make(map[string]tftypes.Value, len(obj.AttributeTypes))

	for name, attrType := range obj.AttributeTypes {
		raw, set := attrs[name]

		switch v := raw.(type) {
		case nil:
			if set {
				t.Fatalf("attribute %q: use omission for null", name)
			}

			values[name] = tftypes.NewValue(attrType, nil)
		case unknown:
			values[name] = tftypes.NewValue(attrType, tftypes.UnknownValue)
		case map[string]any:
			values[name] = configValue(t, attrType, v)
		default:
			values[name] = tftypes.NewValue(attrType, v)
		}
	}

	for name := range attrs {
		if _, ok := obj.AttributeTypes[name]; !ok {
			t.Fatalf("unknown attribute %q", name)
		}
	}

	return tftypes.NewValue(obj, values)
}

func configure(t *testing.T, p *ipamProvider, attrs map[string]any, deferralAllowed bool) provider.ConfigureResponse {
	t.Helper()

	ctx := context.Background()
	s := providerSchema(t)

	req := provider.ConfigureRequest{
		Config: tfsdk.Config{Schema: s, Raw: configValue(t, s.Type().TerraformType(ctx), attrs)},
	}
	req.ClientCapabilities.DeferralAllowed = deferralAllowed

	var resp provider.ConfigureResponse

	p.Configure(ctx, req, &resp)

	return resp
}

func configureOK(t *testing.T, attrs map[string]any) *Data {
	t.Helper()

	resp := configure(t, &ipamProvider{}, attrs, false)
	if resp.Diagnostics.HasError() {
		t.Fatalf("configure: %v", resp.Diagnostics)
	}

	data, ok := resp.ResourceData.(*Data)
	if !ok || data == nil || data.Client == nil {
		t.Fatalf("ResourceData = %#v, want *Data with a client", resp.ResourceData)
	}

	if resp.DataSourceData != resp.ResourceData {
		t.Fatalf("DataSourceData = %#v, want the ResourceData value", resp.DataSourceData)
	}

	return data
}

func configureErr(t *testing.T, attrs map[string]any) diag.Diagnostics {
	t.Helper()

	resp := configure(t, &ipamProvider{}, attrs, false)
	if !resp.Diagnostics.HasError() {
		t.Fatalf("configure succeeded, want an error")
	}

	if resp.ResourceData != nil || resp.DataSourceData != nil {
		t.Fatalf("failed configure set provider data")
	}

	return resp.Diagnostics
}

func diagText(diags diag.Diagnostics) string {
	var b strings.Builder

	for _, d := range diags {
		b.WriteString(d.Summary())
		b.WriteString("\n")
		b.WriteString(d.Detail())
		b.WriteString("\n")
	}

	return b.String()
}

func requireDiag(t *testing.T, diags diag.Diagnostics, want string) {
	t.Helper()

	if text := diagText(diags); !strings.Contains(text, want) {
		t.Fatalf("diagnostics %q do not contain %q", text, want)
	}
}

func requireNoLeak(t *testing.T, diags diag.Diagnostics, secrets ...string) {
	t.Helper()

	text := diagText(diags)
	for _, s := range secrets {
		if strings.Contains(text, s) {
			t.Fatalf("diagnostics leak %q: %q", s, text)
		}
	}
}

func createPool(t *testing.T, client *ipam.Client) {
	t.Helper()

	_, err := client.CreatePool(context.Background(), ipam.PoolSpec{
		ID:    "root",
		CIDRs: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/16")},
	})
	if err != nil {
		t.Fatalf("CreatePool: %v", err)
	}
}

func TestProviderSchema(t *testing.T) {
	s := providerSchema(t)

	blocks := []string{"s3", "azure_blob", "local"}
	for _, name := range blocks {
		if _, ok := s.Attributes[name].(schema.SingleNestedAttribute); !ok {
			t.Errorf("%s is not a single nested attribute", name)
		}
	}

	if _, ok := s.Attributes["access_propagation_timeout"].(schema.StringAttribute); !ok {
		t.Errorf("access_propagation_timeout is not a string attribute")
	}

	if len(s.Attributes) != 4 {
		t.Errorf("top-level attributes = %d, want 4", len(s.Attributes))
	}

	nested := func(name string) map[string]schema.Attribute {
		return s.Attributes[name].(schema.SingleNestedAttribute).Attributes
	}

	wantAzure := []string{
		"account_url", "use_connection_string_env", "container", "prefix", "tenant_id", "client_id",
		"client_id_file_path", "client_secret", "client_secret_file_path", "client_certificate",
		"client_certificate_path", "client_certificate_password", "oidc_token", "oidc_token_file_path",
		"oidc_request_token", "oidc_request_url", "oidc_azure_service_connection_id", "use_oidc", "use_msi",
		"use_cli", "use_aks_workload_identity",
	}
	wantS3 := []string{"bucket", "prefix", "region", "endpoint", "use_path_style"}
	wantLocal := []string{"path"}

	for block, want := range map[string][]string{"azure_blob": wantAzure, "s3": wantS3, "local": wantLocal} {
		var got []string
		for name := range nested(block) {
			got = append(got, name)
		}

		slices.Sort(got)
		slices.Sort(want)

		if !slices.Equal(got, want) {
			t.Errorf("%s attributes = %v, want %v", block, got, want)
		}
	}

	secrets := []string{"client_secret", "client_certificate", "client_certificate_password", "oidc_token", "oidc_request_token"}
	for name, attr := range nested("azure_blob") {
		if got, want := attr.IsSensitive(), slices.Contains(secrets, name); got != want {
			t.Errorf("azure_blob.%s sensitive = %t, want %t", name, got, want)
		}
	}

	required := map[string]map[string]bool{
		"s3":         {"bucket": true},
		"azure_blob": {"container": true},
		"local":      {"path": true},
	}
	for block, req := range required {
		for name, attr := range nested(block) {
			if attr.IsRequired() != req[name] {
				t.Errorf("%s.%s required = %t, want %t", block, name, attr.IsRequired(), req[name])
			}
		}
	}
}

func validateConfig(t *testing.T, attrs map[string]any) []*tfprotov6.Diagnostic {
	t.Helper()

	ctx := context.Background()

	server, err := providerserver.NewProtocol6WithError(New("test")())()
	if err != nil {
		t.Fatal(err)
	}

	typ := providerSchema(t).Type().TerraformType(ctx)

	dv, err := tfprotov6.NewDynamicValue(typ, configValue(t, typ, attrs))
	if err != nil {
		t.Fatal(err)
	}

	resp, err := server.ValidateProviderConfig(ctx, &tfprotov6.ValidateProviderConfigRequest{Config: &dv})
	if err != nil {
		t.Fatal(err)
	}

	return resp.Diagnostics
}

func TestProviderValidateConfigAtMostOneBackend(t *testing.T) {
	isolateEnv(t)

	s3Block := map[string]any{"bucket": "my-bucket"}
	localBlock := map[string]any{"path": "/tmp/x"}
	azureBlock := map[string]any{"container": "ipam", "account_url": "https://acct.blob.core.windows.net"}

	tests := map[string]struct {
		attrs   map[string]any
		wantErr bool
	}{
		"none":          {attrs: map[string]any{}},
		"local only":    {attrs: map[string]any{"local": localBlock}},
		"s3 only":       {attrs: map[string]any{"s3": s3Block}},
		"azure only":    {attrs: map[string]any{"azure_blob": azureBlock}},
		"s3 and local":  {attrs: map[string]any{"s3": s3Block, "local": localBlock}, wantErr: true},
		"s3 and azure":  {attrs: map[string]any{"s3": s3Block, "azure_blob": azureBlock}, wantErr: true},
		"azure, local":  {attrs: map[string]any{"azure_blob": azureBlock, "local": localBlock}, wantErr: true},
		"all three set": {attrs: map[string]any{"s3": s3Block, "azure_blob": azureBlock, "local": localBlock}, wantErr: true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			diags := validateConfig(t, tc.attrs)

			var errs []string

			for _, d := range diags {
				if d.Severity == tfprotov6.DiagnosticSeverityError {
					errs = append(errs, d.Summary+": "+d.Detail)
				}
			}

			if tc.wantErr != (len(errs) > 0) {
				t.Fatalf("errors = %v, wantErr %t", errs, tc.wantErr)
			}

			if tc.wantErr && !strings.Contains(strings.Join(errs, "\n"), "Invalid Attribute Combination") {
				t.Fatalf("errors = %v, want the attribute combination diagnostic", errs)
			}
		})
	}
}

func TestProviderConfigureRequiresBackend(t *testing.T) {
	isolateEnv(t)

	diags := configureErr(t, map[string]any{})
	requireDiag(t, diags, "one of s3, azure_blob or local must be configured, or IPAM_BACKEND set")
}

func TestProviderConfigureLocal(t *testing.T) {
	isolateEnv(t)

	dir := t.TempDir()
	data := configureOK(t, map[string]any{"local": map[string]any{"path": dir}})

	createPool(t, data.Client)

	if _, err := os.Stat(filepath.Join(dir, "pools", "root.json")); err != nil {
		t.Fatalf("pool document not written to the configured path: %v", err)
	}
}

func TestProviderConfigureEnvBackend(t *testing.T) {
	isolateEnv(t)

	dir := t.TempDir()
	t.Setenv("IPAM_BACKEND", "file://"+dir)

	data := configureOK(t, map[string]any{})
	createPool(t, data.Client)

	if _, err := os.Stat(filepath.Join(dir, "pools", "root.json")); err != nil {
		t.Fatalf("pool document not written to IPAM_BACKEND: %v", err)
	}
}

func TestProviderConfigureBlockWinsOverEnvBackend(t *testing.T) {
	isolateEnv(t)

	envDir, blockDir := t.TempDir(), t.TempDir()
	t.Setenv("IPAM_BACKEND", "file://"+envDir)

	data := configureOK(t, map[string]any{"local": map[string]any{"path": blockDir}})
	createPool(t, data.Client)

	if _, err := os.Stat(filepath.Join(blockDir, "pools", "root.json")); err != nil {
		t.Fatalf("pool not in the block's path: %v", err)
	}

	if _, err := os.Stat(filepath.Join(envDir, "pools")); !os.IsNotExist(err) {
		t.Fatalf("IPAM_BACKEND used although a block is set: %v", err)
	}
}

func TestProviderConfigureEnvBackendInvalid(t *testing.T) {
	isolateEnv(t)

	for _, raw := range []string{
		"s3://AKIAEXAMPLE:topsecretpass@bucket/prefix",
		"azblob://acct.blob.core.windows.net/ipam?sig=topsecretsig",
		"ftp://topsecrethost/x",
	} {
		t.Setenv("IPAM_BACKEND", raw)

		diags := configureErr(t, map[string]any{})
		requireDiag(t, diags, "IPAM_BACKEND")
		requireNoLeak(t, diags, "topsecret", raw)
	}
}

func TestProviderConfigureAccessPropagationTimeout(t *testing.T) {
	local := map[string]any{"path": "/tmp/unused"}

	tests := map[string]struct {
		attr, env string
		want      time.Duration
		wantErr   string
	}{
		"default":           {want: 5 * time.Minute},
		"attribute":         {attr: "90s", want: 90 * time.Second},
		"env":               {env: "2m", want: 2 * time.Minute},
		"attribute wins":    {attr: "30s", env: "2m", want: 30 * time.Second},
		"invalid attribute": {attr: "soon", wantErr: "access_propagation_timeout"},
		"invalid env":       {env: "soon", wantErr: "IPAM_ACCESS_PROPAGATION_TIMEOUT"},
		"zero":              {attr: "0s", wantErr: "greater than zero"},
		"negative":          {attr: "-1m", wantErr: "greater than zero"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			isolateEnv(t)
			local["path"] = t.TempDir()

			attrs := map[string]any{"local": local}
			if tc.attr != "" {
				attrs["access_propagation_timeout"] = tc.attr
			}

			if tc.env != "" {
				t.Setenv("IPAM_ACCESS_PROPAGATION_TIMEOUT", tc.env)
			}

			if tc.wantErr != "" {
				requireDiag(t, configureErr(t, attrs), tc.wantErr)

				return
			}

			if got := configureOK(t, attrs).AccessPropagationTimeout; got != tc.want {
				t.Fatalf("timeout = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestProviderMutationContext(t *testing.T) {
	data := &Data{AccessPropagationTimeout: time.Hour}

	ctx, cancel := data.MutationContext(context.Background())
	defer cancel()

	if !storage.AccessPropagationRetry(ctx) {
		t.Fatal("mutation context does not enable access propagation retries")
	}

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("mutation context has no deadline")
	}

	if left := time.Until(deadline); left <= 59*time.Minute || left > time.Hour {
		t.Fatalf("deadline in %s, want about 1h", left)
	}

	if storage.AccessPropagationRetry(context.Background()) {
		t.Fatal("plain contexts must not retry access denied")
	}
}

func TestProviderConfigureUnknownValues(t *testing.T) {
	isolateEnv(t)

	attrs := map[string]any{"local": map[string]any{"path": unknown{}}}

	resp := configure(t, &ipamProvider{}, attrs, false)
	requireDiag(t, resp.Diagnostics, "unknown")

	if !resp.Diagnostics.HasError() || resp.ResourceData != nil {
		t.Fatalf("unknown path without deferral: diags %v, data %v", resp.Diagnostics, resp.ResourceData)
	}

	resp = configure(t, &ipamProvider{}, attrs, true)
	if resp.Diagnostics.HasError() || resp.Deferred == nil ||
		resp.Deferred.Reason != provider.DeferredReasonProviderConfigUnknown {
		t.Fatalf("unknown path with deferral: diags %v, deferred %v", resp.Diagnostics, resp.Deferred)
	}

	resp = configure(t, &ipamProvider{}, map[string]any{"azure_blob": unknown{}}, false)
	if !resp.Diagnostics.HasError() {
		t.Fatal("unknown azure_blob block configured")
	}
}

func TestProviderConfigureS3(t *testing.T) {
	isolateEnv(t)

	configureOK(t, map[string]any{"s3": map[string]any{
		"bucket": "my-bucket", "prefix": "ipam", "region": "eu-west-2",
		"endpoint": "http://127.0.0.1:9000", "use_path_style": true,
	}})

	tests := map[string]struct {
		block   map[string]any
		want    string
		secrets []string
	}{
		"bad bucket":         {block: map[string]any{"bucket": "Bad_Bucket", "region": "eu-west-2"}, want: "bucket"},
		"ip bucket":          {block: map[string]any{"bucket": "10.0.0.1", "region": "eu-west-2"}, want: "bucket"},
		"no region":          {block: map[string]any{"bucket": "my-bucket"}, want: "region"},
		"endpoint userinfo":  {block: map[string]any{"bucket": "my-bucket", "region": "eu-west-2", "endpoint": "https://AKIA:topsecret@s3.local"}, want: "endpoint", secrets: []string{"topsecret"}},
		"endpoint query":     {block: map[string]any{"bucket": "my-bucket", "region": "eu-west-2", "endpoint": "https://s3.local/?X-Amz-Signature=topsecret"}, want: "endpoint", secrets: []string{"topsecret"}},
		"endpoint fragment":  {block: map[string]any{"bucket": "my-bucket", "region": "eu-west-2", "endpoint": "https://s3.local/#topsecret"}, want: "endpoint", secrets: []string{"topsecret"}},
		"endpoint no scheme": {block: map[string]any{"bucket": "my-bucket", "region": "eu-west-2", "endpoint": "s3.local"}, want: "endpoint"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			diags := configureErr(t, map[string]any{"s3": tc.block})
			requireDiag(t, diags, tc.want)
			requireNoLeak(t, diags, tc.secrets...)
		})
	}
}

func TestProviderConfigureAzureAccountURL(t *testing.T) {
	isolateEnv(t)

	for _, good := range []string{"https://acct123.blob.core.windows.net", "https://acct123.blob.core.windows.net/"} {
		configureOK(t, map[string]any{"azure_blob": map[string]any{"account_url": good, "container": "ipam"}})
	}

	for _, bad := range []string{
		"http://acct123.blob.core.windows.net",
		"https://evil.example.com",
		"https://acct123.blob.core.windows.net.evil.example",
		"https://acct123.evil.example/.blob.core.windows.net",
		"https://Acct123.blob.core.windows.net",
		"https://ab.blob.core.windows.net",
		"https://acct123.blob.core.windows.net:8443",
		"https://acct123.blob.core.windows.net/ipam",
		"https://acct123.blob.core.windows.net?sv=2024&sig=topsecretsig",
		"https://acct123.blob.core.windows.net/?",
		"https://acct123.blob.core.windows.net#topsecretfrag",
		"https://user:topsecretpw@acct123.blob.core.windows.net",
		" https://acct123.blob.core.windows.net",
		"acct123.blob.core.windows.net",
	} {
		t.Run(bad, func(t *testing.T) {
			diags := configureErr(t, map[string]any{"azure_blob": map[string]any{"account_url": bad, "container": "ipam"}})
			requireDiag(t, diags, "<account>.blob.core.windows.net")
			requireNoLeak(t, diags, "topsecret", bad)
		})
	}
}

func TestProviderConfigureAzureContainer(t *testing.T) {
	isolateEnv(t)

	for _, bad := range []string{"ab", "Upper", "a--b", "-ab", "has/slash", strings.Repeat("a", 64)} {
		diags := configureErr(t, map[string]any{"azure_blob": map[string]any{
			"account_url": "https://acct123.blob.core.windows.net", "container": bad,
		}})
		requireDiag(t, diags, "container")
	}
}

func TestProviderConfigureAzureEndpointChoice(t *testing.T) {
	isolateEnv(t)

	requireDiag(t, configureErr(t, map[string]any{"azure_blob": map[string]any{"container": "ipam"}}),
		"exactly one of account_url or use_connection_string_env")
	requireDiag(t, configureErr(t, map[string]any{"azure_blob": map[string]any{
		"container": "ipam", "use_connection_string_env": false,
	}}), "exactly one of account_url or use_connection_string_env")
	requireDiag(t, configureErr(t, map[string]any{"azure_blob": map[string]any{
		"container": "ipam", "use_connection_string_env": true, "account_url": "https://acct123.blob.core.windows.net",
	}}), "exactly one of account_url or use_connection_string_env")
}

func TestProviderConfigureAzureConnectionString(t *testing.T) {
	isolateEnv(t)

	block := map[string]any{"container": "ipam", "use_connection_string_env": true}

	requireDiag(t, configureErr(t, map[string]any{"azure_blob": block}), "AZURE_STORAGE_CONNECTION_STRING")

	realKey := "dG9wc2VjcmV0a2V5dG9wc2VjcmV0a2V5dG9wc2VjcmV0a2V5"
	t.Setenv("AZURE_STORAGE_CONNECTION_STRING",
		"DefaultEndpointsProtocol=https;AccountName=prod;AccountKey="+realKey+";BlobEndpoint=https://prod.blob.core.windows.net/")

	diags := configureErr(t, map[string]any{"azure_blob": block})
	requireDiag(t, diags, "loopback")
	requireNoLeak(t, diags, realKey)

	t.Setenv("AZURE_STORAGE_CONNECTION_STRING", azuriteConnectionString)
	configureOK(t, map[string]any{"azure_blob": block})
}

type failingTransport struct {
	t        *testing.T
	requests atomic.Int64
}

func (f *failingTransport) Do(req *http.Request) (*http.Response, error) {
	f.requests.Add(1)
	f.t.Errorf("unexpected network request during Configure: %s %s", req.Method, req.URL.Host)

	return nil, http.ErrHandlerTimeout
}

func (f *failingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return f.Do(req)
}

var _ policy.Transporter = (*failingTransport)(nil)

func TestProviderConfigureIsOffline(t *testing.T) {
	isolateEnv(t)

	transport := &failingTransport{t: t}

	previous := http.DefaultTransport
	http.DefaultTransport = transport

	t.Cleanup(func() { http.DefaultTransport = previous })

	p := &ipamProvider{azureTransport: transport}

	resp := configure(t, p, map[string]any{"azure_blob": map[string]any{
		"account_url": "https://acct123.blob.core.windows.net", "container": "ipam",
		"use_msi": true, "use_cli": false, "client_id": "00000000-0000-0000-0000-000000000001",
		"tenant_id": "00000000-0000-0000-0000-000000000002",
	}}, false)
	if resp.Diagnostics.HasError() {
		t.Fatalf("configure: %v", resp.Diagnostics)
	}

	if _, ok := resp.ResourceData.(*Data); !ok {
		t.Fatalf("ResourceData = %#v", resp.ResourceData)
	}

	if n := transport.requests.Load(); n != 0 {
		t.Fatalf("Configure made %d network requests, want 0", n)
	}
}

func TestProviderConfigureAzureNoAuthMethod(t *testing.T) {
	isolateEnv(t)

	diags := configureErr(t, map[string]any{"azure_blob": map[string]any{
		"account_url": "https://acct123.blob.core.windows.net", "container": "ipam", "use_cli": false,
	}})
	requireDiag(t, diags, "no Azure authentication method is enabled")
}

func TestProviderConfigureAzureSecretsNotEchoed(t *testing.T) {
	isolateEnv(t)

	secrets := map[string]any{
		"client_secret":               "topsecret-client-secret",
		"client_certificate":          "topsecret-not-base64-cert!!",
		"client_certificate_password": "topsecret-cert-password",
		"oidc_token":                  "topsecret-oidc-token",
		"oidc_request_token":          "topsecret-oidc-request-token",
	}

	block := map[string]any{
		"account_url": "https://acct123.blob.core.windows.net", "container": "ipam",
		"tenant_id": "00000000-0000-0000-0000-000000000002", "client_id": "00000000-0000-0000-0000-000000000001",
		"use_oidc": true, "oidc_request_url": "https://token.example", "use_cli": false,
	}
	for k, v := range secrets {
		block[k] = v
	}

	resp := configure(t, &ipamProvider{}, map[string]any{"azure_blob": block}, false)
	requireNoLeak(t, resp.Diagnostics, "topsecret")

	if resp.Diagnostics.HasError() {
		t.Fatalf("credentials are built lazily, configure must succeed offline: %v", resp.Diagnostics)
	}

	block["container"] = "Bad_Container"
	resp = configure(t, &ipamProvider{}, map[string]any{"azure_blob": block}, false)

	if !resp.Diagnostics.HasError() {
		t.Fatal("invalid container configured")
	}

	requireNoLeak(t, resp.Diagnostics, "topsecret")
}

type conflictingStore struct {
	*memory.Store
	puts atomic.Int64
}

func (s *conflictingStore) PutIfMatch(context.Context, string, []byte, storage.Version) (storage.Version, error) {
	s.puts.Add(1)

	return "", storage.ErrPreconditionFailed
}

func TestProviderClientRetriesUntilDeadline(t *testing.T) {
	store := &conflictingStore{Store: memory.New()}
	client := newClient(store, ipam.WithBackoff(time.Millisecond, time.Millisecond))

	createPool(t, client)

	data := &Data{Client: client, AccessPropagationTimeout: 300 * time.Millisecond}

	ctx, cancel := data.MutationContext(context.Background())
	defer cancel()

	_, err := client.Allocate(ctx, "root", ipam.AllocationSpec{PrefixLength: 24})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Allocate error = %v, want the mutation deadline", err)
	}

	if n := store.puts.Load(); n <= 8 {
		t.Fatalf("conditional writes = %d, want retries beyond the library default of 8", n)
	}
}

func TestProviderRedact(t *testing.T) {
	got := redact("bad secret abc and abc again; empty stays", "abc", "")
	if want := "bad secret <redacted> and <redacted> again; empty stays"; got != want {
		t.Fatalf("redact = %q, want %q", got, want)
	}
}
