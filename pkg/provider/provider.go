package provider

import (
	"context"
	"fmt"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	exov2 "github.com/exoscale/egoscale/v2"
	exov3 "github.com/exoscale/egoscale/v3"
	"github.com/exoscale/egoscale/v3/credentials"

	"github.com/exoscale/terraform-provider-exoscale/exoscale"
	"github.com/exoscale/terraform-provider-exoscale/pkg/config"
	providerConfig "github.com/exoscale/terraform-provider-exoscale/pkg/provider/config"
	"github.com/exoscale/terraform-provider-exoscale/pkg/resources/block_storage"
	"github.com/exoscale/terraform-provider-exoscale/pkg/resources/database"
	"github.com/exoscale/terraform-provider-exoscale/pkg/resources/iam"
	"github.com/exoscale/terraform-provider-exoscale/pkg/resources/nlb_service"
	"github.com/exoscale/terraform-provider-exoscale/pkg/resources/sos_bucket_policy"
	"github.com/exoscale/terraform-provider-exoscale/pkg/resources/zones"
)

const (
	KeyAttrName         = "key"
	SecretAttrName      = "secret"
	EnvironmentAttrName = "environment"
	SOSEndpointAttrName = "sos_endpoint"
	TimeoutAttrName     = "timeout"
	DelayAttrName       = "delay"
)

var _ provider.Provider = &ExoscaleProvider{}

type ExoscaleProvider struct{}

type ExoscaleProviderModel struct {
	Key         types.String  `tfsdk:"key"`
	Secret      types.String  `tfsdk:"secret"`
	Environment types.String  `tfsdk:"environment"`
	Timeout     types.Float64 `tfsdk:"timeout"`
	Delay       types.Int64   `tfsdk:"delay"`
	SOSEndpoint types.String  `tfsdk:"sos_endpoint"`
}

func (p *ExoscaleProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "exoscale"
}

func (p *ExoscaleProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			KeyAttrName: schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Exoscale API key",
			},
			SecretAttrName: schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Exoscale API secret",
			},
			EnvironmentAttrName: schema.StringAttribute{
				Optional: true,
			},
			SOSEndpointAttrName: schema.StringAttribute{
				Optional: true,
			},
			TimeoutAttrName: schema.Float64Attribute{
				Optional: true,
				MarkdownDescription: fmt.Sprintf(
					"Timeout in seconds for waiting on compute resources to become available (by default: %.0f)",
					config.DefaultTimeout.Seconds()),
			},
			DelayAttrName: schema.Int64Attribute{
				Optional:           true,
				DeprecationMessage: "Does nothing",
			},
		},
	}
}

func (p *ExoscaleProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data ExoscaleProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	var key string
	if data.Key.IsNull() {
		key = providerConfig.GetMultiEnvDefault([]string{
			"EXOSCALE_KEY",
			"EXOSCALE_API_KEY",
			"CLOUDSTACK_KEY",
			"CLOUDSTACK_API_KEY",
		}, "")
	} else {
		key = data.Key.ValueString()
	}

	var secret string
	if data.Secret.IsNull() {
		secret = providerConfig.GetMultiEnvDefault([]string{
			"EXOSCALE_SECRET",
			"EXOSCALE_SECRET_KEY",
			"EXOSCALE_API_SECRET",
			"CLOUDSTACK_SECRET",
			"CLOUDSTACK_SECRET_KEY",
		}, "")
	} else {
		secret = data.Secret.ValueString()
	}

	var environment string
	if data.Environment.IsNull() {
		environment = providerConfig.GetMultiEnvDefault([]string{
			"EXOSCALE_API_ENVIRONMENT",
		}, exoscale.DefaultEnvironment)
	} else {
		environment = data.Environment.ValueString()
	}

	var sosEndpoint string
	if data.SOSEndpoint.IsNull() {
		sosEndpoint = providerConfig.GetMultiEnvDefault([]string{
			"EXOSCALE_SOS_ENDPOINT",
			"EXOSCALE_STORAGE_API_ENDPOINT",
		}, "")
	} else {
		sosEndpoint = data.SOSEndpoint.ValueString()
	}

	var timeout float64
	if data.Timeout.IsNull() {
		var err error
		timeout, err = providerConfig.GetTimeout()

		if err != nil {
			resp.Diagnostics.AddError(err.Error(), "")
		}
	} else {
		timeout = data.Timeout.ValueFloat64()
	}

	exov2.UserAgent = exoscale.UserAgent

	baseConfig := providerConfig.BaseConfig{
		Key:         key,
		Secret:      secret,
		Timeout:     exoscale.ConvertTimeout(timeout),
		Environment: environment,
		SOSEndpoint: sosEndpoint,
	}

	clv2, err := exoscale.CreateClient(&baseConfig)
	if err != nil {
		resp.Diagnostics.AddError(err.Error(), "")

		return
	}

	_ = clv2

	// Exoscale v3 client
	creds := credentials.NewStaticCredentials(
		key,
		secret,
	)

	opts := []exov3.ClientOpt{}
	if ep := os.Getenv("EXOSCALE_API_ENDPOINT"); ep != "" {
		opts = append(opts, exov3.ClientOptWithEndpoint(exov3.Endpoint(ep)), exov3.ClientOptWithUserAgent(exoscale.UserAgent))
	}

	clv3, err := exov3.NewClient(creds, opts...)
	if err != nil {
		resp.Diagnostics.AddError(err.Error(), "unable to initialize Exoscale API V3 client")
	}

	resp.DataSourceData = &providerConfig.ExoscaleProviderConfig{
		Config:      baseConfig,
		ClientV2:    clv2,
		ClientV3:    clv3,
		Environment: environment,
		SOSEndpoint: sosEndpoint,
	}

	resp.ResourceData = &providerConfig.ExoscaleProviderConfig{
		Config:      baseConfig,
		ClientV2:    clv2,
		ClientV3:    clv3,
		Environment: environment,
		SOSEndpoint: sosEndpoint,
	}
}

func (p *ExoscaleProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		func() datasource.DataSource {
			return &zones.ZonesDataSource{}
		},
		database.NewDataSourceURI,
		iam.NewDataSourceOrgPolicy,
		iam.NewDataSourceRole,
		iam.NewDataSourceAPIKey,
		block_storage.NewDataSourceVolume,
		block_storage.NewDataSourceSnapshot,
		func() datasource.DataSource {
			return &nlb_service.NLBServiceListDataSource{}
		},
		sos_bucket_policy.NewDataSourceSOSBucketPolicy,
	}
}

func (p *ExoscaleProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		database.DeprecatedNewResource,
		database.NewServiceResource,
		database.NewMysqlUserResource,
		database.NewKafkaUserResource,
		database.NewOpensearchUserResource,
		database.NewPGUserResource,
		database.NewPGDatabaseResource,
		database.NewMysqlDatabaseResource,
		iam.NewResourceOrgPolicy,
		iam.NewResourceRole,
		iam.NewResourceAPIKey,
		block_storage.NewResourceVolume,
		block_storage.NewResourceSnapshot,
		sos_bucket_policy.NewResourceSOSBucketPolicy,
	}
}

func New() func() provider.Provider {
	return func() provider.Provider {
		return &ExoscaleProvider{}
	}
}
