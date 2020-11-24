package cloudfoundry

import (
	"context"

	"code.cloudfoundry.org/cli/api/cloudcontroller/ccv3"
	"code.cloudfoundry.org/cli/resources"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/terraform-providers/terraform-provider-cloudfoundry/cloudfoundryv3/managers"
)

func resourceRoute() *schema.Resource {

	return &schema.Resource{

		CreateContext: resourceRouteCreate,
		ReadContext:   resourceRouteRead,
		DeleteContext: resourceRouteDelete,

		// Importer: &schema.ResourceImporter{
		// 	State: ImportRead(resourceRouteRead),
		// },

		Schema: map[string]*schema.Schema{

			"domain_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"space_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"host": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},

			// 			"port": { // tcp routes not implemented
			// 				Type:          schema.TypeInt,
			// 				Optional:      true,
			// 				Computed:      true,
			// 			},

			"path": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
		},
	}
}

func resourceRouteCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	session := meta.(*managers.Session)

	route, warns, err := session.ClientV3.CreateRoute(resources.Route{
		DomainGUID: d.Get("domain_id").(string),
		SpaceGUID:  d.Get("space_id").(string),
		Host:       d.Get("host").(string),
		Path:       d.Get("path").(string),
	})
	diags = append(diags, diagFromClient("create-route", warns, err)...)
	if diags.HasError() {
		return diags
	}

	d.SetId(route.GUID)
	return diags
}

func resourceRouteRead(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	session := meta.(*managers.Session)

	routes, warns, err := session.ClientV3.GetRoutes(
		ccv3.Query{Key: ccv3.DomainGUIDFilter, Values: []string{d.Get("domain_id").(string)}},
		ccv3.Query{Key: ccv3.SpaceGUIDFilter, Values: []string{d.Get("space_id").(string)}},
		ccv3.Query{Key: ccv3.HostsFilter, Values: []string{d.Get("host").(string)}},
		ccv3.Query{Key: ccv3.PathsFilter, Values: []string{d.Get("path").(string)}},
	)
	diags = append(diags, diagFromClient("get-routes", warns, err)...)
	if diags.HasError() {
		return diags
	}
	if len(routes) != 1 {
		d.SetId("")
		return diags
	}
	return diags
}

func resourceRouteDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	session := meta.(*managers.Session)

	_, warns, err := session.ClientV3.DeleteRoute(d.Id())
	diags = append(diags, diagFromClient("delete-route", warns, err)...)
	if diags.HasError() {
		return diags
	}

	return diags
}
