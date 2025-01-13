package database

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	exoscale "github.com/exoscale/egoscale/v2"
	exoapi "github.com/exoscale/egoscale/v2/api"

	"github.com/exoscale/terraform-provider-exoscale/pkg/config"
	providerConfig "github.com/exoscale/terraform-provider-exoscale/pkg/provider/config"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &ServiceResource{}
var _ resource.ResourceWithImportState = &ServiceResource{}

func NewServiceResource() resource.Resource {
	return &ServiceResource{}
}

// ServiceResource defines the DBaaS Service resource implementation.
type ServiceResource struct {
	client *exoscale.Client
	env    string
}

// ServiceResourceModel describes the generic DBaaS Service resource data model.
type ServiceResourceModel struct {
	Id                    types.String `tfsdk:"id"`
	CreatedAt             types.String `tfsdk:"created_at"`
	DiskSize              types.Int64  `tfsdk:"disk_size"`
	MaintenanceDOW        types.String `tfsdk:"maintenance_dow"`
	MaintenanceTime       types.String `tfsdk:"maintenance_time"`
	Name                  types.String `tfsdk:"name"`
	NodeCPUs              types.Int64  `tfsdk:"node_cpus"`
	NodeMemory            types.Int64  `tfsdk:"node_memory"`
	Nodes                 types.Int64  `tfsdk:"nodes"`
	Plan                  types.String `tfsdk:"plan"`
	State                 types.String `tfsdk:"state"`
	CA                    types.String `tfsdk:"ca_certificate"`
	TerminationProtection types.Bool   `tfsdk:"termination_protection"`
	Type                  types.String `tfsdk:"type"`
	UpdatedAt             types.String `tfsdk:"updated_at"`
	Zone                  types.String `tfsdk:"zone"`

	Pg         *ResourcePgModel         `tfsdk:"pg"`
	Mysql      *ResourceMysqlModel      `tfsdk:"mysql"`
	Redis      *ResourceRedisModel      `tfsdk:"redis"`
	Kafka      *ResourceKafkaModel      `tfsdk:"kafka"`
	Opensearch *ResourceOpensearchModel `tfsdk:"opensearch"`
	Grafana    *ResourceGrafanaModel    `tfsdk:"grafana"`

	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (r *ServiceResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dbaas"
}

func (r *ServiceResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage Exoscale [Database Services (DBaaS)](https://community.exoscale.com/documentation/dbaas/).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The ID of this resource.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "The creation date of the database service.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"disk_size": schema.Int64Attribute{
				MarkdownDescription: "The disk size of the database service.",
				Computed:            true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"maintenance_dow": schema.StringAttribute{
				MarkdownDescription: "The day of week to perform the automated database service maintenance (`never`, `monday`, `tuesday`, `wednesday`, `thursday`, `friday`, `saturday`, `sunday`).",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{
					stringvalidator.AlsoRequires(path.Expressions{
						path.MatchRoot("maintenance_time"),
					}...),
					stringvalidator.OneOf(
						"never",
						"monday",
						"tuesday",
						"wednesday",
						"thursday",
						"friday",
						"saturday",
						"sunday",
					),
				},
			},
			"maintenance_time": schema.StringAttribute{
				MarkdownDescription: "The time of day to perform the automated database service maintenance (`HH:MM:SS`)",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{
					stringvalidator.AlsoRequires(path.Expressions{
						path.MatchRoot("maintenance_dow"),
					}...),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "❗ The name of the database service.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"node_cpus": schema.Int64Attribute{
				MarkdownDescription: "The number of CPUs of the database service.",
				Computed:            true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"node_memory": schema.Int64Attribute{
				MarkdownDescription: "The amount of memory of the database service.",
				Computed:            true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"nodes": schema.Int64Attribute{
				MarkdownDescription: "The number of nodes of the database service.",
				Computed:            true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"plan": schema.StringAttribute{
				MarkdownDescription: "The plan of the database service (use the [Exoscale CLI](https://github.com/exoscale/cli/) - `exo dbaas type show <TYPE> --plans` - for reference).",
				Required:            true,
			},
			"state": schema.StringAttribute{
				MarkdownDescription: "The current state of the database service.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"ca_certificate": schema.StringAttribute{
				MarkdownDescription: "CA Certificate required to reach a DBaaS service through a TLS-protected connection.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"termination_protection": schema.BoolAttribute{
				MarkdownDescription: "The database service protection boolean flag against termination/power-off.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "❗ The type of the database service (`kafka`, `mysql`, `opensearch`, `pg`, `redis`, `grafana`).",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf(ServicesList...),
				},
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "The date of the latest database service update.",
				Computed:            true,
			},
			"zone": schema.StringAttribute{
				MarkdownDescription: "❗ The Exoscale [Zone](https://www.exoscale.com/datacenters/) name.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf(config.Zones...),
				},
			},
		},
		Blocks: map[string]schema.Block{
			"grafana":    ResourceGrafanaSchema,
			"kafka":      ResourceKafkaSchema,
			"mysql":      ResourceMysqlSchema,
			"opensearch": ResourceOpensearchSchema,
			"pg":         ResourcePgSchema,
			"redis":      ResourceRedisSchema,
			"timeouts":   timeouts.BlockAll(ctx),
		},
		Version: 1,
	}
}

func (r *ServiceResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	r.client = req.ProviderData.(*providerConfig.ExoscaleProviderConfig).ClientV2
	r.env = req.ProviderData.(*providerConfig.ExoscaleProviderConfig).Environment
}

func (r *ServiceResource) UpgradeState(ctx context.Context) map[int64]resource.StateUpgrader {
	return map[int64]resource.StateUpgrader{
		// SDKv2 to Framework migration requires state upgrade in database blocks
		// to remove array from database blocks.
		0: {
			PriorSchema: &schema.Schema{
				Attributes: map[string]schema.Attribute{
					"id":                     schema.StringAttribute{Computed: true},
					"created_at":             schema.StringAttribute{Computed: true},
					"disk_size":              schema.Int64Attribute{Computed: true},
					"maintenance_dow":        schema.StringAttribute{Computed: true, Optional: true},
					"maintenance_time":       schema.StringAttribute{Computed: true, Optional: true},
					"name":                   schema.StringAttribute{Required: true},
					"node_cpus":              schema.Int64Attribute{Computed: true},
					"node_memory":            schema.Int64Attribute{Computed: true},
					"nodes":                  schema.Int64Attribute{Computed: true},
					"plan":                   schema.StringAttribute{Required: true},
					"state":                  schema.StringAttribute{Computed: true},
					"ca_certificate":         schema.StringAttribute{Computed: true},
					"termination_protection": schema.BoolAttribute{Computed: true, Optional: true},
					"type":                   schema.StringAttribute{Required: true},
					"updated_at":             schema.StringAttribute{Computed: true},
					"zone":                   schema.StringAttribute{Required: true},
				},
				Blocks: map[string]schema.Block{
					"grafana": schema.ListNestedBlock{
						NestedObject: schema.NestedBlockObject{
							Attributes: ResourceGrafanaSchema.Attributes,
						},
					},
					"kafka": schema.ListNestedBlock{
						NestedObject: schema.NestedBlockObject{
							Attributes: ResourceKafkaSchema.Attributes,
						},
					},
					"mysql": schema.ListNestedBlock{
						NestedObject: schema.NestedBlockObject{
							Attributes: ResourceMysqlSchema.Attributes,
						},
					},
					"opensearch": schema.ListNestedBlock{
						NestedObject: schema.NestedBlockObject{
							Attributes: ResourceOpensearchSchema.Attributes,
						},
					},
					"pg": schema.ListNestedBlock{
						NestedObject: schema.NestedBlockObject{
							Attributes: ResourcePgSchema.Attributes,
						},
					},
					"redis": schema.ListNestedBlock{
						NestedObject: schema.NestedBlockObject{
							Attributes: ResourceRedisSchema.Attributes,
						},
					},
					"timeouts": timeouts.BlockAll(ctx),
				},
			},
			StateUpgrader: func(
				ctx context.Context,
				req resource.UpgradeStateRequest,
				resp *resource.UpgradeStateResponse,
			) {
				priorState := struct {
					Id                    types.String              `tfsdk:"id"`
					CreatedAt             types.String              `tfsdk:"created_at"`
					DiskSize              types.Int64               `tfsdk:"disk_size"`
					MaintenanceDOW        types.String              `tfsdk:"maintenance_dow"`
					MaintenanceTime       types.String              `tfsdk:"maintenance_time"`
					Name                  types.String              `tfsdk:"name"`
					NodeCPUs              types.Int64               `tfsdk:"node_cpus"`
					NodeMemory            types.Int64               `tfsdk:"node_memory"`
					Nodes                 types.Int64               `tfsdk:"nodes"`
					Plan                  types.String              `tfsdk:"plan"`
					State                 types.String              `tfsdk:"state"`
					CA                    types.String              `tfsdk:"ca_certificate"`
					TerminationProtection types.Bool                `tfsdk:"termination_protection"`
					Type                  types.String              `tfsdk:"type"`
					UpdatedAt             types.String              `tfsdk:"updated_at"`
					Zone                  types.String              `tfsdk:"zone"`
					Pg                    []ResourcePgModel         `tfsdk:"pg"`
					Mysql                 []ResourceMysqlModel      `tfsdk:"mysql"`
					Redis                 []ResourceRedisModel      `tfsdk:"redis"`
					Kafka                 []ResourceKafkaModel      `tfsdk:"kafka"`
					Opensearch            []ResourceOpensearchModel `tfsdk:"opensearch"`
					Grafana               []ResourceGrafanaModel    `tfsdk:"grafana"`
					Timeouts              timeouts.Value            `tfsdk:"timeouts"`
				}{}

				resp.Diagnostics.Append(req.State.Get(ctx, &priorState)...)
				if resp.Diagnostics.HasError() {
					return
				}

				upgradedStateData := ServiceResourceModel{
					Id:                    priorState.Id,
					CreatedAt:             priorState.CreatedAt,
					DiskSize:              priorState.DiskSize,
					MaintenanceDOW:        priorState.MaintenanceDOW,
					MaintenanceTime:       priorState.MaintenanceTime,
					Name:                  priorState.Name,
					NodeCPUs:              priorState.NodeCPUs,
					NodeMemory:            priorState.NodeMemory,
					Nodes:                 priorState.Nodes,
					Plan:                  priorState.Plan,
					State:                 priorState.State,
					CA:                    priorState.CA,
					TerminationProtection: priorState.TerminationProtection,
					Type:                  priorState.Type,
					UpdatedAt:             priorState.UpdatedAt,
					Zone:                  priorState.Zone,
					Timeouts:              priorState.Timeouts,
				}
				if len(priorState.Pg) > 0 {
					upgradedStateData.Pg = &priorState.Pg[0]
				}
				if len(priorState.Mysql) > 0 {
					upgradedStateData.Mysql = &priorState.Mysql[0]
				}
				if len(priorState.Redis) > 0 {
					upgradedStateData.Redis = &priorState.Redis[0]
				}
				if len(priorState.Kafka) > 0 {
					upgradedStateData.Kafka = &priorState.Kafka[0]
				}
				if len(priorState.Opensearch) > 0 {
					upgradedStateData.Opensearch = &priorState.Opensearch[0]
				}
				if len(priorState.Grafana) > 0 {
					upgradedStateData.Grafana = &priorState.Grafana[0]
				}

				resp.Diagnostics.Append(resp.State.Set(ctx, upgradedStateData)...)
			},
		},
	}
}

func (r *ServiceResource) ValidateConfig(
	ctx context.Context,
	req resource.ValidateConfigRequest,
	resp *resource.ValidateConfigResponse,
) {
	// To prevent issues with import we require that the database block is defined for chosen database type.
	var dbType types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("type"), &dbType)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If `type` is not configured, return as attribute validator will catch error.
	if dbType.IsNull() || dbType.IsUnknown() {
		return
	}

	var dbObj types.Object
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root(dbType.ValueString()), &dbObj)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if dbObj.IsNull() || dbObj.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root(dbType.ValueString()),
			"Attribute is required",
			fmt.Sprintf("Attribute %q must be defined when database type is %q", dbType.ValueString(), dbType.ValueString()),
		)
	}
}

