package toproto5

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/internal/fwserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
)

// ServerCapabilities returns the *tfprotov5.ServerCapabilities for a
// *fwserver.ServerCapabilities.
func ServerCapabilities(ctx context.Context, fw *fwserver.ServerCapabilities) *tfprotov5.ServerCapabilities {
	if fw == nil {
		return nil
	}

	return &tfprotov5.ServerCapabilities{
		PlanDestroy: fw.PlanDestroy,
	}
}
