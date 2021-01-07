---
layout: "cloudfoundry"
page_title: "Cloud Foundry: cloudfoundry_deployment"
sidebar_current: "docs-cf-resource-deployment"
description: |-
  Provides a Cloud Foundry Deployment resource.
---

# cloudfoundry_deployment

Provides a Cloud Foundry [application](https://docs.cloudfoundry.org/devguide/deploy-apps/deploy-app.html) resource.

## Example Usage

The following example creates an application, stages a droplet and deploys it with the "rolling" (zero downtime) strategy.

```hcl
resource "cloudfoundry_app" "basic" {
	provider              = cloudfoundry-v3
	name                  = "basic-buildpack"
	space_id              = data.cloudfoundry_space.myspace.id
	environment           = {MY_VAR = "1"}
	instances             = 2
	memory_in_mb          = 1024
	disk_in_mb            = 1024
	health_check_type     = "http"
	health_check_endpoint = "/"
}

resource "cloudfoundry_droplet" "basic" {
	provider         = cloudfoundry-v3
	app_id           = cloudfoundry_app.basic.id
	buildpacks       = ["binary_buildpack"]
	environment      = cloudfoundry_app.basic.environment
	command          = cloudfoundry_app.basic.command
	source_code_path = "/path/to/source.zip"
	source_code_hash = filemd5("/path/to/source.zip")
}

resource "cloudfoundry_deployment" "basic" {
	provider   = cloudfoundry-v3
	strategy   = "rolling"
	app_id     = cloudfoundry_app.basic.id
	droplet_id = cloudfoundry_droplet.basic.id
}
```

## Argument Reference

The following arguments are supported:

* `app_id` - (Required) The GUID of the associated Cloud Foundry application
* `droplet_id` - (Required) The GUID of the application droplet to deploy.
* `strategy` - (Required) The deployment method. Currently only `rolling` supported.
