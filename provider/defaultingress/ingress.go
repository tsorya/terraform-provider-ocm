package defaultingress

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	"github.com/terraform-redhat/terraform-provider-rhcs/provider/common"
	"github.com/terraform-redhat/terraform-provider-rhcs/provider/common/attrvalidators"
	"reflect"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const lowestDefaultIngressVer = "4.14.0-0"

var ValidWildcardPolicies = []string{string(cmv1.WildcardPolicyWildcardsDisallowed),
	string(cmv1.WildcardPolicyWildcardsAllowed)}
var DefaultWildcardPolicy = cmv1.WildcardPolicyWildcardsDisallowed

var ValidNamespaceOwnershipPolicies = []string{string(cmv1.NamespaceOwnershipPolicyStrict),
	string(cmv1.NamespaceOwnershipPolicyInterNamespaceAllowed)}
var DefaultNamespaceOwnershipPolicy = cmv1.NamespaceOwnershipPolicyStrict

func IngressResource() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Description: "Unique identifier of the ingress.",
			Computed:    true,
			Optional:    true,
			PlanModifiers: []planmodifier.String{
				// This passes the state through to the plan, preventing
				// "known after apply" since we know it won't change.
				stringplanmodifier.UseStateForUnknown(),
			},
		},
		"route_selectors": schema.MapAttribute{
			Description: "Route Selectors for ingress. Format should be a comma-separated list of 'key=value'. " +
				"If no label is specified, all routes will be exposed on both routers." +
				"For legacy ingress support these are inclusion labels, otherwise they are treated as exclusion label.",

			ElementType: types.StringType,
			Optional:    true,
			Validators:  []validator.Map{NotEmptyMapValidator()},
		},
		"excluded_namespaces": schema.ListAttribute{
			Description: "Excluded namespaces for ingress. Format should be a comma-separated list 'value1, value2...'. " +
				"If no values are specified, all namespaces will be exposed.",
			ElementType: types.StringType,
			Optional:    true,
			Validators: []validator.List{
				listvalidator.SizeAtLeast(1),
			},
		},
		"route_wildcard_policy": schema.StringAttribute{
			Description: fmt.Sprintf("Wildcard Policy for ingress. Options are %s. Default is '%s'.",
				strings.Join(ValidWildcardPolicies, ","), DefaultWildcardPolicy),
			Optional:   true,
			Computed:   true,
			Validators: []validator.String{attrvalidators.EnumValueValidator(ValidWildcardPolicies)},
		},
		"route_namespace_ownership_policy": schema.StringAttribute{
			Description: fmt.Sprintf("Namespace Ownership Policy for ingress. Options are %s. Default is '%s'.",
				strings.Join(ValidNamespaceOwnershipPolicies, ","), DefaultNamespaceOwnershipPolicy),
			Optional:   true,
			Computed:   true,
			Validators: []validator.String{attrvalidators.EnumValueValidator(ValidNamespaceOwnershipPolicies)},
		},
		"cluster_routes_hostname": schema.StringAttribute{
			Description: "Components route hostname for oauth, console, download.",
			Optional:    true,
		},
		"cluster_routes_tls_secret_ref": schema.StringAttribute{
			Description: "Components route TLS secret reference for oauth, console, download.",
			Optional:    true,
		},
	}
}

func PopulateDefaultIngress(ctx context.Context, state *DefaultIngress,
	client *cmv1.IngressesClient) (*DefaultIngress, error) {

	// in case default ingress is not part of state no need to populate it
	if state == nil {
		return nil, nil
	}
	ingresses, err := client.List().SendContext(ctx)
	if err != nil {
		return nil, err
	}
	for _, ingress := range ingresses.Items().Slice() {
		if ingress.Default() {
			if state == nil {
				state = &DefaultIngress{}
			}
			state.Id = types.StringValue(ingress.ID())

			routeSelectors, ok := ingress.GetRouteSelectors()
			if ok {
				state.RouteSelectors, err = common.ConvertStringMapToMapType(routeSelectors)
				if err != nil {
					return nil, err
				}
			}

			excludedNamespaces, ok := ingress.GetExcludedNamespaces()
			if ok {
				state.ExcludedNamespaces, err = common.StringArrayToList(excludedNamespaces)
				if err != nil {
					return nil, err
				}
			}
			wp, ok := ingress.GetRouteWildcardPolicy()
			if ok {
				state.WildcardPolicy = types.StringValue(string(wp))
			} else {
				state.WildcardPolicy = types.StringNull()
			}
			rnmop, ok := ingress.GetRouteNamespaceOwnershipPolicy()
			if ok {
				state.NamespaceOwnershipPolicy = types.StringValue(string(rnmop))
			} else {
				state.NamespaceOwnershipPolicy = types.StringNull()
			}
			hostname, ok := ingress.GetClusterRoutesHostname()
			if ok {
				state.ClusterRoutesHostname = types.StringValue(hostname)
			} else {
				state.ClusterRoutesHostname = types.StringNull()
			}
			tls, ok := ingress.GetClusterRoutesTlsSecretRef()
			if ok {
				state.ClusterRoutesTlsSecretRef = types.StringValue(tls)
			} else {
				state.ClusterRoutesTlsSecretRef = types.StringNull()
			}
			break
		}
	}
	return state, nil
}

