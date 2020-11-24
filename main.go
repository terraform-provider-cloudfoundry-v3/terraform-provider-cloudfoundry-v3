package main

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/plugin"
	cloudfoundry "github.com/terraform-providers/terraform-provider-cloudfoundry/cloudfoundryv3"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc: cloudfoundry.Provider,
	})
}
