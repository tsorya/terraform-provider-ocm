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

package groupmembership

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	sdk "github.com/openshift-online/ocm-sdk-go"
	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
)

type GroupMembershipResource struct {
	collection *cmv1.ClustersClient
}

var _ resource.ResourceWithConfigure = &GroupMembershipResource{}
var _ resource.ResourceWithImportState = &GroupMembershipResource{}

func New() resource.Resource {
	return &GroupMembershipResource{}
}

func (g *GroupMembershipResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group_membership"
}

func (g *GroupMembershipResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages user group membership.",
		Attributes: map[string]schema.Attribute{
			"cluster": schema.StringAttribute{
				Description: "Identifier of the cluster.",
				Required:    true,
			},
			"group": schema.StringAttribute{
				Description: "Identifier of the group.",
				Required:    true,
			},
			"id": schema.StringAttribute{
				Description: "Identifier of the membership.",
				Computed:    true,
			},
			"user": schema.StringAttribute{
				Description: "user name.",
				Required:    true,
			},
		},
	}
	return
}

func (g *GroupMembershipResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {

	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	connection, ok := req.ProviderData.(*sdk.Connection)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *sdk.Connaction, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	g.collection = connection.ClustersMgmt().V1().Clusters()
}

func (g *GroupMembershipResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Get the plan:
	state := &GroupMembershipState{}
	diags := req.Plan.Get(ctx, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Wait till the cluster is ready:
	resource := g.collection.Cluster(state.Cluster.String())
	pollCtx, cancel := context.WithTimeout(ctx, 1*time.Hour)
	defer cancel()
	_, err := resource.Poll().
		Interval(30 * time.Second).
		Predicate(func(get *cmv1.ClusterGetResponse) bool {
			return get.Body().State() == cmv1.ClusterStateReady
		}).
		StartContext(pollCtx)
	if err != nil {
		resp.Diagnostics.AddError(
			"Can't poll cluster state",
			fmt.Sprintf(
				"Can't poll state of cluster with identifier '%s': %v",
				state.Cluster.String(), err,
			),
		)
		return
	}

	// Create the membership:
	builder := cmv1.NewUser()
	builder.ID(state.User.String())
	object, err := builder.Build()
	if err != nil {
		resp.Diagnostics.AddError(
			"Can't build group membership",
			fmt.Sprintf(
				"Can't build group membership for cluster '%s' and group '%s': %v",
				state.Cluster.String(), state.Group.String(), err,
			),
		)
		return
	}
	collection := resource.Groups().Group(state.Group.String()).Users()
	add, err := collection.Add().Body(object).SendContext(ctx)
	if err != nil {
		resp.Diagnostics.AddError(
			"Can't create group membership",
			fmt.Sprintf(
				"Can't create group membership for cluster '%s' and group '%s': %v",
				state.Cluster.String(), state.Group.String(), err,
			),
		)
		return
	}
	object = add.Body()

	// Save the state:
	g.populateState(object, state)
	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

func (g *GroupMembershipResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Get the current state:
	state := &GroupMembershipState{}
	diags := req.State.Get(ctx, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Find the group membership:
	obj := g.collection.Cluster(state.Cluster.String()).Groups().Group(state.Group.String()).
		Users().
		User(state.ID.String())
	get, err := obj.Get().SendContext(ctx)
	if err != nil {
		resp.Diagnostics.AddError(
			"Can't find group membership",
			fmt.Sprintf(
				"Can't find user group membership identifier '%s' for "+
					"cluster '%s' and group '%s': %v",
				state.ID.String(), state.Cluster.String(), state.Group.String(), err,
			),
		)
		return
	}
	object := get.Body()

	// Save the state:
	g.populateState(object, state)

	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

func (g *GroupMembershipResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Until we support. return an informative error
	resp.Diagnostics.AddError("Can't update group membership", "Update is currently not supported.")
}

func (g *GroupMembershipResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Get the state:
	state := &GroupMembershipState{}
	diags := req.State.Get(ctx, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Send the request to delete group membership:
	obj := g.collection.Cluster(state.Cluster.String()).Groups().Group(state.Group.String()).
		Users().
		User(state.ID.String())
	_, err := obj.Delete().SendContext(ctx)
	if err != nil {
		resp.Diagnostics.AddError(
			"Can't delete group membership",
			fmt.Sprintf(
				"Can't delete group membership with identifier '%s' for "+
					"cluster '%s' and group '%s': %v",
				state.ID.String(), state.Cluster.String(), state.Group.String(), err,
			),
		)
		return
	}

	// Remove the state:
	resp.State.RemoveResource(ctx)
}

func (g *GroupMembershipResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// populateState copies the data from the API object to the Terraform state.
func (g *GroupMembershipResource) populateState(object *cmv1.User, state *GroupMembershipState) {
	if id, ok := object.GetID(); ok {
		state.ID = types.StringValue(id)
		state.User = types.StringValue(id)
	}
}
