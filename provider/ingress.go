package provider

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	"github.com/terraform-redhat/terraform-provider-rhcs/provider/common"
	"reflect"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const lowestDefaultIngressVer = "4.14.0-0"

var ValidWildcardPolicies = []string{string(cmv1.WildcardPolicyWildcardsDisallowed),
	string(cmv1.WildcardPolicyWildcardsAllowed)}
var DefaultWildcardPolicy = cmv1.WildcardPolicyWildcardsDisallowed

var ValidNamespaceOwnershipPolicies = []string{string(cmv1.NamespaceOwnershipPolicyStrict),
	string(cmv1.NamespaceOwnershipPolicyInterNamespaceAllowed)}
var DefaultNamespaceOwnershipPolicy = cmv1.NamespaceOwnershipPolicyStrict

type DefaultIngress struct {
	RouteSelectors            types.Map    `tfsdk:"route_selectors"`
	ExcludedNamespaces        types.List   `tfsdk:"excluded_namespaces"`
	WildcardPolicy            types.String `tfsdk:"route_wildcard_policy"`
	NamespaceOwnershipPolicy  types.String `tfsdk:"route_namespace_ownership_policy"`
	Id                        types.String `tfsdk:"id"`
	ClusterRoutesHostname     types.String `tfsdk:"cluster_routes_hostname"`
	ClusterRoutesTlsSecretRef types.String `tfsdk:"cluster_routes_tls_secret_ref"`
}

func ingressResource() tfsdk.NestedAttributes {
	return tfsdk.SingleNestedAttributes(map[string]tfsdk.Attribute{
		"id": {
			Description: "Unique identifier of the ingress.",
			Type:        types.StringType,
			Computed:    true,
			Optional:    true,
			PlanModifiers: []tfsdk.AttributePlanModifier{
				ValueCannotBeChangedModifier(),
				tfsdk.UseStateForUnknown(),
			},
		},
		"route_selectors": {
			Description: "Route Selectors for ingress. Format should be a comma-separated list of 'key=value'. " +
				"If no label is specified, all routes will be exposed on both routers." +
				"For legacy ingress support these are inclusion labels, otherwise they are treated as exclusion label.",

			Type: types.MapType{
				ElemType: types.StringType,
			},
			Optional:   true,
			Validators: common.NotEmptyMap(),
		},
		"excluded_namespaces": {
			Description: "Excluded namespaces for ingress. Format should be a comma-separated list 'value1, value2...'. " +
				"If no values are specified, all namespaces will be exposed.",
			Type: types.ListType{
				ElemType: types.StringType,
			},
			Optional:   true,
			Validators: common.NotEmptyList(),
		},
		"route_wildcard_policy": {
			Description: fmt.Sprintf("Wildcard Policy for ingress. Options are %s. Default is '%s'.",
				strings.Join(ValidWildcardPolicies, ","), DefaultWildcardPolicy),
			Type:       types.StringType,
			Optional:   true,
			Computed:   true,
			Validators: EnumValueValidator(ValidWildcardPolicies),
		},
		"route_namespace_ownership_policy": {
			Description: fmt.Sprintf("Namespace Ownership Policy for ingress. Options are %s. Default is '%s'.",
				strings.Join(ValidNamespaceOwnershipPolicies, ","), DefaultNamespaceOwnershipPolicy),
			Type:       types.StringType,
			Optional:   true,
			Computed:   true,
			Validators: EnumValueValidator(ValidNamespaceOwnershipPolicies),
		},
		"cluster_routes_hostname": {
			Description: "Components route hostname for oauth, console, download.",
			Type:        types.StringType,
			Optional:    true,
		},
		"cluster_routes_tls_secret_ref": {
			Description: "Components route TLS secret reference for oauth, console, download.",
			Type:        types.StringType,
			Optional:    true,
		},
	})
}

