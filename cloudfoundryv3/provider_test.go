package cloudfoundry_test

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	cloudfoundry "github.com/terraform-providers/terraform-provider-cloudfoundry/cloudfoundryv3"
	"github.com/terraform-providers/terraform-provider-cloudfoundry/cloudfoundryv3/testfoundry"
)

var testAccProviders map[string]*schema.Provider
var testAccProvider *schema.Provider
var testAccEnv *testfoundry.TestEnv

func init() {
	testAccProvider = cloudfoundry.Provider()
	testAccProviders = map[string]*schema.Provider{
		"cloudfoundry": testAccProvider,
	}
	testAccEnv = testfoundry.NewTestEnv()
}

func TestProvider(t *testing.T) {
	if err := cloudfoundry.Provider().InternalValidate(); err != nil {
		t.Fatalf("err: %s", err)
	}
}

func TestProvider_impl(t *testing.T) {
	var _ *schema.Provider = cloudfoundry.Provider()
}

func testAccPreCheck(t *testing.T) func() {
	return func() {
		if err := os.Getenv("CF_API_URL"); err == "" {
			t.Fatal("CF_API_URL must be set for acceptance tests")
		}
		if err := os.Getenv("CF_USER"); err == "" {
			t.Fatal("CF_USER must be set for acceptance tests")
		}
		if err := os.Getenv("CF_PASSWORD"); err == "" {
			t.Fatal("CF_PASSWORD must be set for acceptance tests")
		}
		if err := os.Getenv("TEST_SPACE_NAME"); err == "" {
			t.Fatal("TEST_SPACE_NAME must be set for acceptance tests")
		}
	}
}