func (r *ServiceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ServiceResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Set timeout
	t, diags := data.Timeouts.Create(ctx, config.DefaultTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, t)
	defer cancel()

	data.Id = data.Name
	ctx = exoapi.WithEndpoint(ctx, exoapi.NewReqEndpoint(r.env, data.Zone.ValueString()))

	switch data.Type.ValueString() {
	case "pg":
		r.createPg(ctx, &data, &resp.Diagnostics)
	case "mysql":
		r.createMysql(ctx, &data, &resp.Diagnostics)
	case "redis":
		r.createRedis(ctx, &data, &resp.Diagnostics)
	case "kafka":
		r.createKafka(ctx, &data, &resp.Diagnostics)
	case "opensearch":
		r.createOpensearch(ctx, &data, &resp.Diagnostics)
	case "grafana":
		r.createGrafana(ctx, &data, &resp.Diagnostics)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)

	tflog.Trace(ctx, "resource created", map[string]interface{}{
		"id": data.Id,
	})
}

func (r *ServiceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ServiceResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
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

	data.Id = data.Name
	ctx = exoapi.WithEndpoint(ctx, exoapi.NewReqEndpoint(r.env, data.Zone.ValueString()))

	switch data.Type.ValueString() {
	case "pg":
		r.readPg(ctx, &data, &resp.Diagnostics)
	case "mysql":
		r.readMysql(ctx, &data, &resp.Diagnostics)
	case "redis":
		r.readRedis(ctx, &data, &resp.Diagnostics)
	case "kafka":
		r.readKafka(ctx, &data, &resp.Diagnostics)
	case "opensearch":
		r.readOpensearch(ctx, &data, &resp.Diagnostics)
	case "grafana":
		r.readGrafana(ctx, &data, &resp.Diagnostics)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)

	tflog.Trace(ctx, "resource read done", map[string]interface{}{
		"id": data.Id,
	})
}

