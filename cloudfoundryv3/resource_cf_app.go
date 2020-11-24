package cloudfoundry

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"time"

	"code.cloudfoundry.org/cli/api/cloudcontroller/ccv3"
	"code.cloudfoundry.org/cli/api/cloudcontroller/ccv3/constant"
	"code.cloudfoundry.org/cli/resources"
	"code.cloudfoundry.org/cli/types"
	"code.cloudfoundry.org/cli/util/manifest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/terraform-providers/terraform-provider-cloudfoundry/cloudfoundryv3/managers"
	"gopkg.in/yaml.v2"
)

func resourceApp() *schema.Resource {
	return &schema.Resource{

		CreateContext: resourceAppCreate,
		ReadContext:   resourceAppRead,
		UpdateContext: resourceAppUpdate,
		DeleteContext: resourceAppDelete,

		// Importer: &schema.ResourceImporter{
		// 	State: resourceAppImport,
		// },
		Schema: map[string]*schema.Schema{

			"space_id": {
				Description: "The GUID of the parent Space to place this application in",
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
			},

			"name": {
				Description: "Name of the application. Names must be unique within a Space",
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
			},

			"lifecycle_type": {
				Description:  "The lifecycle type of the source. There are two types (lifecycles) of cloudfoundry application builds, 'buildpack' and 'docker'. For buildpack source types, you must supply `source_code_path` to a zip of application source code. For the 'docker' source type, you must supply the `docker_image`.",
				Type:         schema.TypeString,
				Optional:     true,
				Computed:     true,
				ValidateFunc: validation.StringInSlice([]string{"buildpack", "docker", "kpack"}, false),
				ForceNew:     true,
			},

			"source_code_path": {
				Description:  "Path to a zip of the application source code. Required if type is 'buildpack'",
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringIsNotEmpty,
			},

			"source_code_hash": {
				Description:   "Set this to a sum of the source_code data to trigger deployments on changes",
				Type:          schema.TypeString,
				Optional:      true,
				ValidateFunc:  validation.StringIsNotEmpty,
				ConflictsWith: []string{"docker_image"},
			},

			"buildpacks": {
				Description: "A list of the names of buildpacks, URLs from which they may be downloaded",
				Type:        schema.TypeList,
				Optional:    true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				ConflictsWith: []string{"docker_image"},
			},

			"stack": {
				Description:   "The root filesystem to use with the buildpack, for example cflinuxfs3",
				Type:          schema.TypeString,
				Optional:      true,
				Default:       "cflinuxfs3",
				ValidateFunc:  validation.StringIsNotEmpty,
				ConflictsWith: []string{"docker_image"},
			},

			"docker_image": {
				Description:   "The docker image to use. Required if lifecycle type is 'docker'",
				Type:          schema.TypeString,
				Optional:      true,
				ValidateFunc:  validation.StringIsNotEmpty,
				ConflictsWith: []string{"stack", "buildpacks", "source_code_path"},
			},

			"strategy": {
				Description:  "Strategy used for the deployment",
				Type:         schema.TypeString,
				Optional:     true,
				Default:      "rolling",
				ValidateFunc: validation.StringInSlice([]string{"rolling"}, false),
			},

			"command": {
				Description: "The command used to start the process; this overrides start commands from Procfiles and buildpacks",
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Sensitive:   true,
			},

			"health_check_type": {
				Description:  "Type of health check to perform; one of: port, process or http",
				Type:         schema.TypeString,
				Optional:     true,
				Default:      "port",
				ValidateFunc: validation.StringInSlice([]string{"port", "process", "http"}, false),
			},

			"health_check_endpoint": {
				Description: "HTTP endpoint called to determine if the app is healthy. (valid only when type is http)",
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
			},

			"health_check_timeout": {
				Description: "timeout waiting for healthcheck response",
				Type:        schema.TypeInt,
				Optional:    true,
				Computed:    true,
			},

			"instances": {
				Description: "The number of instances of this application's web process to run",
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     1,
			},

			"memory_in_mb": {
				Description:  "The memory limit in mb for all instances of the web process",
				Type:         schema.TypeInt,
				Optional:     true,
				Default:      1024,
				ValidateFunc: validation.IntAtLeast(64),
			},

			"disk_in_mb": {
				Description:  "The disk limit in mb allocated per instance",
				Type:         schema.TypeInt,
				Optional:     true,
				Default:      1024,
				ValidateFunc: validation.IntAtLeast(64),
			},

			"service": {
				Description: "Configure service bindings between the app and a service instance.",
				Type:        schema.TypeSet,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"service_instance_id": {
							Description: "The GUID of the service instance to bind to this app",
							Type:        schema.TypeString,
							Required:    true,
						},
						"parameters": {
							Description:  "A JSON blob of arbitrary key/value pairs to send to the service broker during binding",
							Type:         schema.TypeString,
							Optional:     true,
							Default:      "{}",
							ValidateFunc: validation.StringIsJSON,
						},
					},
				},
				Optional: true,
			},

			"state": {
				Description:  "Current desired state of the app; valid values are STOPPED or STARTED",
				Type:         schema.TypeString,
				Optional:     true,
				Default:      "STARTED",
				ValidateFunc: validation.StringInSlice([]string{"STARTED", "STOPPED"}, false),
			},

			"environment": {
				Description: "The environment variables associated with the given app. Environment variable names may not start with VCAP_. PORT is not a valid environment variable.",
				Type:        schema.TypeMap,
				Optional:    true,
				Sensitive:   true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
					ValidateFunc: validation.StringDoesNotMatch(
						regexp.MustCompile(`(^VCAP_|^PORT$)`),
						"Environment variables named 'PORT' or starting with 'VCAP_' are reserved",
					),
				},
			},
		},
	}
}

