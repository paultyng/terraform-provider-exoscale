package database

import (
	"context"
	"fmt"
	"strings"

	exoscale "github.com/exoscale/egoscale/v3"
	providerConfig "github.com/exoscale/terraform-provider-exoscale/pkg/provider/config"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

var _ resource.Resource = &MysqlUserResource{}
var _ resource.ResourceWithImportState = &MysqlUserResource{}

func NewMysqlUserResource() resource.Resource {
	return &MysqlUserResource{}
}

type MysqlUserResource struct {
	UserResource
}

type MysqlUserResourceModel struct {
	UserResourceModel
	Authentication types.String `tfsdk:"authentication"`
}

func (r *MysqlUserResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*providerConfig.ExoscaleProviderConfig).ClientV3
}

func (r *MysqlUserResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dbaas_mysql_user"
}

func (r *MysqlUserResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Manage service users for MySQL Exoscale [Database Services (DBaaS)](https://community.exoscale.com/documentation/dbaas/).",
		Attributes: buildUserAttributes(map[string]schema.Attribute{
			"authentication": schema.StringAttribute{
				MarkdownDescription: "❗ Authentication details. The possible values are `null`, `caching_sha2_password` and `mysql_native_password`.",
				Optional:            true,
				Computed:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("caching_sha2_password", "mysql_native_password"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		}),
		Blocks: map[string]schema.Block{
			"timeouts": timeouts.BlockAll(ctx),
		},
	}
}

func (r *MysqlUserResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data MysqlUserResourceModel
	UserRead(ctx, req, resp, &data, r.client)
}

func (r *MysqlUserResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data MysqlUserResourceModel
	UserCreate(ctx, req, resp, &data, r.client)
}

func (r *MysqlUserResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var stateData, planData MysqlUserResourceModel
	UserUpdate(ctx, req, resp, &stateData, &planData, r.client)
}

func (r *MysqlUserResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data MysqlUserResourceModel
	UserDelete(ctx, req, resp, &data, r.client)
}

func (r *MysqlUserResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)

	idParts := strings.Split(req.ID, "@")

	if len(idParts) != 2 || idParts[0] == "" || idParts[1] == "" {

		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected import identifier with format: service/username@zone. Got: %q", req.ID),
		)

		return
	}

	userID := idParts[0]
	zone := idParts[1]

	id := strings.Split(userID, "/")

	if len(id) != 2 || id[0] == "" || id[1] == "" {
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected import identifier with format: service/username@zone. Got: %q", req.ID),
		)
	}

	serviceName := id[0]
	username := id[1]

	var data MysqlUserResourceModel

	// Set timeouts (quirk https://github.com/hashicorp/terraform-plugin-framework-timeouts/issues/46)
	var timeouts timeouts.Value
	resp.Diagnostics.Append(resp.State.GetAttribute(ctx, path.Root("timeouts"), &timeouts)...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Timeouts = timeouts

	data.Id = types.StringValue(userID)
	data.Username = types.StringValue(username)
	data.Service = types.StringValue(serviceName)
	data.Zone = types.StringValue(zone)

	UserReadForImport(ctx, req, resp, &data, r.client)

}

func (data *MysqlUserResourceModel) CreateResource(ctx context.Context, client *exoscale.Client, diagnostics *diag.Diagnostics) {

	createRequest := exoscale.CreateDBAASMysqlUserRequest{
		Username: exoscale.DBAASUserUsername(data.Username.ValueString()),
	}

	if !data.Authentication.IsNull() {
		createRequest.Authentication = exoscale.EnumMysqlAuthenticationPlugin(data.Authentication.ValueString())

	}

	op, err := client.CreateDBAASMysqlUser(ctx, data.Service.ValueString(), createRequest)
	if err != nil {
		diagnostics.AddError(
			"Client Error",
			fmt.Sprintf("Unable to create service user, got error %s", err.Error()),
		)
		return
	}

	_, err = client.Wait(ctx, op, exoscale.OperationStateSuccess)
	if err != nil {
		diagnostics.AddError(
			"Client Error",
			fmt.Sprintf("Unable to create service user, got error %s", err.Error()),
		)
		return
	}

	svc, err := client.GetDBAASServiceMysql(ctx, data.Service.ValueString())
	if err != nil {
		diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read database service mysql, got error: %s", err))
		return
	}

	for _, user := range svc.Users {
		if user.Username == data.Username.ValueString() {
			data.Password = basetypes.NewStringValue(user.Password)
			data.Type = basetypes.NewStringValue(user.Type)
			data.Authentication = basetypes.NewStringValue(user.Authentication)
			return
		}
	}
	diagnostics.AddError("Client Error", "Unable to find newly created user for the service")
}

func (data *MysqlUserResourceModel) Delete(ctx context.Context, client *exoscale.Client, diagnostics *diag.Diagnostics) {

	op, err := client.DeleteDBAASMysqlUser(ctx, data.Service.ValueString(), data.Username.ValueString())
	if err != nil {
		diagnostics.AddError(
			"Client Error",
			fmt.Sprintf("Unable to delete service user, got error %s", err.Error()),
		)
		return
	}

	_, err = client.Wait(ctx, op, exoscale.OperationStateSuccess)
	if err != nil {
		diagnostics.AddError(
			"Client Error",
			fmt.Sprintf("Unable to delete service user, got error %s", err.Error()),
		)
		return
	}

}

func (data *MysqlUserResourceModel) ReadResource(ctx context.Context, client *exoscale.Client, diagnostics *diag.Diagnostics) {

	svc, err := client.GetDBAASServiceMysql(ctx, data.Service.ValueString())
	if err != nil {
		diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read service mysql user, got error: %s", err))
		return
	}

	for _, user := range svc.Users {
		if user.Username == data.Username.ValueString() {
			data.Password = basetypes.NewStringValue(user.Password)
			data.Type = basetypes.NewStringValue(user.Type)
			data.Authentication = basetypes.NewStringValue(user.Authentication)
			return
		}
	}
	diagnostics.AddError("Client Error", "Unable to read user for the service")
}

func (data *MysqlUserResourceModel) UpdateResource(ctx context.Context, client *exoscale.Client, diagnostics *diag.Diagnostics) {
	// Nothing to do here as all fields of this resource are immutable; replaces will be required
	// automatically
}

func (data *MysqlUserResourceModel) WaitForService(ctx context.Context, client *exoscale.Client, diagnostics *diag.Diagnostics) {
	_, err := waitForDBAASServiceReadyForUsers(ctx, client.GetDBAASServiceMysql, data.Service.ValueString(), func(t *exoscale.DBAASServiceMysql) bool { return t.State == exoscale.EnumServiceStateRunning })
	if err != nil {
		diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read Database service MySQL %s", err.Error()))
	}
}

func (data *MysqlUserResourceModel) GetTimeouts() timeouts.Value {
	return data.Timeouts
}

func (data *MysqlUserResourceModel) SetTimeouts(t timeouts.Value) {
	data.Timeouts = t
}

func (data *MysqlUserResourceModel) GetID() basetypes.StringValue {
	return data.Id
}
func (data *MysqlUserResourceModel) GetZone() basetypes.StringValue {
	return data.Zone
}

func (data *MysqlUserResourceModel) GenerateID() {
	data.Id = basetypes.NewStringValue(fmt.Sprintf("%s/%s", data.Service.ValueString(), data.Username.ValueString()))
}
