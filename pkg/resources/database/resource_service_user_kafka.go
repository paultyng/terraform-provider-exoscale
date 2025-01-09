package database

import (
	"context"
	"fmt"
	"strings"

	exoscale "github.com/exoscale/egoscale/v3"
	providerConfig "github.com/exoscale/terraform-provider-exoscale/pkg/provider/config"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

var _ resource.Resource = &KafkaUserResource{}
var _ resource.ResourceWithImportState = &KafkaUserResource{}

func NewKafkaUserResource() resource.Resource {
	return &KafkaUserResource{}
}

type KafkaUserResource struct {
	UserResource
}

type KafkaUserResourceModel struct {
	UserResourceModel
	AccessKey        types.String `tfsdk:"access_key"`
	AccessCert       types.String `tfsdk:"access_cert"`
	AccessCertExpiry types.String `tfsdk:"access_cert_expiry"`
}

func (r *KafkaUserResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*providerConfig.ExoscaleProviderConfig).ClientV3
}

func (r *KafkaUserResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dbaas_kafka_user"
}

func (r *KafkaUserResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Manage service users for a Kafka Exoscale [Database Services (DBaaS)](https://community.exoscale.com/documentation/dbaas/).",
		Attributes: buildUserAttributes(map[string]schema.Attribute{
			"access_key": schema.StringAttribute{
				Description: "Access certificate key for the user.",
				Computed:    true,
				Sensitive:   true,
			},
			"access_cert": schema.StringAttribute{
				Description: "Access certificate for the user.",
				Computed:    true,
				Sensitive:   true,
			},
			"access_cert_expiry": schema.StringAttribute{
				Description: "Access certificate expiry date.",
				Computed:    true,
			},
		}),
		Blocks: map[string]schema.Block{
			"timeouts": timeouts.BlockAll(ctx),
		},
	}
}

func (r *KafkaUserResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data KafkaUserResourceModel
	UserRead(ctx, req, resp, &data, r.client)
}

func (r *KafkaUserResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {

	var data KafkaUserResourceModel
	UserCreate(ctx, req, resp, &data, r.client)
}

func (r *KafkaUserResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var stateData, planData KafkaUserResourceModel
	UserUpdate(ctx, req, resp, &stateData, &planData, r.client)
}

func (r *KafkaUserResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data KafkaUserResourceModel
	UserDelete(ctx, req, resp, &data, r.client)
}

func (r *KafkaUserResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
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

	var data KafkaUserResourceModel

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

func (data *KafkaUserResourceModel) CreateResource(ctx context.Context, client *exoscale.Client, diagnostics *diag.Diagnostics) {

	createRequest := exoscale.CreateDBAASKafkaUserRequest{
		Username: exoscale.DBAASUserUsername(data.Username.ValueString()),
	}

	op, err := client.CreateDBAASKafkaUser(ctx, data.Service.ValueString(), createRequest)
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

	svc, err := client.GetDBAASServiceKafka(ctx, data.Service.ValueString())
	if err != nil {
		diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read database service kafka, got error: %s", err))
		return
	}

	for _, user := range svc.Users {
		if user.Username == data.Username.ValueString() {
			data.Password = basetypes.NewStringValue(user.Password)
			data.Type = basetypes.NewStringValue(user.Type)
			data.AccessCert = basetypes.NewStringValue(user.AccessCert)
			data.AccessKey = basetypes.NewStringValue(user.AccessKey)
			data.AccessCertExpiry = basetypes.NewStringValue(user.AccessCertExpiry.String())
			return
		}
	}
	diagnostics.AddError("Client Error", "Unable to find newly created user for the service")
}

func (data *KafkaUserResourceModel) Delete(ctx context.Context, client *exoscale.Client, diagnostics *diag.Diagnostics) {

	op, err := client.DeleteDBAASKafkaUser(ctx, data.Service.ValueString(), data.Username.ValueString())
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

func (data *KafkaUserResourceModel) ReadResource(ctx context.Context, client *exoscale.Client, diagnostics *diag.Diagnostics) {

	svc, err := client.GetDBAASServiceKafka(ctx, data.Service.ValueString())
	if err != nil {
		diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read service kafka user, got error: %s", err))
		return
	}

	for _, user := range svc.Users {
		if user.Username == data.Username.ValueString() {
			data.Password = basetypes.NewStringValue(user.Password)
			data.Type = basetypes.NewStringValue(user.Type)
			data.AccessCert = basetypes.NewStringValue(user.AccessCert)
			data.AccessKey = basetypes.NewStringValue(user.AccessKey)
			data.AccessCertExpiry = basetypes.NewStringValue(user.AccessCertExpiry.String())
			return
		}
	}
	diagnostics.AddError("Client Error", "Unable to read user for the service")
}

func (data *KafkaUserResourceModel) UpdateResource(ctx context.Context, client *exoscale.Client, diagnostics *diag.Diagnostics) {
	// Nothing to do here as all fields of this resource are immutable; replaces will be required
	// automatically
}

func (data *KafkaUserResourceModel) WaitForService(ctx context.Context, client *exoscale.Client, diagnostics *diag.Diagnostics) {
	_, err := waitForDBAASServiceReadyForUsers(ctx, client.GetDBAASServiceKafka, data.Service.ValueString(), func(t *exoscale.DBAASServiceKafka) bool { return len(t.Users) > 0 })
	if err != nil {
		diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read Database service Kafka %s", err.Error()))
	}
}

func (data *KafkaUserResourceModel) GetTimeouts() timeouts.Value {
	return data.Timeouts
}

func (data *KafkaUserResourceModel) SetTimeouts(t timeouts.Value) {
	data.Timeouts = t
}

func (data *KafkaUserResourceModel) GetID() basetypes.StringValue {
	return data.Id
}
func (data *KafkaUserResourceModel) GetZone() basetypes.StringValue {
	return data.Zone
}

func (data *KafkaUserResourceModel) GenerateID() {
	data.Id = basetypes.NewStringValue(fmt.Sprintf("%s/%s", data.Service.ValueString(), data.Username.ValueString()))
}
