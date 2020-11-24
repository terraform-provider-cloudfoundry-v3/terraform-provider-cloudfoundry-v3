package cloudfoundry

import (
	"context"
	"fmt"

	"code.cloudfoundry.org/cli/api/cloudcontroller/ccv3"
	"code.cloudfoundry.org/cli/resources"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/terraform-providers/terraform-provider-cloudfoundry/cloudfoundryv3/managers"
)

func resourceRouteDestination() *schema.Resource {

	return &schema.Resource{

		CreateContext: resourceRouteDestinationCreate,
		ReadContext:   resourceRouteDestinationRead,
		DeleteContext: resourceRouteDestinationDelete,

		// Importer: &schema.ResourceImporter{
		// 	State: ImportRead(resourceRouteRead),
		// },

		Schema: map[string]*schema.Schema{

			"route_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"app_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			// "process_type": { // not implemented yet
			// 	Type:     schema.TypeString,
			// 	Optional: true,
			// 	Default:  "web",
			// 	ForceNew: true,
			// },

			// "port": { // not implemented yet
			// 	Type:     schema.TypeInt,
			// 	Optional: true,
			// 	ForceNew: true,
			// },
		},
	}
}

func resourceRouteDestinationCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	session := meta.(*managers.Session)
	routeGUID := d.Get("route_id").(string)
	appGUID := d.Get("app_id").(string)

	warns, err := session.ClientV3.MapRoute(routeGUID, appGUID)
	diags = append(diags, diagFromClient("map-route-destination", warns, err)...)
	if diags.HasError() {
		return diags
	}
	destination, errs := resourceRouteDestinationGet(ctx, d, meta)
	diags = append(diags, errs...)
	if diags.HasError() {
		return diags
	}
	if destination == nil {
		return diag.FromErr(fmt.Errorf("unable to find the destination we just mapped for route:%s app:%s", routeGUID, appGUID))
	}
	d.SetId(destination.GUID)
	return nil
}

func resourceRouteDestinationGet(ctx context.Context, d *schema.ResourceData, meta interface{}) (_ *resources.RouteDestination, diags diag.Diagnostics) {
	session := meta.(*managers.Session)
	routeGUID := d.Get("route_id").(string)
	appGUID := d.Get("app_id").(string)

	// FIXME: remove this once there is a ccv3.GetRoute
	v2Route, _, err := session.ClientV2.GetRoute(routeGUID)
	if err != nil {
		return nil, diag.FromErr(err)
	}

	routes, warns, err := session.ClientV3.GetRoutes(
		ccv3.Query{Key: ccv3.DomainGUIDFilter, Values: []string{v2Route.DomainGUID}},
		ccv3.Query{Key: ccv3.SpaceGUIDFilter, Values: []string{v2Route.SpaceGUID}},
		ccv3.Query{Key: ccv3.HostsFilter, Values: []string{v2Route.Host}},
		ccv3.Query{Key: ccv3.PathsFilter, Values: []string{v2Route.Path}},
	)
	diags = append(diags, diagFromClient("get-routes-for-destination", warns, err)...)
	if diags.HasError() {
		return nil, diags
	}
	if len(routes) != 1 {
		return nil, diags
	}
	var destination *resources.RouteDestination
	for _, d := range routes[0].Destinations {
		if d.App.GUID == appGUID {
			destination = &d
			break
		}
	}
	if destination == nil {
		return nil, diags
	}

	return destination, diags
}

func resourceRouteDestinationRead(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	destination, errs := resourceRouteDestinationGet(ctx, d, meta)
	diags = append(diags, errs...)
	if diags.HasError() {
		return diags
	}
	if destination == nil {
		d.SetId("")
		return diags
	}

	return nil
}

func resourceRouteDestinationDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	session := meta.(*managers.Session)
	routeGUID := d.Get("route_id").(string)
	destinationGUID := d.Id()

	warns, err := session.ClientV3.UnmapRoute(routeGUID, destinationGUID)
	diags = append(diags, diagFromClient("remove-route-destination", warns, err)...)
	if diags.HasError() {
		return diags
	}

	return diags
}
