package cloudfoundry_test

import (
	"fmt"
	"testing"

	"code.cloudfoundry.org/cli/api/cloudcontroller/ccv3"
	"code.cloudfoundry.org/cli/api/cloudcontroller/ccv3/constant"
	"code.cloudfoundry.org/cli/resources"
	"code.cloudfoundry.org/cli/types"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccResAppBuildpackBasic(t *testing.T) {
	space := testAccEnv.Space
	appSourceZipPath := testAccEnv.AssetPath("dummy-app.zip")

	src := `
		resource "cloudfoundry_v3_app" "basic" {
			name = "basic-buildpack"
			space_id = %q
			state = "STARTED"

			lifecycle_type = "buildpack"
			buildpacks = ["binary_buildpack"]

			source_code_path = %q
			source_code_hash = %q

			environment = {
				VERSION = %q,
			}

			process {
				instances = %d
				memory_in_mb = 1024
				disk_in_mb = 1024
				healthcheck_type = "http"
				healthcheck_timeout = 111
				healthcheck_endpoint = "/"
			}
		}
	`

	var step1Droplet resources.Droplet
	var step2Droplet resources.Droplet
	var step3Droplet resources.Droplet
	var step4Droplet resources.Droplet

	resource.Test(t, resource.TestCase{
		PreCheck:     testAccPreCheck(t),
		Providers:    testAccProviders,
		CheckDestroy: appCheckDestroy,
		Steps: []resource.TestStep{

			// expect that an application can be deployed based on a specified
			// source zip + buildpack and that the basic process configuration
			// is reflected

			{
				Config: fmt.Sprintf(src, space.GUID, appSourceZipPath, "hash1", "1", 2),
				Check: resource.ComposeTestCheckFunc(
					appCopyDroplet("cloudfoundry_v3_app.basic", &step1Droplet),
					appCheckExists("cloudfoundry_v3_app.basic"),
					appCheckProcessByType("cloudfoundry_v3_app.basic", "web", resources.Process{
						HealthCheckType:     constant.HTTP,
						Instances:           types.NullInt{Value: 2},
						HealthCheckEndpoint: "/",
						HealthCheckTimeout:  111,
						MemoryInMB:          types.NullUint64{Value: 1024},
						DiskInMB:            types.NullUint64{Value: 1024},
					}),
					resource.TestCheckResourceAttr("cloudfoundry_v3_app.basic", "name", "basic-buildpack"),
					resource.TestCheckResourceAttr("cloudfoundry_v3_app.basic", "space_id", space.GUID),
					resource.TestCheckResourceAttr("cloudfoundry_v3_app.basic", "state", "STARTED"),
					resource.TestCheckResourceAttr("cloudfoundry_v3_app.basic", "environment.VERSION", "1"),
					resource.TestCheckResourceAttr("cloudfoundry_v3_app.basic", "process.0.type", "web"),
					resource.TestCheckResourceAttr("cloudfoundry_v3_app.basic", "process.0.instances", "2"),
					resource.TestCheckResourceAttr("cloudfoundry_v3_app.basic", "process.0.memory_in_mb", "1024"),
					resource.TestCheckResourceAttr("cloudfoundry_v3_app.basic", "process.0.disk_in_mb", "1024"),
					resource.TestCheckResourceAttr("cloudfoundry_v3_app.basic", "process.0.healthcheck_type", "http"),
				),
			},

			// expect that a change to environment triggers a new build,
			// a new droplet for the app

			{
				Config: fmt.Sprintf(src, space.GUID, appSourceZipPath, "hash1", "2", 2),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("cloudfoundry_v3_app.basic", "environment.VERSION", "2"),
					appCopyDroplet("cloudfoundry_v3_app.basic", &step2Droplet),
					appCheckDropletNotMatch(&step1Droplet, &step2Droplet),
				),
			},

			// expect change to source_code_hash to trigger a rebuild of the
			// package and trigger a deployment that changes the current
			// droplet

			{
				Config: fmt.Sprintf(src, space.GUID, appSourceZipPath, "hash2", "2", 2),
				Check: resource.ComposeTestCheckFunc(
					appCopyDroplet("cloudfoundry_v3_app.basic", &step3Droplet),
					appCheckDropletNotMatch(&step2Droplet, &step3Droplet),
				),
			},

			// expect that a change to process instance scaling should NOT
			// trigger a deployment or cause droplet change

			{
				Config: fmt.Sprintf(src, space.GUID, appSourceZipPath, "hash2", "2", 1),
				Check: resource.ComposeTestCheckFunc(
					appCopyDroplet("cloudfoundry_v3_app.basic", &step4Droplet),
					appCheckDropletMatch(&step3Droplet, &step4Droplet),
					appCheckProcessByType("cloudfoundry_v3_app.basic", "web", resources.Process{
						HealthCheckType:     constant.HTTP,
						Instances:           types.NullInt{Value: 1},
						HealthCheckEndpoint: "/",
						HealthCheckTimeout:  111,
						MemoryInMB:          types.NullUint64{Value: 1024},
						DiskInMB:            types.NullUint64{Value: 1024},
					}),
				),
			},
		},
	})
}

