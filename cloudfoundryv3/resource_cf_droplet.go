package cloudfoundry

import (
	"context"
	"fmt"
	"os"
	"time"

	"code.cloudfoundry.org/cli/api/cloudcontroller/ccv3"
	"code.cloudfoundry.org/cli/api/cloudcontroller/ccv3/constant"
	"code.cloudfoundry.org/cli/resources"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/terraform-providers/terraform-provider-cloudfoundry/cloudfoundryv3/managers"
)

func resourceDroplet() *schema.Resource {

	return &schema.Resource{
		Description: "deployments associate the desired active droplet to use for a running application and handle managing the changes without interuption",

		CreateContext: resourceDropletCreate,
		ReadContext:   resourceDropletRead,
		DeleteContext: resourceDropletDelete,

		Schema: map[string]*schema.Schema{

			"app_id": {
				Description:  "application GUID to build package/droplet for",
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.NoZeroValues,
				ForceNew:     true,
			},

			"type": {
				Description:  "The lifecycle type of the source. There are two types (lifecycles) of cloudfoundry application builds, 'buildpack' and 'docker'. For buildpack source types, you must supply `source_code_path` to a zip of application source code. For the 'docker' source type, you must supply the `docker_image`.",
				Type:         schema.TypeString,
				Optional:     true,
				Default:      "buildpack",
				ValidateFunc: validation.StringInSlice([]string{"buildpack", "docker", "kpack"}, false),
				ForceNew:     true,
			},

			"source_code_path": {
				Description:   "Path to a zip of the application source code. Required if type is 'buildpack'",
				Type:          schema.TypeString,
				Optional:      true,
				ValidateFunc:  validation.StringIsNotEmpty,
				ConflictsWith: []string{"docker_image"},
				ForceNew:      true,
			},

			"source_code_hash": {
				Description:   "Set this to a sum of the source_code data to trigger deployments on changes",
				Type:          schema.TypeString,
				Optional:      true,
				ValidateFunc:  validation.StringIsNotEmpty,
				ConflictsWith: []string{"docker_image"},
				ForceNew:      true,
			},

			"buildpacks": {
				Description: "A list of the names of buildpacks, URLs from which they may be downloaded",
				Type:        schema.TypeList,
				Optional:    true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				ConflictsWith: []string{"docker_image"},
				ForceNew:      true,
			},

			"stack": {
				Description:   "The root filesystem to use with the buildpack, for example cflinuxfs3",
				Type:          schema.TypeString,
				Optional:      true,
				Default:       "cflinuxfs3",
				ValidateFunc:  validation.StringIsNotEmpty,
				ConflictsWith: []string{"docker_image"},
				ForceNew:      true,
			},

			"docker_image": {
				Description:   "The docker image to use. Required if lifecycle type is 'docker'",
				Type:          schema.TypeString,
				Optional:      true,
				ValidateFunc:  validation.StringIsNotEmpty,
				ConflictsWith: []string{"stack", "buildpacks", "source_code_path"},
				ForceNew:      true,
			},

			// environment cannot currently override the app's environment
			// this field is mainly to ensure new droplets are re-staged when env changes
			"environment": {
				Description: "The environment variables associated with the given droplet. Environment variable names may not start with VCAP_. PORT is not a valid environment variable.",
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
				ForceNew: true,
			},

			// command cannot currently override the app's environment
			// this field is mainly to ensure new droplets are re-staged when command changes
			"command": {
				Description: "The command used to start the process, this should be passed from the app resource to trigger rebuild on change",
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
				ForceNew:    true,
			},
		},
	}
}

