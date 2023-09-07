/*
Copyright (c) 2021 Red Hat, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

func New(primary interface{ Meta() interface{} }) provider.Provider {
	return &fwprovider{
		Primary: primary,
	}
}

type fwprovider struct {
	Primary interface{ Meta() interface{} }
}

func (p *fwprovider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "aws"
}

func (p *fwprovider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"url": schema.StringAttribute{
				Description: "URL of the API server.",
				Optional:    true,
			},
			"token_url": schema.StringAttribute{
				Description: "OpenID token URL.",
				Optional:    true,
			},
			"user": schema.StringAttribute{
				Description: "User name.",
				Optional:    true,
				Sensitive:   true,
			},
			"password": schema.StringAttribute{
				Description: "User password.",
				Optional:    true,
				Sensitive:   true,
			},
			"token": schema.StringAttribute{
				Description: "Access or refresh token that is " +
					"generated from https://console.redhat.com/openshift/token/rosa.",
				Optional:  true,
				Sensitive: true,
			},
			"client_id": schema.StringAttribute{
				Description: "OpenID client identifier.",
				Optional:    true,
			},
			"client_secret": schema.StringAttribute{
				Description: "OpenID client secret.",
				Optional:    true,
				Sensitive:   true,
			},
			"trusted_cas": schema.StringAttribute{
				Description: "PEM encoded certificates of authorities that will " +
					"be trusted. If this is not explicitly specified, then " +
					"the clusterservice will trust the certificate authorities " +
					"trusted by default by the system.",
				Optional: true,
			},
			"insecure": schema.BoolAttribute{
				Description: "When set to 'true' enables insecure communication " +
					"with the server. This disables verification of TLS " +
					"certificates and host names, and it is not recommended " +
					"for production environments.",
				Optional: true,
			},
		},
	}
}

// Configure is called at the beginning of the provider lifecycle, when
// Terraform sends to the provider the values the user specified in the
// provider configuration block.
func (p *fwprovider) Configure(ctx context.Context, request provider.ConfigureRequest, response *provider.ConfigureResponse) {
	// Provider's parsed configuration (its instance state) is available through the primary provider's Meta() method.
	v := p.Primary.Meta()
	response.DataSourceData = v
	response.ResourceData = v
}

// DataSources satisfies the provider.Provider interface for Provider.
func (p *fwprovider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		// Provider specific implementation
	}
}

// Resources satisfies the provider.Provider interface for Provider.
func (p *fwprovider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		// Provider specific implementation
	}
}
