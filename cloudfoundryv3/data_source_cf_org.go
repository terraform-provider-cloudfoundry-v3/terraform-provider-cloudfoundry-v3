package cloudfoundry

import (
	"fmt"

	"code.cloudfoundry.org/cli/api/cloudcontroller/ccv2"
	"code.cloudfoundry.org/cli/api/cloudcontroller/ccv2/constant"
	"github.com/terraform-providers/terraform-provider-cloudfoundry/cloudfoundryv3/managers"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceOrg() *schema.Resource {

	return &schema.Resource{

		Read: dataSourceOrgRead,

		Schema: map[string]*schema.Schema{

			"name": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			// labelsKey:      labelsSchema(),
			// annotationsKey: annotationsSchema(),
		},
	}
}

func dataSourceOrgRead(d *schema.ResourceData, meta interface{}) error {

	session := meta.(*managers.Session)
	if session == nil {
		return fmt.Errorf("client is nil")
	}

	name := d.Get("name").(string)

	orgs, _, err := session.ClientV2.GetOrganizations(
		ccv2.Filter{
			Type:     constant.NameFilter,
			Operator: constant.EqualOperator,
			Values:   []string{name},
		},
	)
	if err != nil {
		return err
	}

	if len(orgs) == 0 {
		return NotFound
	}
	d.SetId(orgs[0].GUID)

	// err = metadataRead(orgMetadata, d, meta, true)
	// if err != nil {
	// 	return err
	// }
	return err
}
