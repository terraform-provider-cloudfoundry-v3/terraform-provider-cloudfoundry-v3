package cloudfoundry

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"code.cloudfoundry.org/cli/api/cloudcontroller/ccerror"
	"code.cloudfoundry.org/cli/api/cloudcontroller/ccv2/constant"
	"code.cloudfoundry.org/cli/api/uaa"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/terraform-providers/terraform-provider-cloudfoundry/cloudfoundryv3/managers"
)

func resourceServiceBinding() *schema.Resource {

	return &schema.Resource{

		CreateContext: resourceServiceBindingCreate,
		ReadContext:   resourceServiceBindingRead,
		DeleteContext: resourceServiceBindingDelete,

		Schema: map[string]*schema.Schema{

			"app_id": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.NoZeroValues,
				ForceNew:     true,
			},

			"service_instance_id": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.NoZeroValues,
				ForceNew:     true,
			},

			"params": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
		},
	}
}

func resourceServiceBindingCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	session := meta.(*managers.Session)
	appGUID := d.Get("app_id").(string)
	serviceInstanceGUID := d.Get("service_instance_id").(string)
	paramsJSON := d.Get("params").(string)
	if paramsJSON == "" {
		paramsJSON = "{}"
	}

	var params map[string]interface{}
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		return diag.FromErr(err)
	}

	// FIXME: ccv3 credential bindings are experimental and the endpoint not
	// available on GOV.UK PaaS yet so using the v2 method for now
	binding, _, err := session.ClientV2.CreateServiceBinding(appGUID, serviceInstanceGUID, "", true, params)
	diags = append(diags, diagFromClient("create-service-credential-binding", nil, err)...)
	if diags.HasError() {
		return diags
	}

	stateConf := &resource.StateChangeConf{
		Pending:        serviceBindingPendingStates,
		Target:         serviceBindingSuccessStates,
		Refresh:        serviceBindingStateFunc(session, binding.GUID),
		Timeout:        d.Timeout(schema.TimeoutCreate),
		PollInterval:   5 * time.Second,
		Delay:          3 * time.Second,
		NotFoundChecks: 1,
	}
	if _, err = stateConf.WaitForStateContext(ctx); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(binding.GUID)
	return diags
}

func resourceServiceBindingRead(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	session := meta.(*managers.Session)
	bindingGUID := d.Id()

	_, _, err := session.ClientV2.GetServiceBinding(bindingGUID)
	if err != nil {
		if IsErrNotFound(err) {
			d.SetId("")
			return diags
		}
		diags = append(diags, diagFromClient("read-service-binding", nil, err)...)
		if diags.HasError() {
			return diags
		}
	}

	return diags
}

func resourceServiceBindingDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	session := meta.(*managers.Session)

	_, _, err := session.ClientV2.DeleteServiceBinding(d.Id(), true)
	diags = append(diags, diagFromClient("delete-service-binding", nil, err)...)
	if diags.HasError() {
		return diags
	}

	return diags
}

func serviceBindingStateFunc(s *managers.Session, bindingGUID string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		binding, _, err := s.ClientV2.GetServiceBinding(bindingGUID)
		if err != nil {
			return nil, "", err
		}
		if binding.LastOperation.State == constant.LastOperationFailed {
			return binding, string(binding.LastOperation.State), fmt.Errorf(
				"Binding failed, reason: %s",
				binding.LastOperation.Description,
			)
		}
		return binding, string(binding.LastOperation.State), nil
	}
}

var serviceBindingPendingStates = []string{
	string(constant.LastOperationInProgress),
}

var serviceBindingSuccessStates = []string{
	string(constant.LastOperationSucceeded),
}

func IsErrNotFound(err error) bool {
	if httpErr, ok := err.(ccerror.RawHTTPStatusError); ok && httpErr.StatusCode == 404 {
		return true
	}
	if _, ok := err.(ccerror.ResourceNotFoundError); ok {
		return true
	}
	if uaaErr, ok := err.(uaa.RawHTTPStatusError); ok && uaaErr.StatusCode == 404 {
		return true
	}
	return false
}