func resourceDropletCreate(ctx context.Context, d *schema.ResourceData, m interface{}) (diags diag.Diagnostics) {
	s := m.(*managers.Session)
	lifecycleType := constant.AppLifecycleType(d.Get("type").(string))
	appGUID := d.Get("app_id").(string)
	buildpacks := d.Get("buildpacks").([]interface{})
	waitTimeout := d.Timeout(schema.TimeoutCreate)

	// FIXME! we should not have to update the app's default lifecycle type
	// the API docs say that we can override the buildpacks field and lifecycle
	// type. however the ccv3 client does not yet support this so this is a bit
	// of a hack to allow us to configure the droplet without splitting half the
	// config between the "app" and "droplet" resources. This should be refactored
	// as soon as the ccv3 client supports the full API
	cfg := resources.Application{
		GUID:          appGUID,
		LifecycleType: lifecycleType,
	}
	if lifecycleType == constant.AppLifecycleTypeBuildpack {
		for _, v := range buildpacks {
			cfg.LifecycleBuildpacks = append(cfg.LifecycleBuildpacks, v.(string))
		}
	}
	_, warns, err := s.ClientV3.UpdateApplication(cfg)
	diags = append(diags, diagFromClient("update-app-lifeycle-type", warns, err)...)
	if diags.HasError() {
		return diags
	}

	switch lifecycleType {
	case constant.AppLifecycleTypeBuildpack:
		sourceCodePath := d.Get("source_code_path").(string)
		newBuildpackDroplet, errs := createBuildpackDroplet(ctx, s, appGUID, sourceCodePath, waitTimeout)
		diags = append(diags, errs...)
		if diags.HasError() {
			return diags
		}
		d.SetId(newBuildpackDroplet.GUID)
	case constant.AppLifecycleTypeDocker:
		// set lifecycle type on app
		dockerImage := d.Get("docker_image").(string)
		newDockerDroplet, errs := createDockerDroplet(ctx, s, appGUID, dockerImage, waitTimeout)
		diags = append(diags, errs...)
		if diags.HasError() {
			return diags
		}
		d.SetId(newDockerDroplet.GUID)
	default:
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  fmt.Sprintf("lifecycle type %s is not support", lifecycleType),
		})
		return diags
	}

	return resourceDropletRead(ctx, d, m)
}

func resourceDropletRead(ctx context.Context, d *schema.ResourceData, m interface{}) (diags diag.Diagnostics) {
	s := m.(*managers.Session)
	dropletGUID := d.Id()
	lifecycleType := constant.AppLifecycleType(d.Get("type").(string))

	droplets, warns, err := s.ClientV3.GetDroplets(
		ccv3.Query{Key: ccv3.GUIDFilter, Values: []string{dropletGUID}},
	)
	diags = append(diags, diagFromClient("get-droplet-for-read", warns, err)...)
	if diags.HasError() {
		return diags
	}
	if len(droplets) == 0 {
		fmt.Println("droplet not found!")
		d.SetId("")
		return diags
	}
	droplet := droplets[0]

	switch lifecycleType {
	case constant.AppLifecycleTypeBuildpack:
		buildpacks := []string{}
		for _, bp := range droplet.Buildpacks {
			buildpacks = append(buildpacks, bp.Name)
		}
		_ = d.Set("buildpacks", buildpacks)
		_ = d.Set("stack", droplet.Stack)
	case constant.AppLifecycleTypeDocker:
		_ = d.Set("docker_image", droplet.Image)
	}

	return diags
}

func resourceDropletDelete(ctx context.Context, d *schema.ResourceData, m interface{}) (diags diag.Diagnostics) {
	// s := m.(*managers.Session)

	// You don't really delete droplets as they may be in use elsewhere, so this is a no-op
	// TODO: should we delete droplet if we _know_ are not in use?

	return diags
}

func createBuildpackDroplet(ctx context.Context, s *managers.Session, appGUID, sourceCodePath string, waitTimeout time.Duration) (_ *resources.Droplet, diags diag.Diagnostics) {

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

	if sourceCodePath == "" {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  "source_code_path required for lifecycle type buildpack",
			Detail:   "set the source_code_path to a path to a zipped up version of your application source code",
		})
		return nil, diags
	}

	archive, err := os.Open(sourceCodePath)
	if err != nil {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  "failed to read zip file for source_code_path: " + sourceCodePath,
			Detail:   err.Error(),
		})
		return nil, diags
	}
	defer archive.Close()
	archiveInfo, err := archive.Stat()
	if err != nil {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  "failed to stat zip file for source_code_path: " + sourceCodePath,
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
		Timeout:        waitTimeout,
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
		Timeout:        waitTimeout,
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

func createDockerDroplet(ctx context.Context, s *managers.Session, appGUID, dockerImage string, waitTimeout time.Duration) (_ *resources.Droplet, diags diag.Diagnostics) {

	pkg, warns, err := s.ClientV3.CreatePackage(resources.Package{
		Type:        constant.PackageTypeDocker,
		DockerImage: dockerImage,
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
		Timeout:        waitTimeout,
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
