package common

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func NotEmptyList() []tfsdk.AttributeValidator {
	return []tfsdk.AttributeValidator{
		&AttributeValidator{
			Desc: "Validate that list is not empty",
			Validator: func(ctx context.Context, req tfsdk.ValidateAttributeRequest, resp *tfsdk.ValidateAttributeResponse) {

				value := &types.List{}
				diag := req.Config.GetAttribute(ctx, req.AttributePath, value)
				if diag.HasError() {
					// No attribute to validate
					return
				}
				if len(value.Elems) == 0 && value.Null != true {
					resp.Diagnostics.AddError(fmt.Sprintf("Invalid %s.", req.AttributePath.LastStep()),
						fmt.Sprintf("Expected at least one value in list for %s.",
							req.AttributePath.LastStep()),
					)
				}
			},
		},
	}
}

func NotEmptyMap() []tfsdk.AttributeValidator {
	return []tfsdk.AttributeValidator{
		&AttributeValidator{
			Desc: "Validate that list is not empty",
			Validator: func(ctx context.Context, req tfsdk.ValidateAttributeRequest, resp *tfsdk.ValidateAttributeResponse) {

				value := &types.Map{}
				diag := req.Config.GetAttribute(ctx, req.AttributePath, value)
				if diag.HasError() {
					// No attribute to validate
					return
				}
				if len(value.Elems) == 0 && value.Null != true {
					resp.Diagnostics.AddError(fmt.Sprintf("Invalid %s.", req.AttributePath.LastStep()),
						fmt.Sprintf("Expected at least one value in map for %s.",
							req.AttributePath.LastStep()),
					)
				}
			},
		},
	}
}