func populateDefaultIngress(ctx context.Context, state *ClusterRosaClassicState,
	client *cmv1.IngressesClient) error {

	// in case default ingress is not part of state no need to populate it
	if state.DefaultIngress == nil {
		return nil
	}
	ingresses, err := client.List().SendContext(ctx)
	if err != nil {
		return err
	}
	for _, ingress := range ingresses.Items().Slice() {
		if ingress.Default() {
			if state.DefaultIngress == nil {
				state.DefaultIngress = &DefaultIngress{}
			}
			state.DefaultIngress.Id = types.String{Value: ingress.ID()}

			state.DefaultIngress.RouteSelectors = common.MapStringToMapTypeWithNullAsEmpty(ingress.RouteSelectors())
			state.DefaultIngress.ExcludedNamespaces = common.StringArrayToListWithNullAsEmpty(ingress.ExcludedNamespaces())

			state.DefaultIngress.WildcardPolicy = types.String{Value: string(ingress.RouteWildcardPolicy())}
			state.DefaultIngress.NamespaceOwnershipPolicy =
				types.String{Value: string(ingress.RouteNamespaceOwnershipPolicy())}
			state.DefaultIngress.ClusterRoutesHostname = common.StringToStringType(ingress.ClusterRoutesHostname())
			state.DefaultIngress.ClusterRoutesTlsSecretRef = common.StringToStringType(ingress.ClusterRoutesTlsSecretRef())
			break
		}
	}
	return nil
}

func getDefaultIngressBuilder(state *ClusterRosaClassicState) *cmv1.IngressBuilder {
	ingressBuilder := cmv1.NewIngress()
	routeSelectors := common.OptionalMap(state.DefaultIngress.RouteSelectors)
	if routeSelectors == nil {
		routeSelectors = map[string]string{}
	}
	ingressBuilder.RouteSelectors(routeSelectors)
	ingressBuilder.ExcludedNamespaces(common.OptionalList(state.DefaultIngress.ExcludedNamespaces)...)
	if !common.IsStringAttributeEmpty(state.DefaultIngress.WildcardPolicy) {
		ingressBuilder.RouteWildcardPolicy(cmv1.WildcardPolicy(state.DefaultIngress.WildcardPolicy.Value))
	}
	if !common.IsStringAttributeEmpty(state.DefaultIngress.NamespaceOwnershipPolicy) {
		ingressBuilder.RouteNamespaceOwnershipPolicy(cmv1.NamespaceOwnershipPolicy(state.DefaultIngress.NamespaceOwnershipPolicy.Value))
	}
	return ingressBuilder
}

func setIngress(state *ClusterRosaClassicState, clusterBuilder *cmv1.ClusterBuilder) *cmv1.ClusterBuilder {
	if state.DefaultIngress == nil {
		return clusterBuilder
	}

	ingressBuilder := getDefaultIngressBuilder(state)
	ingressBuilder.Default(true)
	clusterBuilder.Ingresses(cmv1.NewIngressList().Items(ingressBuilder))
	return clusterBuilder
}

func updateIngress(ctx context.Context, state, plan *ClusterRosaClassicState,
	clusterCollection *cmv1.ClustersClient) error {
	if !reflect.DeepEqual(state.DefaultIngress, plan.DefaultIngress) {
		err := validateDefaultIngress(ctx, plan, state.Version.Value)
		if err != nil {
			return err
		}
		// In case default ingress was not part of state till now and we want to set
		// it as day2 we need to bring it first as we need to set specific id
		if state.DefaultIngress == nil || common.IsStringAttributeEmpty(state.DefaultIngress.Id) {
			state.DefaultIngress = &DefaultIngress{}
			err = populateDefaultIngress(ctx, state, clusterCollection.Cluster(plan.ID.Value).Ingresses())
			if err != nil {
				return err
			}
		}

		ingressBuilder := getDefaultIngressBuilder(plan)

		if !reflect.DeepEqual(state.DefaultIngress.ClusterRoutesHostname, plan.DefaultIngress.ClusterRoutesHostname) {
			ingressBuilder.ClusterRoutesHostname(plan.DefaultIngress.ClusterRoutesHostname.Value)
		}
		if !reflect.DeepEqual(state.DefaultIngress.ClusterRoutesTlsSecretRef, plan.DefaultIngress.ClusterRoutesTlsSecretRef) {
			ingressBuilder.ClusterRoutesTlsSecretRef(plan.DefaultIngress.ClusterRoutesTlsSecretRef.Value)
		}

		ingress, err := ingressBuilder.Build()
		if err != nil {
			return err
		}

		_, err = clusterCollection.Cluster(plan.ID.Value).Ingresses().Ingress(state.DefaultIngress.Id.Value).Update().
			Body(ingress).SendContext(ctx)

		if err != nil {
			return err
		}
	}

	return nil
}

func validateDefaultIngress(ctx context.Context, state *ClusterRosaClassicState, version string) error {
	if state.DefaultIngress == nil {
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
	ingress := state.DefaultIngress

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
