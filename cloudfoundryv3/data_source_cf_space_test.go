package cloudfoundry_test

import (
	"fmt"
	"testing"

	"code.cloudfoundry.org/cli/api/cloudcontroller/ccv2"
	"code.cloudfoundry.org/cli/api/cloudcontroller/ccv2/constant"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccDataSourceSpace_normal(t *testing.T) {

	ref := "data.cloudfoundry_space.default"

	org := testAccEnv.Organization
	space := testAccEnv.Space

	src := `
		data "cloudfoundry_space" "default" {
			name = %q
			org_name = %q
		}
	`

	resource.ParallelTest(t,
		resource.TestCase{
			PreCheck:  func() { testAccPreCheck(t) },
			Providers: testAccProviders,
			Steps: []resource.TestStep{
				{
					Config: fmt.Sprintf(src, space.Name, org.Name),
					Check: resource.ComposeTestCheckFunc(
						checkDataSourceSpaceExists(ref),
						resource.TestCheckResourceAttr(
							ref, "name", space.Name),
						resource.TestCheckResourceAttr(
							ref, "org_name", org.Name),
						resource.TestCheckResourceAttr(
							ref, "org", org.GUID),
					),
				},
			},
		})
}

func checkDataSourceSpaceExists(resource string) resource.TestCheckFunc {

	return func(s *terraform.State) error {

		session := testAccEnv.Session

		rs, ok := s.RootModule().Resources[resource]
		if !ok {
			return fmt.Errorf("space '%s' not found in terraform state", resource)
		}

		id := rs.Primary.ID
		name := rs.Primary.Attributes["name"]
		org := rs.Primary.Attributes["org"]

		spaces, _, err := session.ClientV2.GetSpaces(
			ccv2.Filter{
				Type:     constant.NameFilter,
				Operator: constant.EqualOperator,
				Values:   []string{name},
			},
			ccv2.Filter{
				Type:     constant.OrganizationGUIDFilter,
				Operator: constant.EqualOperator,
				Values:   []string{org},
			},
		)
		if err != nil {
			return err
		}
		if len(spaces) == 0 {
			return fmt.Errorf("space not found")
		}
		if id != spaces[0].GUID {
			return fmt.Errorf("id not match")
		}

		return nil
	}
}
