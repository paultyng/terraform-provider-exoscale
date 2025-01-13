package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"

	v3 "github.com/exoscale/egoscale/v3"
	"github.com/exoscale/terraform-provider-exoscale/pkg/config"
	providerConfig "github.com/exoscale/terraform-provider-exoscale/pkg/provider/config"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

type MysqlDatabaseResource struct {
	DBResource
}

type MysqlDatabaseResourceModel struct {
	DBResourceModel
}

var _ resource.Resource = &MysqlDatabaseResource{}
var _ resource.ResourceWithImportState = &MysqlDatabaseResource{}

func NewMysqlDatabaseResource() resource.Resource {
	return &MysqlDatabaseResource{}
}

func (r *MysqlDatabaseResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*providerConfig.ExoscaleProviderConfig).ClientV3
}

// ImportState implements resource.ResourceWithImportState.
func (p *MysqlDatabaseResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)

	idParts := strings.Split(req.ID, "@")

	if len(idParts) != 2 || idParts[0] == "" || idParts[1] == "" {

		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected import identifier with format: service/database_name@zone. Got: %q", req.ID),
		)

		return
	}

	databaseID := idParts[0]
	zone := idParts[1]

	id := strings.Split(databaseID, "/")

	if len(id) != 2 || id[0] == "" || id[1] == "" {
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected import identifier with format: service/database_name@zone. Got: %q", req.ID),
		)
	}

	serviceName := id[0]
	databaseName := id[1]

	var data MysqlDatabaseResourceModel

	// Set timeouts (quirk https://github.com/hashicorp/terraform-plugin-framework-timeouts/issues/46)
	var timeouts timeouts.Value
	resp.Diagnostics.Append(resp.State.GetAttribute(ctx, path.Root("timeouts"), &timeouts)...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Timeouts = timeouts

	data.Id = types.StringValue(databaseID)
	data.DatabaseName = types.StringValue(databaseName)
	data.Service = types.StringValue(serviceName)
	data.Zone = types.StringValue(zone)

	ReadResourceForImport(ctx, req, resp, &data, p.client)
}

// Create implements resource.Resource.
func (p *MysqlDatabaseResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data MysqlDatabaseResourceModel
	CreateResource(ctx, req, resp, &data, p.client)
}

// Delete implements resource.Resource.
func (p *MysqlDatabaseResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data MysqlDatabaseResourceModel
	DeleteResource(ctx, req, resp, &data, p.client)
}

// Metadata implements resource.Resource.
func (p *MysqlDatabaseResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dbaas_mysql_database"
}

// Read implements resource.Resource.
func (p *MysqlDatabaseResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data MysqlDatabaseResourceModel
	ReadResource(ctx, req, resp, &data, p.client)

}