func (r *ServiceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var stateData, planData ServiceResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	// Read Terraform state data (for comparison) into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Set timeout
	t, diags := stateData.Timeouts.Update(ctx, config.DefaultTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, t)
	defer cancel()

	ctx = exoapi.WithEndpoint(ctx, exoapi.NewReqEndpoint(r.env, planData.Zone.ValueString()))

	switch planData.Type.ValueString() {
	case "pg":
		r.updatePg(ctx, &stateData, &planData, &resp.Diagnostics)
	case "mysql":
		r.updateMysql(ctx, &stateData, &planData, &resp.Diagnostics)
	case "redis":
		r.updateRedis(ctx, &stateData, &planData, &resp.Diagnostics)
	case "kafka":
		r.updateKafka(ctx, &stateData, &planData, &resp.Diagnostics)
	case "opensearch":
		r.updateOpensearch(ctx, &stateData, &planData, &resp.Diagnostics)
	case "grafana":
		r.updateGrafana(ctx, &stateData, &planData, &resp.Diagnostics)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)

	tflog.Trace(ctx, "resource updated", map[string]interface{}{
		"id": planData.Id,
	})
}

func (r *ServiceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ServiceResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Set timeout
	t, diags := data.Timeouts.Delete(ctx, config.DefaultTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, t)
	defer cancel()

	ctx = exoapi.WithEndpoint(ctx, exoapi.NewReqEndpoint(r.env, data.Zone.ValueString()))

	err := r.client.DeleteDatabaseService(ctx, data.Zone.ValueString(), &exoscale.DatabaseService{Name: data.Id.ValueStringPointer()})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete database service, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "resource deleted", map[string]interface{}{
		"id": data.Id,
	})
}