func TestAccResAppDockerBasic(t *testing.T) {
	space := testAccEnv.Space

	src := `
		resource "cloudfoundry_v3_app" "basic" {
			name = "basic-docker"
			space_id = %q
			state = "STARTED"

			lifecycle_type = "docker"
			docker_image = "cloudfoundry/diego-docker-app:latest"

			environment = {
				VERSION = %q,
			}

			process {
				instances = 2
				memory_in_mb = 1024
				disk_in_mb = 1024
				healthcheck_timeout = 112
				healthcheck_type = "process"
			}
		}
	`

	var step1Droplet resources.Droplet
	var step2Droplet resources.Droplet

	resource.Test(t, resource.TestCase{
		PreCheck:     testAccPreCheck(t),
		Providers:    testAccProviders,
		CheckDestroy: appCheckDestroy,
		Steps: []resource.TestStep{

			// expect that an application with docker lifecycle type can be deployed based on
			// a docker hub image and that the basic process config is reflected

			{
				Config: fmt.Sprintf(src, space.GUID, "1"),
				Check: resource.ComposeTestCheckFunc(
					appCopyDroplet("cloudfoundry_v3_app.basic", &step1Droplet),
					appCheckExists("cloudfoundry_v3_app.basic"),
					appCheckProcessByType("cloudfoundry_v3_app.basic", "web", resources.Process{
						Instances:           types.NullInt{Value: 2},
						HealthCheckType:     constant.Process,
						HealthCheckEndpoint: "",
						HealthCheckTimeout:  112,
						MemoryInMB:          types.NullUint64{Value: 1024},
						DiskInMB:            types.NullUint64{Value: 1024},
					}),
					resource.TestCheckResourceAttr("cloudfoundry_v3_app.basic", "name", "basic-docker"),
					resource.TestCheckResourceAttr("cloudfoundry_v3_app.basic", "space_id", space.GUID),
					resource.TestCheckResourceAttr("cloudfoundry_v3_app.basic", "state", "STARTED"),
					resource.TestCheckResourceAttr("cloudfoundry_v3_app.basic", "docker_image", "cloudfoundry/diego-docker-app:latest"),
					resource.TestCheckResourceAttr("cloudfoundry_v3_app.basic", "environment.VERSION", "1"),
					resource.TestCheckResourceAttr("cloudfoundry_v3_app.basic", "process.0.type", "web"),
					resource.TestCheckResourceAttr("cloudfoundry_v3_app.basic", "process.0.instances", "2"),
					resource.TestCheckResourceAttr("cloudfoundry_v3_app.basic", "process.0.memory_in_mb", "1024"),
					resource.TestCheckResourceAttr("cloudfoundry_v3_app.basic", "process.0.disk_in_mb", "1024"),
					resource.TestCheckResourceAttr("cloudfoundry_v3_app.basic", "process.0.healthcheck_type", "process"),
				),
			},

			// expect that a change to the environment will trigger a rolling
			// restart by creating a new droplet

			{
				Config: fmt.Sprintf(src, space.GUID, "2"),
				Check: resource.ComposeTestCheckFunc(
					appCopyDroplet("cloudfoundry_v3_app.basic", &step2Droplet),
					appCheckDropletNotMatch(&step1Droplet, &step2Droplet),
					resource.TestCheckResourceAttr("cloudfoundry_v3_app.basic", "environment.VERSION", "2"),
				),
			},
		},
	})
}