func getDefaultIngressBuilder(ctx context.Context, state *DefaultIngress) *cmv1.IngressBuilder {
	ingressBuilder := cmv1.NewIngress()
	routeSelectors, err := common.OptionalMap(ctx, state.RouteSelectors)
	if err != nil {
		return nil
	}
	if routeSelectors == nil {
		routeSelectors = map[string]string{}
	}
	ingressBuilder.RouteSelectors(routeSelectors)

	excludedNamespace, err := common.OptionalList(ctx, state.ExcludedNamespaces)
	if err != nil {
		return nil
	}
	if excludedNamespace == nil {
		excludedNamespace = []string{}
	}
	ingressBuilder.ExcludedNamespaces(excludedNamespace...)

	if !common.IsStringAttributeEmpty(state.WildcardPolicy) {
		ingressBuilder.RouteWildcardPolicy(cmv1.WildcardPolicy(state.WildcardPolicy.ValueString()))
	}
	if !common.IsStringAttributeEmpty(state.NamespaceOwnershipPolicy) {
		ingressBuilder.RouteNamespaceOwnershipPolicy(cmv1.NamespaceOwnershipPolicy(state.NamespaceOwnershipPolicy.ValueString()))
	}
	return ingressBuilder
}

func SetIngress(ctx context.Context, state *DefaultIngress, clusterBuilder *cmv1.ClusterBuilder) *cmv1.ClusterBuilder {
	if state == nil {
		return clusterBuilder
	}

	ingressBuilder := getDefaultIngressBuilder(ctx, state)
	ingressBuilder.Default(true)
	clusterBuilder.Ingresses(cmv1.NewIngressList().Items(ingressBuilder))
	return clusterBuilder
}

func UpdateIngress(ctx context.Context, state, plan *DefaultIngress, clusterId, version string,
	clusterCollection *cmv1.ClustersClient) error {
	if !reflect.DeepEqual(state, plan) {
		err := ValidateDefaultIngress(ctx, plan, version)
		if err != nil {
			return err
		}
		// In case default ingress was not part of state till now and we want to set
		// it as day2 we need to bring it first as we need to set specific id
		if state == nil || common.IsStringAttributeEmpty(state.Id) {
			state = &DefaultIngress{}
			state, err = PopulateDefaultIngress(ctx, state, clusterCollection.Cluster(clusterId).Ingresses())
			if err != nil {
				return err
			}
		}

		ingressBuilder := getDefaultIngressBuilder(ctx, plan)

		if !reflect.DeepEqual(state.ClusterRoutesHostname, plan.ClusterRoutesHostname) {
			ingressBuilder.ClusterRoutesHostname(plan.ClusterRoutesHostname.ValueString())
		}
		if !reflect.DeepEqual(state.ClusterRoutesTlsSecretRef, plan.ClusterRoutesTlsSecretRef) {
			ingressBuilder.ClusterRoutesTlsSecretRef(plan.ClusterRoutesTlsSecretRef.ValueString())
		}

		ingress, err := ingressBuilder.Build()
		if err != nil {
			return err
		}

		_, err = clusterCollection.Cluster(clusterId).Ingresses().Ingress(state.Id.ValueString()).Update().
			Body(ingress).SendContext(ctx)

		if err != nil {
			return err
		}
	}

	return nil
}

func ValidateDefaultIngress(ctx context.Context, state *DefaultIngress, version string) error {
	if state == nil {
		return nil
	}

	greater, err := common.IsGreaterThanOrEqual(version, lowestDefaultIngressVer)
	if err != nil {
		return fmt.Errorf("version '%s' is not supported: %v", version, err)
	}
	if !greater {
		msg := fmt.Sprintf("version '%s' is not supported with default ingress, "+
			"minimum supported version is %s", version, lowestDefaultIngressVer)
		tflog.Error(ctx, msg)
		return fmt.Errorf(msg)
	}
	ingress := state

	if common.IsStringAttributeEmpty(ingress.Id) && (!common.IsStringAttributeEmpty(ingress.ClusterRoutesHostname) ||
		!common.IsStringAttributeEmpty(ingress.ClusterRoutesTlsSecretRef)) {
		msg := fmt.Sprintf("default_ingress params: cluster_routes_hostname and cluster_routes_tls_secret_ref " +
			"can't be set on cluster creation")
		tflog.Error(ctx, msg)
		return fmt.Errorf(msg)
	}
	if common.IsStringAttributeEmpty(ingress.ClusterRoutesHostname) != common.IsStringAttributeEmpty(ingress.ClusterRoutesTlsSecretRef) {
		msg := fmt.Sprintf("default_ingress params: cluster_routes_hostname and cluster_routes_tls_secret_ref must be set together")
		tflog.Error(ctx, msg)
		return fmt.Errorf(msg)
	}

	return nil
}
