package cloudfoundry_test

import (
	"fmt"
	"testing"

	"code.cloudfoundry.org/cli/api/cloudcontroller/ccv2"
	"code.cloudfoundry.org/cli/api/cloudcontroller/ccv2/constant"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/terraform-providers/terraform-provider-cloudfoundry/cloudfoundryv3/managers"
)

func TestAccResServiceInstancesWithAsyncPlan(t *testing.T) {

	space := testAccEnv.Space
	servicePlan := testAccEnv.ServicePlan

	refFakeAsyncPlan := "cloudfoundry_service_instance.async"
	resource.Test(t,
		resource.TestCase{
			PreCheck:  func() { testAccPreCheck(t) },
			Providers: testAccProviders,
			CheckDestroy: testAccCheckServiceInstanceDestroyed(
				[]string{
					"async",
				},
				refFakeAsyncPlan),
			Steps: []resource.TestStep{
				{
					Config: fmt.Sprintf(`
						resource "cloudfoundry_service_instance" "async" {
						  name = "async"
						  space_id = "%s"
						  service_plan_id = "%s"
						}
					`, space.GUID, servicePlan.GUID),
					Check: resource.ComposeTestCheckFunc(
						testAccCheckServiceInstanceExists(refFakeAsyncPlan),
						resource.TestCheckResourceAttr(refFakeAsyncPlan, "name", "async"),
					),
				},
			},
		},
	)
}

func testAccCheckServiceInstanceExists(resource string) resource.TestCheckFunc {

	return func(s *terraform.State) error {

		session := testAccProvider.Meta().(*managers.Session)

		rs, ok := s.RootModule().Resources[resource]
		if !ok {
			return fmt.Errorf("service instance '%s' not found in terraform state", resource)
		}

		id := rs.Primary.ID
		_, _, err := session.ClientV2.GetServiceInstance(id)
		if err != nil {
			return err
		}

		return nil
	}
}

func testAccCheckServiceInstanceDestroyed(names []string, testResource string) resource.TestCheckFunc {

	return func(s *terraform.State) error {
		session := testAccProvider.Meta().(*managers.Session)
		rs, ok := s.RootModule().Resources[testResource]
		if !ok {
			return fmt.Errorf("the service instance '%s' not found in terraform state", testResource)
		}

		for _, n := range names {
			sis, _, err := session.ClientV2.GetServiceInstances(
				ccv2.Filter{
					Type:     constant.NameFilter,
					Operator: constant.EqualOperator,
					Values:   []string{n},
				},
				ccv2.Filter{
					Type:     constant.SpaceGUIDFilter,
					Operator: constant.EqualOperator,
					Values:   []string{rs.Primary.Attributes["space_id"]},
				},
			)
			if err != nil {
				return err
			}
			if len(sis) > 0 {
				return fmt.Errorf("service instance with name '%s' still exists in cloud foundry", n)
			}
		}
		return nil
	}
}
