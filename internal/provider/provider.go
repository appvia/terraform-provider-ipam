// Copyright 2026 Appvia Ltd.
// SPDX-License-Identifier: Apache-2.0

// Package provider implements the IPAM Terraform provider.
package provider

import (
	"context"
	"net"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/appvia/ipam"
	"github.com/appvia/ipam/storage"
	"github.com/appvia/ipam/storage/azblob"
	"github.com/appvia/ipam/storage/local"
	"github.com/appvia/ipam/storage/s3"
)

const (
	envBackend                  = "IPAM_BACKEND"
	envAccessPropagationTimeout = "IPAM_ACCESS_PROPAGATION_TIMEOUT"
	envConnectionString         = "AZURE_STORAGE_CONNECTION_STRING"
	defaultPropagationTimeout   = "5m"
	redacted                    = "<redacted>"
)

var (
	s3BucketPattern       = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`)
	azureAccountHost      = regexp.MustCompile(`^[a-z0-9]{3,24}\.blob\.core\.windows\.net$`)
	azureContainerPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
)

var _ provider.Provider = &ipamProvider{}

type ipamProvider struct {
	version string
	// azureTransport replaces the Azure Blob HTTP transport in tests.
	azureTransport policy.Transporter
}

// Data is the provider data handed to resources and data sources.
type Data struct {
	// Client retries conditional-write conflicts until its context ends, so
	// every mutating call must run under MutationContext.
	Client *ipam.Client
	// AccessPropagationTimeout bounds Create, Update and Delete.
	AccessPropagationTimeout time.Duration
}

// MutationContext returns the context for Create, Update and Delete: access
// denied is retried while new role assignments propagate, and the whole
// operation is bounded by AccessPropagationTimeout. Read and data sources must
// use their plain context so access denied fails fast.
func (d *Data) MutationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(storage.WithAccessPropagationRetry(ctx), d.AccessPropagationTimeout)
}

type providerModel struct {
	S3                       *s3Model        `tfsdk:"s3"`
	AzureBlob                *azureBlobModel `tfsdk:"azure_blob"`
	Local                    *localModel     `tfsdk:"local"`
	AccessPropagationTimeout types.String    `tfsdk:"access_propagation_timeout"`
}

type s3Model struct {
	Bucket       types.String `tfsdk:"bucket"`
	Prefix       types.String `tfsdk:"prefix"`
	Region       types.String `tfsdk:"region"`
	Endpoint     types.String `tfsdk:"endpoint"`
	UsePathStyle types.Bool   `tfsdk:"use_path_style"`
}

type azureBlobModel struct {
	AccountURL                   types.String `tfsdk:"account_url"`
	UseConnectionStringEnv       types.Bool   `tfsdk:"use_connection_string_env"`
	Container                    types.String `tfsdk:"container"`
	Prefix                       types.String `tfsdk:"prefix"`
	TenantID                     types.String `tfsdk:"tenant_id"`
	ClientID                     types.String `tfsdk:"client_id"`
	ClientIDFilePath             types.String `tfsdk:"client_id_file_path"`
	ClientSecret                 types.String `tfsdk:"client_secret"`
	ClientSecretFilePath         types.String `tfsdk:"client_secret_file_path"`
	ClientCertificate            types.String `tfsdk:"client_certificate"`
	ClientCertificatePath        types.String `tfsdk:"client_certificate_path"`
	ClientCertificatePassword    types.String `tfsdk:"client_certificate_password"`
	OIDCToken                    types.String `tfsdk:"oidc_token"`
	OIDCTokenFilePath            types.String `tfsdk:"oidc_token_file_path"`
	OIDCRequestToken             types.String `tfsdk:"oidc_request_token"`
	OIDCRequestURL               types.String `tfsdk:"oidc_request_url"`
	OIDCAzureServiceConnectionID types.String `tfsdk:"oidc_azure_service_connection_id"`
	UseOIDC                      types.Bool   `tfsdk:"use_oidc"`
	UseMSI                       types.Bool   `tfsdk:"use_msi"`
	UseCLI                       types.Bool   `tfsdk:"use_cli"`
	UseAKSWorkloadIdentity       types.Bool   `tfsdk:"use_aks_workload_identity"`
}

type localModel struct {
	Path types.String `tfsdk:"path"`
}

// New returns a factory for the provider at the given version.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &ipamProvider{version: version}
	}
}

func (p *ipamProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "ipam"
	resp.Version = p.version
}

func conflictsWithOthers(name string) []validator.Object {
	var others []path.Expression

	for _, other := range []string{"s3", "azure_blob", "local"} {
		if other != name {
			others = append(others, path.MatchRoot(other))
		}
	}

	return []validator.Object{objectvalidator.ConflictsWith(others...)}
}

func optionalString(description string) schema.StringAttribute {
	return schema.StringAttribute{Optional: true, Description: description}
}

func sensitiveString(description string) schema.StringAttribute {
	return schema.StringAttribute{Optional: true, Sensitive: true, Description: description}
}

func optionalBool(description string) schema.BoolAttribute {
	return schema.BoolAttribute{Optional: true, Description: description}
}

func (p *ipamProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Allocates non-overlapping IP ranges from pools stored in shared object storage. " +
			"Configure at most one of `s3`, `azure_blob` or `local`; when none is set, the " + envBackend +
			" environment variable must hold a backend URL. Configure performs no network calls.",
		Attributes: map[string]schema.Attribute{
			"s3": schema.SingleNestedAttribute{
				Optional:    true,
				Description: "Amazon S3 (or S3-compatible) backend. Credentials come from the AWS SDK default chain.",
				Validators:  conflictsWithOthers("s3"),
				Attributes: map[string]schema.Attribute{
					"bucket":         schema.StringAttribute{Required: true, Description: "Bucket name."},
					"prefix":         optionalString("Key prefix under which pool documents are stored."),
					"region":         optionalString("Bucket region. Defaults to the AWS SDK environment (AWS_REGION, shared config)."),
					"endpoint":       optionalString("Custom http(s) endpoint for S3-compatible servers."),
					"use_path_style": optionalBool("Use path-style addressing."),
				},
			},
			"azure_blob": schema.SingleNestedAttribute{
				Optional: true,
				Description: "Azure Blob Storage backend (flat namespace accounts only). Authentication attributes " +
					"and their ARM_* environment fallbacks match the azapi provider.",
				Validators: conflictsWithOthers("azure_blob"),
				Attributes: map[string]schema.Attribute{
					"account_url": optionalString("Blob service endpoint, https://<account>.blob.core.windows.net. " +
						"Exactly one of account_url or use_connection_string_env must be set."),
					"use_connection_string_env": optionalBool("Use shared-key auth from " + envConnectionString +
						". Accepted only for the Azurite development account on a loopback endpoint; for tests " +
						"and local development."),
					"container":                        schema.StringAttribute{Required: true, Description: "Existing container name."},
					"prefix":                           optionalString("Virtual directory under which pool documents are stored."),
					"tenant_id":                        optionalString("Tenant ID. Env: ARM_TENANT_ID."),
					"client_id":                        optionalString("Client ID. Env: ARM_CLIENT_ID."),
					"client_id_file_path":              optionalString("File containing the client ID. Env: ARM_CLIENT_ID_FILE_PATH."),
					"client_secret":                    sensitiveString("Client secret. Env: ARM_CLIENT_SECRET."),
					"client_secret_file_path":          optionalString("File containing the client secret. Env: ARM_CLIENT_SECRET_FILE_PATH."),
					"client_certificate":               sensitiveString("Base64-encoded PKCS#12 client certificate. Env: ARM_CLIENT_CERTIFICATE."),
					"client_certificate_path":          optionalString("Path to a PKCS#12 client certificate. Env: ARM_CLIENT_CERTIFICATE_PATH."),
					"client_certificate_password":      sensitiveString("Client certificate password. Env: ARM_CLIENT_CERTIFICATE_PASSWORD."),
					"oidc_token":                       sensitiveString("OIDC ID token. Env: ARM_OIDC_TOKEN."),
					"oidc_token_file_path":             optionalString("File containing an OIDC ID token. Env: ARM_OIDC_TOKEN_FILE_PATH."),
					"oidc_request_token":               sensitiveString("Bearer token for the OIDC token request. Env: ARM_OIDC_REQUEST_TOKEN, ACTIONS_ID_TOKEN_REQUEST_TOKEN."),
					"oidc_request_url":                 optionalString("OIDC token request URL. Env: ARM_OIDC_REQUEST_URL, ACTIONS_ID_TOKEN_REQUEST_URL."),
					"oidc_azure_service_connection_id": optionalString("Azure DevOps service connection ID. Env: ARM_ADO_PIPELINE_SERVICE_CONNECTION_ID, ARM_OIDC_AZURE_SERVICE_CONNECTION_ID."),
					"use_oidc":                         optionalBool("Use OIDC authentication. Env: ARM_USE_OIDC."),
					"use_msi":                          optionalBool("Use managed identity authentication. Env: ARM_USE_MSI."),
					"use_cli":                          optionalBool("Use Azure CLI authentication. Env: ARM_USE_CLI (default true)."),
					"use_aks_workload_identity":        optionalBool("Use AKS workload identity. Env: ARM_USE_AKS_WORKLOAD_IDENTITY."),
				},
			},
			"local": schema.SingleNestedAttribute{
				Optional:    true,
				Description: "Directory on a local disk (development and single-host use only; never a network filesystem).",
				Validators:  conflictsWithOthers("local"),
				Attributes: map[string]schema.Attribute{
					"path": schema.StringAttribute{Required: true, Description: "Directory path; created when missing."},
				},
			},
			"access_propagation_timeout": optionalString("How long Create, Update and Delete keep retrying access " +
				"denied while new role assignments propagate; also bounds those operations. Go duration, default \"" +
				defaultPropagationTimeout + "\". Env: " + envAccessPropagationTimeout + "."),
		},
	}
}

func (p *ipamProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	if unknown := unknownPaths(req.Config.Raw); len(unknown) > 0 {
		if req.ClientCapabilities.DeferralAllowed {
			resp.Deferred = &provider.Deferred{Reason: provider.DeferredReasonProviderConfigUnknown}

			return
		}

		resp.Diagnostics.AddError("Provider configuration is unknown",
			"The ipam provider cannot be configured because "+strings.Join(unknown, ", ")+
				" is unknown until apply. Use values known at plan time (for example literals, variables or "+
				"outputs of an already applied state), or apply the resources they depend on first.")

		return
	}

	var model providerModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &model)...)

	if resp.Diagnostics.HasError() {
		return
	}

	timeout, diags := propagationTimeout(model.AccessPropagationTimeout)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	store, diags := p.openStorage(ctx, model)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	data := &Data{
		Client:                   newClient(store),
		AccessPropagationTimeout: timeout,
	}
	resp.ResourceData = data
	resp.DataSourceData = data
}

// newClient retries conflicting conditional writes until the context ends;
// MutationContext supplies the deadline.
func newClient(store storage.Storage, opts ...ipam.Option) *ipam.Client {
	return ipam.New(store, append([]ipam.Option{ipam.WithMaxAttempts(0)}, opts...)...)
}

func (p *ipamProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{NewPoolResource, NewAllocationResource}
}

func (p *ipamProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{NewPoolDataSource, NewAllocationDataSource}
}

func unknownPaths(raw tftypes.Value) []string {
	var paths []string

	_ = tftypes.Walk(raw, func(p *tftypes.AttributePath, v tftypes.Value) (bool, error) {
		if !v.IsKnown() {
			paths = append(paths, attributePathString(p))

			return false, nil
		}

		return true, nil
	})

	return paths
}

func attributePathString(p *tftypes.AttributePath) string {
	var parts []string

	for _, step := range p.Steps() {
		if name, ok := step.(tftypes.AttributeName); ok {
			parts = append(parts, string(name))
		}
	}

	return strings.Join(parts, ".")
}

func propagationTimeout(attr types.String) (time.Duration, diag.Diagnostics) {
	var diags diag.Diagnostics

	raw, source := defaultPropagationTimeout, "access_propagation_timeout"

	switch {
	case !attr.IsNull():
		raw = attr.ValueString()
	case os.Getenv(envAccessPropagationTimeout) != "":
		raw, source = os.Getenv(envAccessPropagationTimeout), envAccessPropagationTimeout
	}

	timeout, err := time.ParseDuration(raw)

	switch {
	case err != nil:
		diags.AddAttributeError(path.Root("access_propagation_timeout"), "Invalid "+source,
			source+" must be a Go duration such as \"5m\" or \"90s\".")
	case timeout <= 0:
		diags.AddAttributeError(path.Root("access_propagation_timeout"), "Invalid "+source,
			source+" must be greater than zero.")
	}

	return timeout, diags
}

func (p *ipamProvider) openStorage(ctx context.Context, model providerModel) (storage.Storage, diag.Diagnostics) {
	var diags diag.Diagnostics

	switch {
	case model.S3 != nil:
		return openS3(ctx, *model.S3)
	case model.AzureBlob != nil:
		return p.openAzure(ctx, *model.AzureBlob)
	case model.Local != nil:
		store, err := local.New(model.Local.Path.ValueString())
		if err != nil {
			diags.AddAttributeError(path.Root("local").AtName("path"), "Invalid local backend", err.Error())

			return nil, diags
		}

		return store, diags
	}

	raw := os.Getenv(envBackend)
	if raw == "" {
		diags.AddError("No backend configured", "one of s3, azure_blob or local must be configured, or "+envBackend+" set.")

		return nil, diags
	}

	store, err := ipam.OpenStorage(ctx, raw)
	if err != nil {
		diags.AddError("Invalid "+envBackend, envBackend+" is not a usable backend URL: "+redact(err.Error(), raw))

		return nil, diags
	}

	return store, diags
}

func openS3(ctx context.Context, m s3Model) (storage.Storage, diag.Diagnostics) {
	var diags diag.Diagnostics

	block := path.Root("s3")
	bucket, endpoint := m.Bucket.ValueString(), m.Endpoint.ValueString()

	if !s3BucketPattern.MatchString(bucket) || net.ParseIP(bucket) != nil {
		diags.AddAttributeError(block.AtName("bucket"), "Invalid s3 bucket",
			"bucket must be a valid S3 bucket name (3-63 lowercase letters, digits, dots or hyphens).")
	}

	if endpoint != "" && !validS3Endpoint(endpoint) {
		diags.AddAttributeError(block.AtName("endpoint"), "Invalid s3 endpoint",
			"endpoint must be an http or https URL without user info, query or fragment.")
	}

	if diags.HasError() {
		return nil, diags
	}

	store, err := s3.New(ctx, s3.Config{
		Bucket:       bucket,
		Prefix:       m.Prefix.ValueString(),
		Region:       m.Region.ValueString(),
		Endpoint:     endpoint,
		UsePathStyle: m.UsePathStyle.ValueBool(),
	})
	if err != nil {
		diags.AddAttributeError(block, "Invalid s3 backend", err.Error())

		return nil, diags
	}

	return store, diags
}

func validS3Endpoint(raw string) bool {
	u, err := url.Parse(raw)

	return err == nil && !strings.Contains(raw, "#") && (u.Scheme == "http" || u.Scheme == "https") &&
		u.Host != "" && u.User == nil && u.RawQuery == "" && !u.ForceQuery && u.Opaque == ""
}

// azureAccountURL enforces https://<account>.blob.core.windows.net itself:
// azblob.New accepts any https host, so an unchecked value could send Azure
// AD tokens for the storage audience to an arbitrary server.
func azureAccountURL(raw string) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Opaque != "" || u.RawQuery != "" || u.ForceQuery ||
		u.Fragment != "" || strings.Contains(raw, "#") || (u.Path != "" && u.Path != "/") || u.RawPath != "" ||
		!azureAccountHost.MatchString(u.Host) {
		return "", false
	}

	return "https://" + u.Host, true
}

func (p *ipamProvider) openAzure(ctx context.Context, m azureBlobModel) (storage.Storage, diag.Diagnostics) {
	var diags diag.Diagnostics

	block := path.Root("azure_blob")
	rawURL := m.AccountURL.ValueString()
	useConn := m.UseConnectionStringEnv.ValueBool()

	if (rawURL != "") == useConn {
		diags.AddAttributeError(block, "Invalid azure_blob backend",
			"exactly one of account_url or use_connection_string_env = true must be set.")
	}

	if useConn && rawURL == "" && os.Getenv(envConnectionString) == "" {
		diags.AddAttributeError(block.AtName("use_connection_string_env"), "Invalid azure_blob backend",
			"use_connection_string_env requires "+envConnectionString+" to be set.")
	}

	container := m.Container.ValueString()
	if len(container) < 3 || len(container) > 63 || !azureContainerPattern.MatchString(container) {
		diags.AddAttributeError(block.AtName("container"), "Invalid azure_blob container",
			"container must be 3-63 lowercase letters, digits or single hyphens.")
	}

	cfg := azblob.Config{Container: container, Prefix: m.Prefix.ValueString(), Transport: p.azureTransport}

	auth := azblob.AuthConfigFromEnv().Merge(m.authConfig())
	secrets := []string{
		auth.ClientSecret, auth.ClientCertificate, auth.ClientCertificatePassword, auth.OIDCToken,
		auth.OIDCRequestToken, os.Getenv(envConnectionString),
	}

	if rawURL != "" && !useConn {
		accountURL, ok := azureAccountURL(rawURL)
		if !ok {
			diags.AddAttributeError(block.AtName("account_url"), "Invalid azure_blob account_url",
				"account_url must be https://<account>.blob.core.windows.net with no path, port, query, "+
					"fragment or user info.")
		}

		if diags.HasError() {
			return nil, diags
		}

		cred, err := auth.Credential(nil)
		if err != nil {
			diags.AddAttributeError(block, "Invalid Azure authentication", redact(err.Error(), secrets...))

			return nil, diags
		}

		cfg.AccountURL, cfg.Credential = accountURL, cred
	}

	if diags.HasError() {
		return nil, diags
	}

	store, err := azblob.New(ctx, cfg)
	if err != nil {
		diags.AddAttributeError(block, "Invalid azure_blob backend", redact(err.Error(), secrets...))

		return nil, diags
	}

	return store, diags
}

func (m azureBlobModel) authConfig() azblob.AuthConfig {
	return azblob.AuthConfig{
		TenantID:                     m.TenantID.ValueString(),
		ClientID:                     m.ClientID.ValueString(),
		ClientIDFilePath:             m.ClientIDFilePath.ValueString(),
		ClientSecret:                 m.ClientSecret.ValueString(),
		ClientSecretFilePath:         m.ClientSecretFilePath.ValueString(),
		ClientCertificate:            m.ClientCertificate.ValueString(),
		ClientCertificatePath:        m.ClientCertificatePath.ValueString(),
		ClientCertificatePassword:    m.ClientCertificatePassword.ValueString(),
		OIDCToken:                    m.OIDCToken.ValueString(),
		OIDCTokenFilePath:            m.OIDCTokenFilePath.ValueString(),
		OIDCRequestToken:             m.OIDCRequestToken.ValueString(),
		OIDCRequestURL:               m.OIDCRequestURL.ValueString(),
		OIDCAzureServiceConnectionID: m.OIDCAzureServiceConnectionID.ValueString(),
		UseOIDC:                      boolPointer(m.UseOIDC),
		UseMSI:                       boolPointer(m.UseMSI),
		UseCLI:                       boolPointer(m.UseCLI),
		UseAKSWorkloadIdentity:       boolPointer(m.UseAKSWorkloadIdentity),
	}
}

// boolPointer keeps "not set" distinct from false so an explicit false
// overrides an ARM_* environment default in AuthConfig.Merge.
func boolPointer(b types.Bool) *bool {
	if b.IsNull() {
		return nil
	}

	v := b.ValueBool()

	return &v
}

func redact(msg string, secrets ...string) string {
	for _, s := range secrets {
		if s != "" {
			msg = strings.ReplaceAll(msg, s, redacted)
		}
	}

	return msg
}