func diagFromClient(detail string, warns ccv3.Warnings, err error) diag.Diagnostics {
	var diags diag.Diagnostics
	for _, warn := range warns {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Warning,
			Summary:  warn,
			Detail:   detail,
		})
	}
	if err != nil {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  err.Error(),
			Detail:   detail,
		})
	}
	return diags
}

func resourceAppCreate(ctx context.Context, d *schema.ResourceData, m interface{}) (diags diag.Diagnostics) {
	s := m.(*managers.Session)

	desiredApp := resources.Application{
		Name:          d.Get("name").(string),
		SpaceGUID:     d.Get("space_id").(string),
		State:         constant.ApplicationStopped,
		LifecycleType: constant.AppLifecycleType(d.Get("lifecycle_type").(string)),
	}

	app, warns, err := s.ClientV3.CreateApplication(desiredApp)
	diags = append(diags, diagFromClient("create-application", warns, err)...)
	if diags.HasError() {
		return diags
	}

	d.SetId(app.GUID)

	errs := resourceAppUpdate(ctx, d, m)
	diags = append(diags, errs...)
	if diags.HasError() {
		return diags
	}

	return diags
}

func resourceAppRead(ctx context.Context, d *schema.ResourceData, m interface{}) (diags diag.Diagnostics) {
	s := m.(*managers.Session)

	app, errs := getApplication(s, d)
	diags = append(diags, errs...)
	if diags.HasError() {
		return diags
	}

	env, warns, err := s.ClientV3.GetApplicationEnvironment(app.GUID)
	diags = append(diags, diagFromClient("get-application-environment", warns, err)...)
	if diags.HasError() {
		return diags
	}

	web, warns, err := s.ClientV3.GetApplicationProcessByType(app.GUID, "web")
	diags = append(diags, diagFromClient("get-web-processes", warns, err)...)
	if diags.HasError() {
		return diags
	}

	droplet, _, _ := s.ClientV3.GetApplicationDropletCurrent(app.GUID)

	_ = d.Set("name", app.Name)
	_ = d.Set("lifecycle_type", string(app.LifecycleType))
	_ = d.Set("space_id", app.SpaceGUID)
	_ = d.Set("state", string(app.State))
	_ = d.Set("environment", env.EnvironmentVariables)
	_ = d.Set("command", web.Command.Value)
	_ = d.Set("health_check_type", string(web.HealthCheckType))
	_ = d.Set("health_check_endpoint", web.HealthCheckEndpoint)
	_ = d.Set("memory_in_mb", web.MemoryInMB.Value)
	_ = d.Set("disk_in_mb", web.DiskInMB.Value)
	_ = d.Set("instances", web.Instances.Value)

	switch app.LifecycleType {
	case constant.AppLifecycleTypeBuildpack:
		_ = d.Set("stack", app.StackName)
		_ = d.Set("buildpacks", app.LifecycleBuildpacks)
	case constant.AppLifecycleTypeDocker:
		_ = d.Set("docker_image", droplet.Image)
	}

	return diags
}

func resourceAppUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) (diags diag.Diagnostics) {
	s := m.(*managers.Session)

	app, errs := getApplication(s, d)
	diags = append(diags, errs...)
	if diags.HasError() {
		return diags
	}
	app.SpaceGUID = "" // invalid for updates

	errs = applyAppManifest(ctx, s, d)
	diags = append(diags, errs...)
	if diags.HasError() {
		return diags
	}

	currentDroplet, _, _ := s.ClientV3.GetApplicationDropletCurrent(app.GUID)
	desiredDroplet := currentDroplet
	desiredApplicationState := constant.ApplicationState(d.Get("state").(string))

	// update environment vars
	if d.HasChange("environment") {
		errs = applyAppEnvironment(ctx, s, d)
		diags = append(diags, errs...)
		if diags.HasError() {
			return diags
		}
	}

	switch app.LifecycleType {
	case constant.AppLifecycleTypeBuildpack:
		if _, ok := d.GetOk("source_code_path"); ok {
			if d.HasChanges("buildpacks", "source_code_path", "source_code_hash", "stack", "environment") {
				newBuildpackDroplet, errs := createBuildpackDroplet(ctx, s, d)
				diags = append(diags, errs...)
				if diags.HasError() {
					return diags
				}
				desiredDroplet = *newBuildpackDroplet
			}
		}
	case constant.AppLifecycleTypeDocker:
		if _, ok := d.GetOk("docker_image"); ok {
			if d.HasChanges("docker_image", "docker_username", "docker_password", "environment") {
				newDockerDroplet, errs := createDockerDroplet(ctx, s, d)
				diags = append(diags, errs...)
				if diags.HasError() {
					return diags
				}
				desiredDroplet = *newDockerDroplet
			}
		}
	default:
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  fmt.Sprintf("lifecycle type %s is not support", app.LifecycleType),
		})
		return diags
	}

	if desiredDroplet.GUID != "" && desiredDroplet.GUID != currentDroplet.GUID {
		if desiredApplicationState == constant.ApplicationStarted {
			// deployments often fail on the first attempt
			// I have no idea why, so we try a few times
			maxDeployAttempts := 5
			currentDeployAttempt := 0
			for {
				currentDeployAttempt += 1

				log.Printf("[%s] rolling deployment...\n", app.Name)

				deploymentGUID, warns, err := s.ClientV3.CreateApplicationDeployment(d.Id(), desiredDroplet.GUID)
				errs = append(diags, diagFromClient("create-deployment droplet:"+desiredDroplet.GUID, warns, err)...)
				if errs.HasError() {
					if currentDeployAttempt < maxDeployAttempts {
						continue
					}
					return errs
				}
				deploymentState := &resource.StateChangeConf{
					Pending:        deploymentPendingStates,
					Target:         deploymentSuccessStates,
					Refresh:        deploymentStateFunc(s, deploymentGUID),
					Timeout:        d.Timeout(schema.TimeoutUpdate),
					PollInterval:   5 * time.Second,
					Delay:          5 * time.Second,
					NotFoundChecks: 2,
				}
				_, err = deploymentState.WaitForStateContext(ctx)
				if err != nil {
					if currentDeployAttempt < maxDeployAttempts {
						continue
					}
					diags = append(diags, diag.FromErr(err)...)
					return diags
				}

				log.Printf("[%s] rolling deployment... OK!\n", app.Name)

				processes, warns, err := s.ClientV3.GetNewApplicationProcesses(d.Id(), deploymentGUID)
				errs = append(diags, diagFromClient("get-new-application-processes", warns, err)...)
				if errs.HasError() {
					if currentDeployAttempt < maxDeployAttempts {
						continue
					}
					return errs
				}

				for _, process := range processes {
					log.Printf("[%s] waiting for %s process to stablise... \n", app.Name, process.Type)

					jobState := &resource.StateChangeConf{
						Pending:        processInstancePendingStates,
						Target:         processInstanceSuccessStates,
						Refresh:        processInstanceStateFunc(s, process),
						Timeout:        d.Timeout(schema.TimeoutUpdate),
						PollInterval:   2 * time.Second,
						Delay:          2 * time.Second,
						NotFoundChecks: 2,
					}
					if _, err = jobState.WaitForStateContext(ctx); err != nil {
						if currentDeployAttempt < maxDeployAttempts {
							continue
						}
						return diag.FromErr(err)
					}

					log.Printf("[%s] waiting for %s process to stablise... OK!\n", app.Name, process.Type)
				}

				break
			}

		} else {
			_, warns, err := s.ClientV3.SetApplicationDroplet(d.Id(), desiredDroplet.GUID)
			diags = append(diags, diagFromClient("set-current-droplet", warns, err)...)
			if diags.HasError() {
				return diags
			}
		}
	}

	// start or stop the application

	app, errs = getApplication(s, d)
	diags = append(diags, errs...)
	if diags.HasError() {
		return diags
	}
	if desiredApplicationState != app.State {
		switch desiredApplicationState {
		case constant.ApplicationStopped:
			log.Printf("[%s] stopping application... OK!\n", app.Name)
			_, warns, err := s.ClientV3.UpdateApplicationStop(app.GUID)
			diags = append(diags, diagFromClient("update-application", warns, err)...)
			if diags.HasError() {
				return diags
			}
		case constant.ApplicationStarted:
			if desiredDroplet.GUID != "" {
				log.Printf("[%s] starting application... OK!\n", app.Name)
				_, warns, err := s.ClientV3.UpdateApplicationStart(app.GUID)
				diags = append(diags, diagFromClient("update-application", warns, err)...)
				if diags.HasError() {
					return diags
				}
				processes, warns, err := s.ClientV3.GetApplicationProcesses(d.Id())
				diags = append(diags, diagFromClient("get-application-processes", warns, err)...)
				if diags.HasError() {
					return diags
				}

				for _, process := range processes {
					jobState := &resource.StateChangeConf{
						Pending:        processInstancePendingStates,
						Target:         processInstanceSuccessStates,
						Refresh:        processInstanceStateFunc(s, process),
						Timeout:        d.Timeout(schema.TimeoutUpdate),
						PollInterval:   5 * time.Second,
						Delay:          5 * time.Second,
						NotFoundChecks: 2,
					}
					if _, err = jobState.WaitForStateContext(ctx); err != nil {
						return diag.FromErr(err)
					}
				}
			}
		}
	}

	errs = resourceAppRead(ctx, d, m)
	diags = append(diags, errs...)
	if diags.HasError() {
		return diags
	}

	return diags
}

