package database

import (
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/types"

	v3 "github.com/exoscale/egoscale/v3"
)

type DBResource struct {
	client *v3.Client
}

type DBResourceModel struct {
	Id           types.String `tfsdk:"id"`
	Service      types.String `tfsdk:"service"`
	DatabaseName types.String `tfsdk:"database_name"`
	Zone         types.String `tfsdk:"zone"`

	Timeouts timeouts.Value `tfsdk:"timeouts"`
}
