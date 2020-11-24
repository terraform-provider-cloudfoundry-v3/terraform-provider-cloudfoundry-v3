package cloudfoundry

import (
	"fmt"
	"strings"

	"code.cloudfoundry.org/cli/resources"
	"github.com/terraform-providers/terraform-provider-cloudfoundry/cloudfoundryv3/managers"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceDomain() *schema.Resource {

	return &schema.Resource{

		Read: dataSourceDomainRead,

		Schema: map[string]*schema.Schema{

			"name": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"sub_domain": &schema.Schema{
				Type:          schema.TypeString,
				Optional:      true,
				ConflictsWith: []string{"name"},
			},
			"domain": &schema.Schema{
				Type:          schema.TypeString,
				Optional:      true,
				Computed:      true,
				ConflictsWith: []string{"name"},
			},
			"org": &schema.Schema{
				Type:     schema.TypeString,
				Computed: true,
			},
			"internal": &schema.Schema{
				Type:     schema.TypeBool,
				Computed: true,
			},
		},
	}
}

func dataSourceDomainRead(d *schema.ResourceData, meta interface{}) error {

	session := meta.(*managers.Session)
	if session == nil {
		return fmt.Errorf("client is nil")
	}

	var (
		name, prefix string
	)

	domains, _, err := session.ClientV3.GetDomains()
	if err != nil {
		return err
	}

	if v, ok := d.GetOk("sub_domain"); ok {
		prefix = v.(string) + "."
		if v, ok = d.GetOk("domain"); ok {
			name = prefix + v.(string)
		}
	} else if v, ok := d.GetOk("name"); ok {
		name = v.(string)
	} else {
		return fmt.Errorf("neither a full name or sub-domain was provided to do an effective domain search")
	}

	var domain *resources.Domain
	if len(name) == 0 {
		for _, d := range domains {
			if strings.HasPrefix(d.Name, prefix) {
				domain = &d
				break
			}
		}
		if domain == nil {
			return fmt.Errorf("no domain found with sub-domain '%s'", prefix)
		}
	} else {
		for _, d := range domains {
			if name == d.Name {
				domain = &d
				break
			}
		}
		if domain == nil {
			return fmt.Errorf("no domain found with name '%s'", name)
		}
	}

	domainParts := strings.Split(domain.Name, ".")

	_ = d.Set("name", domain.Name)
	_ = d.Set("sub_domain", domainParts[0])
	_ = d.Set("domain", strings.Join(domainParts[1:], "."))
	_ = d.Set("org", domain.OrganizationGUID)
	_ = d.Set("internal", domain.Internal.Value)
	d.SetId(domain.GUID)
	return err
}