func resourceAppDelete(ctx context.Context, d *schema.ResourceData, m interface{}) (diags diag.Diagnostics) {
	s := m.(*managers.Session)

	deleteJobURL, warns, err := s.ClientV3.DeleteApplication(d.Id())
	diags = append(diags, diagFromClient("delete-application", warns, err)...)
	if diags.HasError() {
		return diags
	}

	jobState := &resource.StateChangeConf{
		Pending:        jobPendingStates,
		Target:         jobSuccessStates,
		Refresh:        jobStateFunc(s, deleteJobURL),
		Timeout:        d.Timeout(schema.TimeoutDelete),
		PollInterval:   5 * time.Second,
		Delay:          5 * time.Second,
		NotFoundChecks: 2,
	}
	if _, err = jobState.WaitForStateContext(ctx); err != nil {
		return diag.FromErr(err)
	}

	return diags
}

func getApplication(s *managers.Session, d *schema.ResourceData) (app *resources.Application, diags diag.Diagnostics) {
	apps, warns, err := s.ClientV3.GetApplications(
		ccv3.Query{Key: ccv3.GUIDFilter, Values: []string{d.Id()}},
	)
	diags = append(diags, diagFromClient("get-applications", warns, err)...)
	if diags.HasError() {
		return nil, diags
	}

	if len(apps) == 0 {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Warning,
			Summary:  fmt.Sprintf("app (%s) not found, removing from state", d.Id()),
			Detail:   "get-applications",
		})
		d.SetId("")
		return nil, diags
	}

	return &apps[0], diags
}

