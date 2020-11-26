package cloudfoundry_test

import (
	"fmt"
	"testing"

	"code.cloudfoundry.org/cli/api/cloudcontroller/ccv2"
	"code.cloudfoundry.org/cli/api/cloudcontroller/ccv2/constant"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccDataSourceOrg_normal(t *testing.T) {

	ref := "data.cloudfoundry_org.dd"
	org := testAccEnv.Organization

	src := `
		data "cloudfoundry_org" "dd" {
			name = %q
		}
	`

	resource.ParallelTest(t,
		resource.TestCase{
			PreCheck:  func() { testAccPreCheck(t) },
			Providers: testAccProviders,
			Steps: []resource.TestStep{
				{
					Config: fmt.Sprintf(src, org.Name),
					Check: resource.ComposeTestCheckFunc(
						checkDataSourceOrgExists(ref),
						resource.TestCheckResourceAttr(
							ref, "name", org.Name),
						resource.TestCheckResourceAttr(
							ref, "id", org.GUID),
					),
				},
			},
		})
}

func checkDataSourceOrgExists(resource string) resource.TestCheckFunc {

	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resource]
		if !ok {
			return fmt.Errorf("org '%s' not found in terraform state", resource)
		}

		id := rs.Primary.ID
		name := rs.Primary.Attributes["name"]

		orgs, _, err := testAccEnv.Session.ClientV2.GetOrganizations(
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
			return fmt.Errorf("org not found")
		}
		if id != orgs[0].GUID {
			return fmt.Errorf("id not match")
		}

		return nil
	}
}