// Schema implements resource.Resource.
func (p *MysqlDatabaseResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{

		MarkdownDescription: "❗ Manage service database for a PostgreSQL Exoscale [Database Services (DBaaS)](https://community.exoscale.com/documentation/dbaas/).",
		Attributes: map[string]schema.Attribute{
			// Computed attributes
			"id": schema.StringAttribute{
				MarkdownDescription: "The ID of this resource, computed as service/database_name",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			// Attributes referencing the service
			"service": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "❗ The name of the database service.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
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
			// Variables
			"database_name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "❗ The name of the database for this service.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
		Blocks: map[string]schema.Block{
			"timeouts": timeouts.BlockAll(ctx),
		},
	}
}

// Update implements resource.Resource.
func (p *MysqlDatabaseResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var stateData, planData MysqlDatabaseResourceModel
	UpdateResource(ctx, req, resp, &stateData, &planData, p.client)
}

// ReadResource reads resource from remote and populate the model accordingly
func (data MysqlDatabaseResourceModel) ReadResource(ctx context.Context, client *v3.Client, diagnostics *diag.Diagnostics) {
	svc, err := client.GetDBAASServiceMysql(ctx, data.Service.ValueString())
	if err != nil {
		diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read service mysql, got error: %s", err))
		return
	}

	for _, db := range svc.Databases {
		if string(db) == data.DatabaseName.ValueString() {
			return
		}
	}
	diagnostics.AddError("Client Error", "Unable to find database for the service")
}

// CreateResource creates the resource according to the model, and then
// update computed fields if applicable
func (data MysqlDatabaseResourceModel) CreateResource(ctx context.Context, client *v3.Client, diagnostics *diag.Diagnostics) {

	createRequest := v3.CreateDBAASMysqlDatabaseRequest{
		DatabaseName: v3.DBAASDatabaseName(data.DatabaseName.ValueString()),
	}

	op, err := client.CreateDBAASMysqlDatabase(ctx, data.Service.ValueString(), createRequest)
	if err != nil {
		diagnostics.AddError(
			"Client Error",
			fmt.Sprintf("Unable to create service database, got error %s", err.Error()),
		)
		return
	}

	if _, err := client.Wait(ctx, op, v3.OperationStateSuccess); err != nil {
		diagnostics.AddError(
			"Client Error",
			fmt.Sprintf("Unable to create service database, got error %s", err.Error()),
		)
		return
	}

	svc, err := client.GetDBAASServiceMysql(ctx, data.Service.ValueString())
	if err != nil {
		diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read database service pg, got error: %s", err))
		return
	}

	for _, db := range svc.Databases {
		if string(db) == data.DatabaseName.ValueString() {
			return
		}
	}
	diagnostics.AddError("Client Error", "Unable to find newly created database for the service")
}

// DeleteResource deletes the resource
func (data MysqlDatabaseResourceModel) DeleteResource(ctx context.Context, client *v3.Client, diagnostics *diag.Diagnostics) {

	op, err := client.DeleteDBAASMysqlDatabase(ctx, data.Service.ValueString(), data.DatabaseName.ValueString())
	if err != nil {
		diagnostics.AddError(
			"Client Error",
			fmt.Sprintf("Unable to delete service database, got error %s", err.Error()),
		)
		return
	}
	_, err = client.Wait(ctx, op, v3.OperationStateSuccess)
	if err != nil {
		diagnostics.AddError(
			"Client Error",
			fmt.Sprintf("Unable to delete service database, got error %s", err.Error()),
		)
		return
	}

}

// UpdateResource updates the remote resource w/ the new model
func (MysqlDatabaseResourceModel) UpdateResource(ctx context.Context, client *v3.Client, diagnostics *diag.Diagnostics) {
	// Nothing to do as these resources are systematically recreated
}

func (data MysqlDatabaseResourceModel) WaitForService(ctx context.Context, client *v3.Client, diagnostics *diag.Diagnostics) {
	_, err := waitForDBAASServiceReadyForFn(ctx, client.GetDBAASServiceMysql, data.Service.ValueString(), func(t *v3.DBAASServiceMysql) bool { return t.State == v3.EnumServiceStateRunning })
	// DbaaS API is unstable when a service goes from rebuilding from running,
	// this wait time helps avoid that
	time.Sleep(time.Second * 10)
	if err != nil {
		diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read Database service Mysql %s", err.Error()))
	}
}

// Accessing and setting attributes
func (data *MysqlDatabaseResourceModel) GetTimeouts() timeouts.Value {
	return data.Timeouts
}

func (data *MysqlDatabaseResourceModel) SetTimeouts(t timeouts.Value) {
	data.Timeouts = t
}

func (data *MysqlDatabaseResourceModel) GetID() basetypes.StringValue {
	return data.Id
}

func (data *MysqlDatabaseResourceModel) GetZone() basetypes.StringValue {
	return data.Zone
}

// Should set the return value of .GetID() to service/database_name
func (data *MysqlDatabaseResourceModel) GenerateID() {
	data.Id = basetypes.NewStringValue(fmt.Sprintf("%s/%s", data.Service.ValueString(), data.DatabaseName.ValueString()))
}