func buildAppManifest(s *managers.Session, d *schema.ResourceData) (appManifest *manifest.Application, diags diag.Diagnostics) {
	lifecycleType := constant.AppLifecycleType(d.Get("lifecycle_type").(string))

	manifest := &manifest.Application{
		Name:        d.Get("name").(string),
		Routes:      []string{},
		Services:    []string{},
		RandomRoute: false,
	}

	if v, ok := d.GetOk("instances"); ok {
		n := v.(int)
		manifest.Instances.IsSet = true
		manifest.Instances.Value = n
	}

	if v, ok := d.GetOk("memory_in_mb"); ok {
		n := v.(int)
		manifest.Memory.IsSet = true
		manifest.Memory.Value = uint64(n)
	}

	if v, ok := d.GetOk("disk_in_mb"); ok {
		n := v.(int)
		manifest.DiskQuota.Value = uint64(n)
		manifest.DiskQuota.IsSet = true
	}

	if v, ok := d.GetOk("health_check_type"); ok {
		s := v.(string)
		manifest.HealthCheckType = s
	}

	if v, ok := d.GetOk("health_check_timeout"); ok {
		n := v.(int)
		manifest.HealthCheckTimeout = uint64(n)
	}

	if manifest.HealthCheckType == string(constant.HTTP) {
		if v, ok := d.GetOk("health_check_endpoint"); ok {
			s := v.(string)
			if s == "" {
				s = "/"
			}
			manifest.HealthCheckHTTPEndpoint = s
		}
	}

	if v, ok := d.GetOk("command"); ok {
		s := v.(string)
		if s != "" {
			manifest.Command.IsSet = true
			manifest.Command.Value = s
		}
	}

	if lifecycleType == constant.AppLifecycleTypeBuildpack {
		buildpacks := []string{}
		if vs, ok := d.GetOk("buildpacks"); ok {
			for _, v := range vs.([]interface{}) {
				buildpacks = append(buildpacks, v.(string))
			}
		}
		manifest.Buildpacks = buildpacks
		manifest.StackName = d.Get("stack").(string)
	} else if lifecycleType == constant.AppLifecycleTypeDocker {
		manifest.DockerImage = d.Get("docker_image").(string)
	}

	return manifest, diags
}

func createDockerDroplet(ctx context.Context, s *managers.Session, d *schema.ResourceData) (_ *resources.Droplet, diags diag.Diagnostics) {
	appGUID := d.Id()

	pkg, warns, err := s.ClientV3.CreatePackage(resources.Package{
		Type:        constant.PackageTypeDocker,
		DockerImage: d.Get("docker_image").(string),
		Relationships: resources.Relationships{
			constant.RelationshipTypeApplication: resources.Relationship{
				GUID: appGUID,
			},
		},
	})
	diags = append(diags, diagFromClient("create-docker-package", warns, err)...)
	if diags.HasError() {
		return nil, diags
	}

	build, warns, err := s.ClientV3.CreateBuild(resources.Build{
		PackageGUID: pkg.GUID,
	})
	diags = append(diags, diagFromClient("create-docker-build", warns, err)...)
	if diags.HasError() {
		return nil, diags
	}

	buildState := &resource.StateChangeConf{
		Pending:        buildPendingStates,
		Target:         buildSuccessStates,
		Refresh:        buildStateFunc(s, build.GUID),
		Timeout:        d.Timeout(schema.TimeoutUpdate),
		PollInterval:   5 * time.Second,
		Delay:          5 * time.Second,
		NotFoundChecks: 2,
	}
	if _, err = buildState.WaitForStateContext(ctx); err != nil {
		diags = append(diags, diag.FromErr(err)...)
		return nil, diags
	}

	build, warns, err = s.ClientV3.GetBuild(build.GUID)
	diags = append(diags, diagFromClient("get-docker-build", warns, err)...)
	if diags.HasError() {
		return nil, diags
	}

	droplet, warns, err := s.ClientV3.GetDroplet(build.DropletGUID)
	diags = append(diags, diagFromClient("get-built-docker-droplet", warns, err)...)
	if diags.HasError() {
		return nil, diags
	}

	return &droplet, diags
}

