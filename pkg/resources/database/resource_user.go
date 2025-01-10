package database

import (
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	exoscale "github.com/exoscale/egoscale/v3"
	"github.com/exoscale/terraform-provider-exoscale/pkg/config"
)

// UserResource defines the resource implementation.
type UserResource struct {
	client *exoscale.Client
}

// UserResourceModel describes the resource data model.
type UserResourceModel struct {
	Id       types.String `tfsdk:"id"`
	Service  types.String `tfsdk:"service"`
	Username types.String `tfsdk:"username"`
	Password types.String `tfsdk:"password"`
	Zone     types.String `tfsdk:"zone"`
	Type     types.String `tfsdk:"type"`

	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

var commonAttributes = map[string]schema.Attribute{
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
	"username": schema.StringAttribute{
		Required:            true,
		MarkdownDescription: "❗ The name of the user for this service.",
		PlanModifiers: []planmodifier.String{
			stringplanmodifier.RequiresReplace(),
		},
	},

	// Computed attributes
	"id": schema.StringAttribute{
		MarkdownDescription: "The ID of this resource, computed as service/username",
		Computed:            true,
		PlanModifiers: []planmodifier.String{
			stringplanmodifier.UseStateForUnknown(),
		},
	},
	"password": schema.StringAttribute{
		Description: "The password of the service user.",
		Computed:    true,
		Sensitive:   true,
	},
	"type": schema.StringAttribute{
		Description: "The type of the service user.",
		Computed:    true,
	},
}

func buildUserAttributes(newAttributes map[string]schema.Attribute) map[string]schema.Attribute {

	newSchemas := map[string]schema.Attribute{}
	for k, v := range commonAttributes {
		newSchemas[k] = v
	}
	for k, v := range newAttributes {
		newSchemas[k] = v
	}

	return newSchemas

}