func appCheckDestroy(s *terraform.State) error {
	errs := []error{}
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "cloudfoundry_v3_app" {
			continue
		}

		apps, _, err := testAccEnv.Session.ClientV3.GetApplications(
			ccv3.Query{Key: ccv3.GUIDFilter, Values: []string{rs.Primary.ID}},
		)
		if err != nil {
			return err
		}
		if len(apps) > 0 {
			defer func(guid string) {
				// trigger a delete to try and tidy up...
				_, _, _ = testAccEnv.Session.ClientV3.DeleteApplication(guid)
			}(rs.Primary.ID)
			errs = append(errs, fmt.Errorf("expected app to have been deleted but found: %s", rs.Primary.ID))
		}

	}

	if len(errs) > 0 {
		return errs[0]
	}

	return nil
}

func appCheckExists(n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]

		if !ok {
			return fmt.Errorf("not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("no id set")
		}

		apps, _, err := testAccEnv.Session.ClientV3.GetApplications(
			ccv3.Query{Key: ccv3.GUIDFilter, Values: []string{rs.Primary.ID}},
		)
		if err != nil {
			return err
		}
		if len(apps) != 1 {
			return fmt.Errorf("expected to find exactly 1 app with guid %s got %d", rs.Primary.ID, len(apps))
		}

		return nil
	}
}

func appCheckDropletNotMatch(aDroplet, bDroplet *resources.Droplet) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if aDroplet.GUID == "" {
			return fmt.Errorf("expected aDroplet to have a GUID")
		}
		if bDroplet.GUID == "" {
			return fmt.Errorf("expected bDroplet to have a GUID")
		}
		if aDroplet.GUID == bDroplet.GUID {
			return fmt.Errorf("expected droplet to have changed")
		}
		return nil
	}
}

func appCheckDropletMatch(aDroplet, bDroplet *resources.Droplet) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if aDroplet.GUID == "" {
			return fmt.Errorf("expected aDroplet to have a GUID")
		}
		if bDroplet.GUID == "" {
			return fmt.Errorf("expected bDroplet to have a GUID")
		}
		if aDroplet.GUID != bDroplet.GUID {
			return fmt.Errorf("expected droplet to remain unchanged")
		}
		return nil
	}
}

func appCopyDroplet(n string, dstDroplet *resources.Droplet) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs := s.RootModule().Resources[n]

		droplet, _, err := testAccEnv.Session.ClientV3.GetApplicationDropletCurrent(rs.Primary.ID)
		if err != nil {
			return err
		}
		*dstDroplet = droplet

		return nil
	}
}

func appCheckProcessByType(n string, procType string, expectedProc resources.Process) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]

		if !ok {
			return fmt.Errorf("not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("no id set")
		}

		proc, _, err := testAccEnv.Session.ClientV3.GetApplicationProcessByType(
			rs.Primary.ID,
			procType,
		)
		if err != nil {
			return err
		}

		if proc.Instances.Value != expectedProc.Instances.Value {
			return fmt.Errorf("expected %s instances to be %d got %d", procType, expectedProc.Instances.Value, proc.Instances.Value)
		}

		if proc.DiskInMB.Value != expectedProc.DiskInMB.Value {
			return fmt.Errorf("expected %s disk to be %d got %d", procType, expectedProc.DiskInMB.Value, proc.DiskInMB.Value)
		}

		if proc.MemoryInMB.Value != expectedProc.MemoryInMB.Value {
			return fmt.Errorf("expected %s proc memory to be %d got %d", procType, expectedProc.MemoryInMB.Value, proc.MemoryInMB.Value)
		}

		if proc.HealthCheckEndpoint != expectedProc.HealthCheckEndpoint {
			return fmt.Errorf("expected %s proc healthcheck endpoint to be %q got %q", procType, expectedProc.HealthCheckEndpoint, proc.HealthCheckEndpoint)
		}

		if proc.HealthCheckType != expectedProc.HealthCheckType {
			return fmt.Errorf("expected the %s proc healthcheck type to be %q but got %q", procType, expectedProc.HealthCheckType, proc.HealthCheckType)
		}

		if proc.HealthCheckTimeout != expectedProc.HealthCheckTimeout {
			return fmt.Errorf("expected the %s proc healthcheck timeout to be %q but got %q", procType, expectedProc.HealthCheckTimeout, proc.HealthCheckTimeout)
		}

		return nil
	}
}
