package cloudfoundry_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccResServiceBindingWithAsyncPlan(t *testing.T) {

	space := testAccEnv.Space
	servicePlan := testAccEnv.ServicePlan

	src := `

		resource "cloudfoundry_app" "bind" {
			type = "buildpack"
			name = "foo-with-binding"
			space_id = %q
		}

		resource "cloudfoundry_service_instance" "bind" {
		  name = "bind"
		  space_id = %q
		  service_plan_id = %q
		}

		resource "cloudfoundry_service_binding" "bind" {
			app_id = cloudfoundry_app.bind.id
			service_instance_id = cloudfoundry_service_instance.bind.id
			params = jsonencode({
				ignored = true
			})
		}

	`

	refFakeAsyncPlan := "cloudfoundry_service_instance.bind"
	resource.Test(t,
		resource.TestCase{
			PreCheck:  func() { testAccPreCheck(t) },
			Providers: testAccProviders,
			CheckDestroy: testAccCheckServiceInstanceDestroyed(
				[]string{
					"bind",
				},
				refFakeAsyncPlan),
			Steps: []resource.TestStep{
				{
					Config: fmt.Sprintf(src, space.GUID, space.GUID, servicePlan.GUID),
					Check: resource.ComposeTestCheckFunc(
						testAccCheckServiceInstanceExists(refFakeAsyncPlan),
						resource.TestCheckResourceAttr(refFakeAsyncPlan, "name", "bind"),
					),
				},
			},
		},
	)
}
