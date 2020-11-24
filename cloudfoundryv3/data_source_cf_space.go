package cloudfoundry

import (
	"fmt"

	"code.cloudfoundry.org/cli/api/cloudcontroller/ccv2"
	"code.cloudfoundry.org/cli/api/cloudcontroller/ccv2/constant"
	"github.com/terraform-providers/terraform-provider-cloudfoundry/cloudfoundryv3/managers"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceSpace() *schema.Resource {

	return &schema.Resource{

		Read: dataSourceSpaceRead,

		Schema: map[string]*schema.Schema{

			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"org_name": {
				Type:          schema.TypeString,
				Optional:      true,
				Computed:      true,
				ConflictsWith: []string{"org"},
			},
			"org": {
				Type:          schema.TypeString,
				Optional:      true,
				Computed:      true,
				ConflictsWith: []string{"org_name"},
			},
			"quota_id": {
				Type:     schema.TypeString,
				Computed: true,
			},
			// labelsKey:      labelsSchema(),
			// annotationsKey: annotationsSchema(),
		},
	}
}

func dataSourceSpaceRead(d *schema.ResourceData, meta interface{}) (err error) {

	session := meta.(*managers.Session)
	if session == nil {
		return fmt.Errorf("client is nil")
	}

	name := d.Get("name").(string)

	if d.Get("org").(string) == "" && d.Get("org_name").(string) == "" {
		return fmt.Errorf("You must provide either 'org' or 'org_name' attribute")
	}

	orgId := d.Get("org").(string)
	orgName := d.Get("org_name").(string)
	if d.Get("org_name").(string) != "" {
		orgs, _, err := session.ClientV2.GetOrganizations(
			ccv2.Filter{
				Type:     constant.NameFilter,
				Operator: constant.EqualOperator,
				Values:   []string{orgName},
			},
		)
		if err != nil {
			return err
		}
		if len(orgs) == 0 {
			return fmt.Errorf("Can't found org with name %s", orgName)
		}
		orgId = orgs[0].GUID
	} else {
		org, _, err := session.ClientV2.GetOrganization(orgId)
		if err != nil {
			return err
		}
		orgName = org.Name
	}
	spaces, _, err := session.ClientV2.GetSpaces(
		ccv2.Filter{
			Type:     constant.NameFilter,
			Operator: constant.EqualOperator,
			Values:   []string{name},
		},
		ccv2.Filter{
			Type:     constant.OrganizationGUIDFilter,
			Operator: constant.EqualOperator,
			Values:   []string{orgId},
		},
	)
	if err != nil {
		return err
	}
	if len(spaces) == 0 {
		return NotFound
	}
	space := spaces[0]
	d.SetId(space.GUID)
	_ = d.Set("org_name", orgName)
	_ = d.Set("org", orgId)
	_ = d.Set("quota_id", space.SpaceQuotaDefinitionGUID)

	// err = metadataRead(spaceMetadata, d, meta, true)
	// if err != nil {
	// 	return err
	// }
	return err
}