func createBuildpackDroplet(ctx context.Context, s *managers.Session, d *schema.ResourceData) (_ *resources.Droplet, diags diag.Diagnostics) {
	appGUID := d.Id()

	// create bits package

	pkg, warns, err := s.ClientV3.CreatePackage(resources.Package{
		Type: constant.PackageTypeBits,
		Relationships: resources.Relationships{
			constant.RelationshipTypeApplication: resources.Relationship{
				GUID: appGUID,
			},
		},
	})
	diags = append(diags, diagFromClient("create-bits-package", warns, err)...)
	if diags.HasError() {
		return nil, diags
	}

	// upload bits

	filename := d.Get("source_code_path").(string)
	if filename == "" {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  "source_code_path required for lifecycle type buildpack",
			Detail:   "set the source_code_path to a path to a zipped up version of your application source code",
		})
		return nil, diags
	}

	archive, err := os.Open(filename)
	if err != nil {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  "failed to read zip file for source_code_path: " + filename,
			Detail:   err.Error(),
		})
		return nil, diags
	}
	defer archive.Close()
	archiveInfo, err := archive.Stat()
	if err != nil {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  "failed to stat zip file for source_code_path: " + filename,
			Detail:   err.Error(),
		})
		return nil, diags
	}
	existingResources := []ccv3.Resource{} // TODO: grab from state
	pkg, warns, err = s.ClientV3.UploadBitsPackage(pkg, existingResources, archive, archiveInfo.Size())
	diags = append(diags, diagFromClient("upload-bits", warns, err)...)
	if diags.HasError() {
		return nil, diags
	}

	pkgState := &resource.StateChangeConf{
		Pending:        packagePendingStates,
		Target:         packageSuccessStates,
		Refresh:        packageStateFunc(s, pkg.GUID),
		Timeout:        d.Timeout(schema.TimeoutUpdate),
		PollInterval:   5 * time.Second,
		Delay:          5 * time.Second,
		NotFoundChecks: 2,
	}
	_, err = pkgState.WaitForStateContext(ctx)
	if err != nil {
		diags = append(diags, diag.FromErr(err)...)
		return nil, diags
	}

	// create a build (stage)

	build, warns, err := s.ClientV3.CreateBuild(resources.Build{
		PackageGUID: pkg.GUID,
	})
	diags = append(diags, diagFromClient("create-build", warns, err)...)
	if diags.HasError() {
		return nil, diags
	}

	buildState := &resource.StateChangeConf{
		Pending:        buildPendingStates,
		Target:         buildSuccessStates,
		Refresh:        buildStateFunc(s, build.GUID),
		Timeout:        d.Timeout(schema.TimeoutUpdate),
		PollInterval:   5 * time.Second,
		Delay:          5 * time.Second,
		NotFoundChecks: 2,
	}
	if _, err = buildState.WaitForStateContext(ctx); err != nil {
		diags = append(diags, diag.FromErr(err)...)
		return nil, diags
	}

	build, warns, err = s.ClientV3.GetBuild(build.GUID)
	diags = append(diags, diagFromClient("get-build", warns, err)...)
	if diags.HasError() {
		return nil, diags
	}

	droplet, warns, err := s.ClientV3.GetDroplet(build.DropletGUID)
	diags = append(diags, diagFromClient("get-built-droplet", warns, err)...)
	if diags.HasError() {
		return nil, diags
	}

	return &droplet, diags
}

func applyAppEnvironment(ctx context.Context, s *managers.Session, d *schema.ResourceData) (diags diag.Diagnostics) {
	appGUID := d.Id()

	currentEnv, warns, err := s.ClientV3.GetApplicationEnvironment(appGUID)
	diags = append(diags, diagFromClient("get-current-environment", warns, err)...)
	if diags.HasError() {
		return diags
	}
	desiredEnv := resources.EnvironmentVariables{}
	for k := range currentEnv.EnvironmentVariables {
		desiredEnv[k] = types.FilteredString{
			Value: "",
			IsSet: false,
		}
	}
	for k, v := range d.Get("environment").(map[string]interface{}) {
		desiredEnv[k] = types.FilteredString{
			Value: v.(string),
			IsSet: true,
		}
	}
	_, warns, err = s.ClientV3.UpdateApplicationEnvironmentVariables(
		d.Id(),
		desiredEnv,
	)
	diags = append(diags, diagFromClient("update-environment", warns, err)...)
	if diags.HasError() {
		return diags
	}

	return diags
}

