package cloudfoundry

import (
	"context"
	"fmt"
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

			"type": {
				Description:  "The lifecycle type of the application. There are two types (lifecycles) of cloudfoundry application builds, 'buildpack' and 'docker'. For buildpack source types, you must supply `source_code_path` to a zip of application source code. For the 'docker' source type, you must supply the `docker_image`.",
				Type:         schema.TypeString,
				Optional:     true,
				Default:      "buildpack",
				ValidateFunc: validation.StringInSlice([]string{"buildpack", "docker", "kpack"}, false),
				ForceNew:     true,
			},

			"name": {
				Description: "Name of the application. Names must be unique within a Space",
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
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

			"environment": {
				Description: "The environment variables associated with the given app. Environment variable names may not start with VCAP_. PORT is not a valid environment variable.",
				Type:        schema.TypeMap,
				Optional:    true,
				Sensitive:   true,
				ValidateFunc: validation.All(
					validateEnvMapKeysPattern,
					validateEnvMapEmptyStrings,
				),
				Elem: &schema.Schema{
					Type: schema.TypeString,
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
		LifecycleType: constant.AppLifecycleType(d.Get("type").(string)),
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

	app, exists, errs := getApplication(s, d.Id())
	diags = append(diags, errs...)
	if diags.HasError() {
		return diags
	}
	if !exists {
		d.SetId("")
		return diags
	}

	env, warns, err := s.ClientV3.GetApplicationEnvironment(app.GUID)
	diags = append(diags, diagFromClient("get-application-environment", warns, err)...)
	if diags.HasError() {
		return diags
	}
	if _, ok := d.GetOk("environment"); ok {
		_ = d.Set("environment", env.EnvironmentVariables)
	}

	web, warns, err := s.ClientV3.GetApplicationProcessByType(app.GUID, "web")
	diags = append(diags, diagFromClient("get-web-process", warns, err)...)
	if diags.HasError() {
		return diags
	}
	if _, ok := d.GetOk("command"); ok {
		_ = d.Set("command", web.Command.Value)
	}

	_ = d.Set("name", app.Name)
	_ = d.Set("space_id", app.SpaceGUID)
	_ = d.Set("health_check_type", string(web.HealthCheckType))
	_ = d.Set("health_check_endpoint", web.HealthCheckEndpoint)
	_ = d.Set("memory_in_mb", web.MemoryInMB.Value)
	_ = d.Set("disk_in_mb", web.DiskInMB.Value)
	_ = d.Set("instances", web.Instances.Value)

	return diags
}

func resourceAppUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) (diags diag.Diagnostics) {
	s := m.(*managers.Session)

	app, exists, errs := getApplication(s, d.Id())
	diags = append(diags, errs...)
	if diags.HasError() {
		return diags
	}
	if !exists {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Warning,
			Summary:  fmt.Sprintf("app (%s) not found, removing from state", d.Id()),
			Detail:   "get-application-for-update",
		})
		d.SetId("")
		return diags
	}
	app.SpaceGUID = "" // invalid for updates

	errs = applyAppManifest(ctx, s, d)
	diags = append(diags, errs...)
	if diags.HasError() {
		return diags
	}

	// update environment vars
	if d.HasChange("environment") {
		errs = applyAppEnvironment(ctx, s, d)
		diags = append(diags, errs...)
		if diags.HasError() {
			return diags
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

func getApplication(s *managers.Session, appGUID string) (app *resources.Application, found bool, diags diag.Diagnostics) {
	apps, warns, err := s.ClientV3.GetApplications(
		ccv3.Query{Key: ccv3.GUIDFilter, Values: []string{appGUID}},
	)
	diags = append(diags, diagFromClient("get-applications", warns, err)...)
	if diags.HasError() {
		return nil, false, diags
	}

	if len(apps) == 0 {
		return nil, false, diags
	}

	return &apps[0], true, diags
}

func buildAppManifest(s *managers.Session, d *schema.ResourceData) (appManifest *manifest.Application, diags diag.Diagnostics) {

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

	return manifest, diags
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
