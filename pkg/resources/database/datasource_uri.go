package database

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"time"

	exoscale "github.com/exoscale/egoscale/v3"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/exoscale/terraform-provider-exoscale/pkg/config"
	providerConfig "github.com/exoscale/terraform-provider-exoscale/pkg/provider/config"
	"github.com/exoscale/terraform-provider-exoscale/pkg/utils"
)

const DataSourceURIDescription = `Fetch Exoscale [Database](https://community.exoscale.com/documentation/dbaas/) connection URI data.

This data source returns database conection details of the default (admin) user only.

URI parts are also available individually for convenience.

Corresponding resource: [exoscale_dbaas](../resources/database.md).`

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSourceWithConfigure = &DataSourceURI{}

// DataSourceURI defines the resource implementation.
type DataSourceURI struct {
	client *exoscale.Client
}

func uriWithPassword(uri string, username string, password string) (string, error) {
	if uri == "" {
		return "", fmt.Errorf("empty URI provided")
	}

	re := regexp.MustCompile(`(.*)://(.*)@(.*)`)
	matches := re.FindStringSubmatch(uri)

	if len(matches) != 4 {
		return "", fmt.Errorf("uri must contain username (format: protocol://username@some-host.com)")
	}

	return fmt.Sprintf("%s://%s:%s@%s",
		matches[1],
		username,
		password,
		matches[3]), nil
}

// NewDataSourceURI creates instance of DataSourceURI.
func NewDataSourceURI() datasource.DataSource {
	return &DataSourceURI{}
}

// DataSourceURIModel defines the data model.
type DataSourceURIModel struct {
	Id types.String `tfsdk:"id"`

	Name types.String `tfsdk:"name"`
	Type types.String `tfsdk:"type"`

	URI types.String `tfsdk:"uri"`

	// URI components for convenience
	Schema   types.String `tfsdk:"schema"`
	Host     types.String `tfsdk:"host"`
	Port     types.Int64  `tfsdk:"port"`
	Username types.String `tfsdk:"username"`
	Password types.String `tfsdk:"password"`
	DbName   types.String `tfsdk:"db_name"`

	Zone types.String `tfsdk:"zone"`

	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

// Metadata specifies resource name.
func (d *DataSourceURI) Metadata(
	ctx context.Context,
	req datasource.MetadataRequest,
	resp *datasource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_database_uri"
}

// Schema defines resource attributes.
func (d *DataSourceURI) Schema(
	ctx context.Context,
	req datasource.SchemaRequest,
	resp *datasource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: DataSourceURIDescription,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The ID of this resource.",
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of database service to match.",
				Required:            true,
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "The type of the database service (`kafka`, `mysql`, `opensearch`, `pg`, `redis`).",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.OneOf(ServicesList...),
				},
			},
			"uri": schema.StringAttribute{
				MarkdownDescription: "Database service connection URI.",
				Computed:            true,
				Sensitive:           true,
			},
			"schema": schema.StringAttribute{
				MarkdownDescription: "Database service connection schema",
				Computed:            true,
			},
			"host": schema.StringAttribute{
				MarkdownDescription: "Database service hostname",
				Computed:            true,
			},
			"port": schema.Int64Attribute{
				MarkdownDescription: "Database service port",
				Computed:            true,
			},
			"username": schema.StringAttribute{
				MarkdownDescription: "Admin user username",
				Computed:            true,
			},
			"password": schema.StringAttribute{
				MarkdownDescription: "Admin user password",
				Computed:            true,
				Sensitive:           true,
			},
			"db_name": schema.StringAttribute{
				MarkdownDescription: "Default database name",
				Computed:            true,
			},
			"zone": schema.StringAttribute{
				MarkdownDescription: "The Exoscale Zone name.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.OneOf(config.Zones...),
				},
			},
		},
		Blocks: map[string]schema.Block{
			"timeouts": timeouts.Block(ctx, timeouts.Opts{
				Read: true,
			}),
		},
	}
}

// Configure sets up data source dependencies.
func (d *DataSourceURI) Configure(
	ctx context.Context,
	req datasource.ConfigureRequest,
	resp *datasource.ConfigureResponse,
) {
	if req.ProviderData == nil {
		return
	}

	d.client = req.ProviderData.(*providerConfig.ExoscaleProviderConfig).ClientV3
}

// waitForDBService polls the database service until it reaches the RUNNING state or fails
func waitForDBAASService[T any](
	ctx context.Context,
	getService func(context.Context, string) (*T, error),
	serviceName string,
	getState func(*T) string,
) (*T, error) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