func (r *ServiceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	idParts := strings.Split(req.ID, "@")

	if len(idParts) != 2 || idParts[0] == "" || idParts[1] == "" {
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected import identifier with format: name@zone. Got: %q", req.ID),
		)
		return
	}

	var data ServiceResourceModel

	// Set timeouts (quirk https://github.com/hashicorp/terraform-plugin-framework-timeouts/issues/46)
	var timeouts timeouts.Value
	resp.Diagnostics.Append(resp.State.GetAttribute(ctx, path.Root("timeouts"), &timeouts)...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Timeouts = timeouts

	data.Id = types.StringValue(idParts[0])
	data.Name = types.StringValue(idParts[0])
	data.Zone = types.StringValue(idParts[1])

	ctx = exoapi.WithEndpoint(ctx, exoapi.NewReqEndpoint(r.env, data.Zone.ValueString()))

	services, err := r.client.ListDatabaseServices(ctx, data.Zone.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list Database Services: %s", err))
		return
	}

	for _, s := range services {
		if *s.Name == data.Id.ValueString() {
			data.Type = types.StringPointerValue(s.Type)
			data.Plan = types.StringPointerValue(s.Plan)
			break
		}
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)

	tflog.Trace(ctx, "resource imported", map[string]interface{}{
		"id": data.Id,
	})
}

// We're renaming `exoscale_dbaas` to `exoscale_dbaas“
type DeprecatedServiceResource struct {
	Resource *ServiceResource
}

func (d *DeprecatedServiceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	d.Resource.Create(ctx, req, resp)
}

func (d *DeprecatedServiceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	d.Resource.Delete(ctx, req, resp)
}

func (d *DeprecatedServiceResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	d.Resource.Metadata(ctx, req, resp)
	resp.TypeName = req.ProviderTypeName + "_database"
}

func (d *DeprecatedServiceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	d.Resource.Read(ctx, req, resp)
}

func (d *DeprecatedServiceResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	d.Resource.Schema(ctx, req, resp)
	resp.Schema.DeprecationMessage = "This resource is renamed to exoscale_dbaas, reimport it after renaming it"
	resp.Schema.MarkdownDescription = "❗This resource is deprecated and renamed to exoscale_dbaas, do not use it to create new resources❗\n" + resp.Schema.MarkdownDescription
}

func (d *DeprecatedServiceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	d.Resource.Update(ctx, req, resp)
}

func (r *DeprecatedServiceResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.Resource.Configure(ctx, req, resp)
}

func (r *DeprecatedServiceResource) UpgradeState(ctx context.Context) map[int64]resource.StateUpgrader {
	return r.Resource.UpgradeState(ctx)
}

func (r *DeprecatedServiceResource) ValidateConfig(
	ctx context.Context,
	req resource.ValidateConfigRequest,
	resp *resource.ValidateConfigResponse,
) {
	r.Resource.ValidateConfig(ctx, req, resp)
}

func (r *DeprecatedServiceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	r.Resource.ImportState(ctx, req, resp)
}

func DeprecatedNewResource() resource.Resource {
	return &DeprecatedServiceResource{
		Resource: &ServiceResource{},
	}
}
