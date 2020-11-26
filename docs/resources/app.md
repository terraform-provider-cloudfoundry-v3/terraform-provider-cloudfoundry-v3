---
layout: "cloudfoundry"
page_title: "Cloud Foundry: cloudfoundry_app"
sidebar_current: "docs-cf-resource-app"
description: |-
  Provides a Cloud Foundry Application resource.
---

# cloudfoundry_app

Provides a Cloud Foundry [application](https://docs.cloudfoundry.org/devguide/deploy-apps/deploy-app.html) resource.

## Example Usage

The following example creates an application. The created application is
stopped and it does not stage or deploy your application source.

To build your application droplet and have it deployed to your application
see the `cloudfoundry_droplet` and `cloudfoundry_deployment`
resources.

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
```

## Argument Reference

The following arguments are supported:

* `name` - (Required) The name of the application.
* `space_id` - (Required) The GUID of the associated Cloud Foundry space.
* `instances` - (Optional, Number) The number of app instances that you want to start. Defaults to 1.
* `memory_in_mb` - (Optional, Number) The memory limit for each application instance in megabytes. If not provided, value is computed and retreived from Cloud Foundry.
* `disk_in_mb` - (Optional, Number) The disk space to be allocated for each application instance in megabytes. If not provided, default disk quota is retrieved from Cloud Foundry and assigned.
* `command` - (Optional, String) A custom start command for the application's web process. This overrides the start command provided by the buildpack.
* `health_check_type` - (Optional, String) One of `port`, `process` or `http`
* `health_check_endpoint` - (Optional, String) defaults to "/" set to a path to your healthcheck (only valid for `http` type checks)
* `environment` - (Optional, map of String to string) environment variables for your application processes.


## Attributes Reference

The following attributes are exported along with any defaults for the inputs attributes.

* `id` - The GUID of the application