polling:
	for {
		select {
		case <-ticker.C:
			service, err := getService(ctx, serviceName)
			if err != nil {
				return nil, fmt.Errorf("error polling service status: %w", err)
			}

			state := getState(service)
			if state == string(exoscale.EnumServiceStateRunning) {
				break polling
			} else if state != string(exoscale.EnumServiceStateRebalancing) && state != string(exoscale.EnumServiceStateRebuilding) {
				return nil, fmt.Errorf("service reached unexpected state: %s", state)
			}

		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// Get final state after breaking from polling loop
	service, err := getService(ctx, serviceName)
	if err != nil {
		return nil, fmt.Errorf("error getting final service state: %w", err)
	}
	return service, nil
}

// waitForDBAASServiceReadyForUsers polls the database service until it is ready to accept user creation
func waitForDBAASServiceReadyForUsers[T any](
	ctx context.Context,
	getService func(context.Context, string) (*T, error),
	serviceName string,
	usersReady func(*T) bool,
) (*T, error) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

polling:
	for {
		select {
		case <-ticker.C:
			service, err := getService(ctx, serviceName)
			if err != nil {
				return nil, fmt.Errorf("error polling service status: %w", err)
			}

			usersReady := usersReady(service)
			if usersReady {
				break polling
			}

		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// Get final state after breaking from polling loop
	service, err := getService(ctx, serviceName)
	if err != nil {
		return nil, fmt.Errorf("error getting final service state: %w", err)
	}
	return service, nil
}

// Read defines how the data source updates Terraform's state to reflect the retrieved data.
func (d *DataSourceURI) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data DataSourceURIModel

	// Load Terraform plan into the model.
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Set timeout
	t, diags := data.Timeouts.Read(ctx, config.DefaultTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, t)
	defer cancel()

	// Use API endpoint in selected zone.
	client, err := utils.SwitchClientZone(
		ctx,
		d.client,
		exoscale.ZoneName(data.Zone.ValueString()),
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"unable to change exoscale client zone",
			err.Error(),
		)
		return
	}

	// Read remote state.
	data.Id = data.Name

	var uri string
	var params map[string]interface{}
	var user string

	switch data.Type.ValueString() {
	case "kafka":
		res, err := client.GetDBAASServiceKafka(ctx, data.Name.ValueString())
		if err != nil {
			resp.Diagnostics.AddError(
				"Client Error",
				fmt.Sprintf("Unable to read Database Service Kafka: %s", err),
			)
			return
		}

		uri = res.URI
		params = res.URIParams
	case "mysql":
		res, err := waitForDBAASService(
			ctx,
			client.GetDBAASServiceMysql,
			data.Name.ValueString(),
			func(s *exoscale.DBAASServiceMysql) string { return string(s.State) },
		)
		if err != nil {
			resp.Diagnostics.AddError(
				"Client Error",
				fmt.Sprintf("Unable to read Database Service MySQL: %s", err),
			)
			return
		}

		params = res.URIParams
		if i, ok := params["user"]; ok {
			if s, ok := i.(string); ok {
				user = s
			}
		}
		if user == "" {
			resp.Diagnostics.AddError(
				"Client Error",
				"Database Service MySQL user is empty",
			)
			return
		}
		data.Schema = types.StringValue("mysql")

		creds, err := client.RevealDBAASMysqlUserPassword(ctx, data.Name.ValueString(), user)
		if err != nil {
			resp.Diagnostics.AddError(
				"Client Error",
				fmt.Sprintf("Unable to reveal Database Service MySQL secret: %s", err),
			)
			return
		}
		uri, err = uriWithPassword(res.URI, creds.Username, creds.Password)

		if err != nil {
			resp.Diagnostics.AddError("Client Error",
				fmt.Sprintf("Unable to parse Database Service MySQL secret: %s", err),
			)
			return
		}

		params["password"] = creds.Password
	case "pg":
		res, err := waitForDBAASService(
			ctx,
			client.GetDBAASServicePG,
			data.Name.ValueString(),
			func(s *exoscale.DBAASServicePG) string { return string(s.State) },
		)
		if err != nil {
			resp.Diagnostics.AddError(
				"Client Error",
				fmt.Sprintf("Unable to read Database Service Postgres: %s", err),
			)
			return
		}

		params = res.URIParams
		if i, ok := params["user"]; ok {
			if s, ok := i.(string); ok {
				user = s
			}
		}
		if user == "" {
			resp.Diagnostics.AddError(
				"Client Error",
				"Database Service Postgres user is empty",
			)
			return
		}
		data.Schema = types.StringValue("postgres")

		creds, err := client.RevealDBAASPostgresUserPassword(ctx, data.Name.ValueString(), user)
		if err != nil {
			resp.Diagnostics.AddError(
				"Client Error",
				fmt.Sprintf("Unable to fetch Database Service Postgres: %s", err),
			)
			return
		}
		uri, err = uriWithPassword(res.URI, creds.Username, creds.Password)

		if err != nil {
			resp.Diagnostics.AddError("Client Error",
				fmt.Sprintf("Unable to get Database Service Postgres secret: %s", err),
			)
			return
		}

		params["password"] = creds.Password
	case "redis":
		res, err := waitForDBAASService(
			ctx,
			client.GetDBAASServiceRedis,
			data.Name.ValueString(),
			func(s *exoscale.DBAASServiceRedis) string { return string(s.State) },
		)
		if err != nil {
			resp.Diagnostics.AddError(
				"Client Error",
				fmt.Sprintf("Unable to read Database Service Redis: %s", err),
			)
			return
		}

		params = res.URIParams
		if i, ok := params["user"]; ok {
			if s, ok := i.(string); ok {
				user = s
			}
		}
		if user == "" {
			resp.Diagnostics.AddError(
				"Client Error",
				"Database Service Redis user is empty",
			)
			return
		}
		data.Schema = types.StringValue("rediss")

		creds, err := client.RevealDBAASRedisUserPassword(ctx, data.Name.ValueString(), user)
		if err != nil {
			resp.Diagnostics.AddError(
				"Client Error",
				fmt.Sprintf("Unable to reveal Database Service Redis secret: %s", err),
			)
			return
		}
		uri, err = uriWithPassword(res.URI, creds.Username, creds.Password)

		if err != nil {
			resp.Diagnostics.AddError("Client Error",
				fmt.Sprintf("Unable to parse Database Service Redis secret: %s", err),
			)
			return
		}
		params["password"] = creds.Password
	case "opensearch":
		res, err := waitForDBAASService(
			ctx,
			client.GetDBAASServiceOpensearch,
			data.Name.ValueString(),
			func(s *exoscale.DBAASServiceOpensearch) string { return string(s.State) },
		)
		if err != nil {
			resp.Diagnostics.AddError(
				"Client Error",
				fmt.Sprintf("Unable to read Database Service Opensearch: %s", err),
			)
			return
		}

		params = res.URIParams
		if i, ok := params["user"]; ok {
			if s, ok := i.(string); ok {
				user = s
			}
		}
		if user == "" {
			resp.Diagnostics.AddError(
				"Client Error",
				"Database Service Opensearch user is empty",
			)
			return
		}
		data.Schema = types.StringValue("https")

		creds, err := client.RevealDBAASOpensearchUserPassword(ctx, data.Name.ValueString(), user)
		if err != nil {
			resp.Diagnostics.AddError(
				"Client Error",
				fmt.Sprintf("Unable to reveal Database Service OpenSearch secret: %s", err),
			)
			return
		}

		uri, err = uriWithPassword(res.URI, creds.Username, creds.Password)
		if err != nil {
			resp.Diagnostics.AddError("Client Error",
				fmt.Sprintf("Unable to parse Database Service OpenSearch secret: %s", err),
			)
			return
		}

		params["password"] = creds.Password
	case "grafana":
		res, err := waitForDBAASService(
			ctx,
			client.GetDBAASServiceGrafana,
			data.Name.ValueString(),
			func(s *exoscale.DBAASServiceGrafana) string { return string(s.State) },
		)
		if err != nil {
			resp.Diagnostics.AddError(
				"Client Error",
				fmt.Sprintf("Unable to read Database Service Grafana: %s", err),
			)
			return
		}

		uri = res.URI
		params = res.URIParams
		if i, ok := params["user"]; ok {
			if s, ok := i.(string); ok {
				user = s
			}
		}
		if user == "" {
			resp.Diagnostics.AddError(
				"Client Error",
				"Database Service Grafana user is empty",
			)
			return
		}
		data.Schema = types.StringValue("https")

		creds, err := client.RevealDBAASGrafanaUserPassword(ctx, data.Name.ValueString(), user)
		if err != nil {
			resp.Diagnostics.AddError(
				"Client Error",
				fmt.Sprintf("Unable to reveal Database Service Grafana secret: %s", err),
			)
			return
		}

		params["password"] = creds.Password
	}

	data.URI = types.StringValue(uri)

	if i, ok := params["host"]; ok {
		if s, ok := i.(string); ok {
			data.Host = types.StringValue(s)
		}
	}
	if i, ok := params["port"]; ok {
		if s, ok := i.(string); ok {
			if n, err := strconv.ParseInt(s, 10, 64); err == nil {
				data.Port = types.Int64Value(n)
			}
		}
	}
	if i, ok := params["user"]; ok {
		if s, ok := i.(string); ok {
			data.Username = types.StringValue(s)
		}
	}
	if i, ok := params["password"]; ok {
		if s, ok := i.(string); ok {
			data.Password = types.StringValue(s)
		}
	}
	if i, ok := params["dbname"]; ok {
		if s, ok := i.(string); ok {
			data.DbName = types.StringValue(s)
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