func applyAppManifest(ctx context.Context, s *managers.Session, d *schema.ResourceData) (diags diag.Diagnostics) {
	spaceGUID := d.Get("space_id").(string)

	appManifest, errs := buildAppManifest(s, d)
	diags = append(diags, errs...)
	if diags.HasError() {
		return diags
	}

	manifest := struct {
		Applications []manifest.Application
	}{
		Applications: []manifest.Application{
			*appManifest,
		},
	}
	rawManifest, err := yaml.Marshal(manifest)
	if err != nil {
		return diag.FromErr(err)
	}

	applyJobURL, warns, err := s.ClientV3.UpdateSpaceApplyManifest(spaceGUID, rawManifest)
	diags = append(diags, diagFromClient("apply-manifest", warns, err)...)
	if diags.HasError() {
		return diags
	}

	jobState := &resource.StateChangeConf{
		Pending:        jobPendingStates,
		Target:         jobSuccessStates,
		Refresh:        jobStateFunc(s, applyJobURL),
		Timeout:        d.Timeout(schema.TimeoutUpdate),
		PollInterval:   15 * time.Second,
		Delay:          5 * time.Second,
		NotFoundChecks: 2,
	}
	if _, err = jobState.WaitForStateContext(ctx); err != nil {
		return diag.FromErr(err)
	}

	return diags
}

func buildStateFunc(s *managers.Session, buildGUID string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		build, _, err := s.ClientV3.GetBuild(buildGUID)
		return build, string(build.State), err
	}
}

var buildPendingStates = []string{
	string(constant.BuildStaging),
}

var buildSuccessStates = []string{
	string(constant.BuildStaged),
}

func packageStateFunc(s *managers.Session, packageGUID string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		pkg, _, err := s.ClientV3.GetPackage(packageGUID)
		return pkg, string(pkg.State), err
	}
}

var packagePendingStates = []string{
	string(constant.PackageAwaitingUpload),
	string(constant.PackageCopying),
	string(constant.PackageProcessingUpload),
}

var packageSuccessStates = []string{
	string(constant.PackageReady),
}

func deploymentStateFunc(s *managers.Session, deploymentGUID string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		deployment, _, err := s.ClientV3.GetDeployment(deploymentGUID)
		if err != nil {
			return nil, "", err
		}

		switch deployment.StatusValue {
		case constant.DeploymentStatusValueFinalized:
			switch deployment.StatusReason {
			case constant.DeploymentStatusReasonDeployed:
				return deployment, string(constant.DeploymentDeployed), nil
			default:
				return nil, string(constant.DeploymentFailed), fmt.Errorf("deployment failed: %s", deployment.StatusReason)
			}
		default:
			return deployment, string(constant.DeploymentDeploying), nil
		}
	}
}

var deploymentPendingStates = []string{
	string(constant.DeploymentDeploying),
}

var deploymentSuccessStates = []string{
	string(constant.DeploymentDeployed),
}

func processInstanceStateFunc(s *managers.Session, process resources.Process) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		instances, _, err := s.ClientV3.GetProcessInstances(process.GUID)
		if err != nil {
			return nil, "", err
		}
		runningInstances := []ccv3.ProcessInstance{}
		crashedInstances := []ccv3.ProcessInstance{}

		for _, instance := range instances {
			switch instance.State {
			case constant.ProcessInstanceRunning:
				runningInstances = append(runningInstances, instance)
			case constant.ProcessInstanceCrashed:
				crashedInstances = append(crashedInstances, instance)
			}
		}

		if len(instances) == 0 || len(runningInstances) == process.Instances.Value {
			return instances, ProcessInstancesStable, nil
		}

		if len(crashedInstances) == len(instances) {
			return instances, ProcessInstancesCrashed, fmt.Errorf("all %d process instances are in a crashed state", len(instances))
		}

		return instances, ProcessInstancesPending, nil
	}
}

const (
	ProcessInstancesPending = "PENDING"
	ProcessInstancesStable  = "STABLE"
	ProcessInstancesCrashed = "CRASHED"
)

var processInstancePendingStates = []string{
	string(ProcessInstancesPending),
}

var processInstanceSuccessStates = []string{
	string(ProcessInstancesStable),
}
