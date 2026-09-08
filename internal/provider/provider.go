package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	deployments "github.com/F5Networks/terraform-provider-f5ads/internal/provider/clients/deployments/2026-07-31"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ provider.Provider = &nginxaasProvider{}
)

const (
	deploymentsAPIVersion = "api/2026-07-31"
	authAPIVersion        = "api/v1"
	namespace             = "default"
	apiDomain             = "api.nginxaas.net"
)

// New is a helper function to simplify provider server and testing implementation.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &nginxaasProvider{
			version: version,
		}
	}
}

// nginxaasProvider is the provider implementation.
type nginxaasProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

type nginxaasProviderModel struct {
	Geo          types.String `tfsdk:"geo"`
	ClientID     types.String `tfsdk:"client_id"`
	ClientSecret types.String `tfsdk:"client_secret"`
}

// Metadata returns the provider type name.
func (p *nginxaasProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "f5ads"
	resp.Version = p.version
}

// Schema defines the provider-level schema for configuration data.
func (p *nginxaasProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "The F5 Application Delivery Service (F5 ADS) provider manages F5 ADS deployments, configurations, and certificates. " +
			"Configure the provider with a geography and client credentials to manage resources.",
		Attributes: map[string]schema.Attribute{
			"geo": schema.StringAttribute{
				Optional:    true,
				Description: "API endpoint of the F5 ADS Geography (e.g. \"us\", \"eu\"). Can also be set with the F5ADS_GEO environment variable.",
				Validators: []validator.String{
					stringvalidator.OneOf("us", "eu", "apac"),
				},
			},
			"client_id": schema.StringAttribute{
				Optional:    true,
				Description: "Client ID for authenticating with the F5 ADS API. Can also be set with the F5ADS_CLIENT_ID environment variable.",
			},
			"client_secret": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Client secret for authenticating with the F5 ADS API. Can also be set with the F5ADS_CLIENT_SECRET environment variable.",
			},
		},
	}
}

// Configure prepares a F5 ADS API client for data sources and resources.
func (p *nginxaasProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	tflog.Info(ctx, "Configuring F5 ADS client")

	// Retrieve provider data from configuration
	var config nginxaasProviderModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If the practitioner provides configuration for any of the
	// attributes, it must be a known value.

	if config.Geo.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("geo"),
			"Unknown F5 ADS API geo",
			"The provider cannot create the F5 ADS API client as there is an unknown configuration value for the F5 ADS API geo. "+
				"Either apply the target source of the value first, set the value statically in the provider configuration, or use the F5ADS_GEO environment variable.",
		)
	}

	if config.ClientID.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("client_id"),
			"Unknown F5 ADS Client ID",
			"The provider cannot create the F5 ADS API client as there is an unknown configuration value for the F5 ADS API client ID. "+
				"Either apply the target source of the value first, set the value statically in the provider configuration, or use the F5ADS_CLIENT_ID environment variable.",
		)
	}

	if config.ClientSecret.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("client_secret"),
			"Unknown F5 ADS Client Secret",
			"The provider cannot create the F5 ADS API client as there is an unknown configuration value for the F5 ADS API client secret. "+
				"Either apply the target source of the value first, set the value statically in the provider configuration, or use the F5ADS_CLIENT_SECRET environment variable.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	geo := os.Getenv("F5ADS_GEO")
	clientID := os.Getenv("F5ADS_CLIENT_ID")
	clientSecret := os.Getenv("F5ADS_CLIENT_SECRET")

	if !config.Geo.IsNull() {
		geo = config.Geo.ValueString()
	}

	if !config.ClientID.IsNull() {
		clientID = config.ClientID.ValueString()
	}

	if !config.ClientSecret.IsNull() {
		clientSecret = config.ClientSecret.ValueString()
	}

	// If any of the expected configuration values are missing, return errors
	// with provider specific guidance.

	if geo == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("geo"),
			"Missing F5 ADS API geo",
			"The provider cannot create F5 ADS API client as there is a missing or empty value for the F5 ADS API geo. "+
				"Set the geo value in the configuration or use the F5ADS_GEO environment variable."+
				"If either is already set, ensure the value is not empty.",
		)
	}

	if clientID == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("client_id"),
			"Missing F5 ADS client ID",
			"The provider cannot create F5 ADS API client as there is a missing or empty value for the F5 ADS client ID. "+
				"Set the client_id value in the configuration or use the F5ADS_CLIENT_ID environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
	}

	if clientSecret == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("client_secret"),
			"Missing F5 ADS client secret",
			"The provider cannot create F5 ADS API client as there is a missing or empty value for the F5 ADS client secret. "+
				"Set the client_secret value in the configuration or use the F5ADS_CLIENT_SECRET environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "f5ads_geo", geo)
	ctx = tflog.SetField(ctx, "f5ads_client_id", clientID)
	ctx = tflog.SetField(ctx, "f5ads_client_secret", clientSecret)
	ctx = tflog.MaskFieldValuesWithFieldKeys(ctx, "f5ads_client_secret")

	tflog.Debug(ctx, "Creating F5 ADS client")

	// Create a new F5 ADS API client using the configuration values.
	// This could be a set of clients, i.e., deployments, certs, configs.
	baseURL, err := url.Parse(fmt.Sprintf("https://%s.%s", geo, apiDomain))
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create F5 ADS API client",
			"An unexpected error occurred when creating the F5 ADS API client. "+
				"If the error is not clear, please report the issue to the provider developers.\n\n"+
				"F5 ADS API client error: "+err.Error(),
		)
		return
	}

	token, err := exchangeClientCredentialsForToken(ctx, baseURL, clientID, clientSecret)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to authenticate with F5 ADS API",
			"An unexpected error occurred when exchanging client credentials for an access token.\n\n"+
				"F5 ADS API authentication error: "+err.Error(),
		)
		return
	}

	deploymentsHost := baseURL.JoinPath(deploymentsAPIVersion, "namespaces", namespace)

	client, err := deployments.NewClientWithResponses(
		deploymentsHost.String(),
		deployments.WithRequestEditorFn(
			func(ctx context.Context, req *http.Request) error {
				req.Header.Add("Authorization", "Bearer "+token)
				return nil
			},
		),
	)

	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create F5 ADS API client",
			"An unexpected error occurred when creating the F5 ADS API client. "+
				"If the error is not clear, please report the issue to the provider developers.\n\n"+
				"F5 ADS API client error: "+err.Error(),
		)
		return
	}

	// Make the F5 ADS API client available during DataSource and Resource type
	// Configure methods.
	resp.DataSourceData = client
	resp.ResourceData = client

	tflog.Info(ctx, "Configured F5 ADS client", map[string]any{"success": true})
}

// exchangeClientCredentialsForToken exchanges a customer's F5 ADS client
// credentials for a short-lived F5 ADS access token.
func exchangeClientCredentialsForToken(
	ctx context.Context,
	baseURL *url.URL,
	clientID string,
	clientSecret string,
) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	authBaseURL := baseURL.JoinPath(authAPIVersion, "auth", "token")

	data, err := json.Marshal(map[string]string{
		"client_id":     clientID,
		"client_secret": clientSecret,
		"grant_type":    "client_credentials",
	})
	if err != nil {
		return "", fmt.Errorf("unable to marshal client credentials request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, authBaseURL.String(), bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("unable to create auth token request: %w", err)
	}
	req.Header.Add("Content-Type", "application/json")

	client := http.DefaultClient
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("unable to execute auth token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected auth token response status %d", resp.StatusCode)
	}

	// Limit to 8KB. The token response is small but in case the response footprint
	// increases in the future, we have enough wiggle room to accommodate it.
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024))
	if err != nil {
		return "", fmt.Errorf("unable to read auth token response body: %w", err)
	}

	type tokenResponse struct {
		AccessToken string `json:"access_token"`
	}

	var tr tokenResponse
	err = json.Unmarshal(respBody, &tr)
	if err != nil {
		return "", fmt.Errorf("unable to unmarshal auth token response body: %w", err)
	}

	if tr.AccessToken == "" {
		return "", fmt.Errorf("auth token response was missing access_token")
	}

	return tr.AccessToken, nil
}

// DataSources defines the data sources implemented in the provider.
func (p *nginxaasProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewDeploymentDataSource,
	}
}

// Resources defines the resources implemented in the provider.
func (p *nginxaasProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewDeploymentResource,
	}
}
