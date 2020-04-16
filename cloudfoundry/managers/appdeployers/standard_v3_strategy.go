package appdeployers

import (
	"code.cloudfoundry.org/cli/api/cloudcontroller/ccv3"
	"code.cloudfoundry.org/cli/api/cloudcontroller/ccv3/constant"
)

type StandardV3 struct {
	client    *ccv3.Client
	runBinder *RunBinder
}

func NewStandardV3(client *ccv3.Client, runBinder *RunBinder) *StandardV3 {
	return &StandardV3{
		client:    client,
		runBinder: runBinder,
	}
}

func (s StandardV3) Deploy(appDeploy AppDeploy) (AppDeployResponse, error) {
	stateAsk := appDeploy.AppV3.State
	var deployFunc func(app ccv3.Application) (ccv3.Application, ccv3.Warnings, error)
	if appDeploy.App.GUID != "" {
		deployFunc = s.client.UpdateApplication
	} else {
		deployFunc = s.client.CreateApplication
	}

	actions := Actions{
		{
			Forward: func(ctx Context) (Context, error) {
				app := appDeploy.AppV3
				app.State = constant.ApplicationStopped
				app, _, err := deployFunc(app)
				if err != nil {
					return ctx, err
				}
				ctx["app_response"] = AppDeployResponse{
					AppV3: app,
				}
				return ctx, nil
			},
		},
	}
	var appResp AppDeployResponse
	ctx, err := actions.Execute()
	if appRespCtx, ok := ctx["app_response"]; ok {
		appResp = appRespCtx.(AppDeployResponse)
	}
	if stateAsk == constant.ApplicationStopped || err != nil {
		appResp.AppV3.State = constant.ApplicationStopped
	} else {
		appResp.AppV3.State = constant.ApplicationStarted
	}

	return appResp, err
}

func (s StandardV3) Restage(appDeploy AppDeploy) (AppDeployResponse, error) {
	app, _, err := s.client.RestageApplication(appDeploy.AppV3)
	if err != nil {
		return AppDeployResponse{}, err
	}
	appDeploy.AppV3 = app

	appResp := AppDeployResponse{
		AppV3:           app,
		RouteMapping:    appDeploy.Mappings,
		ServiceBindings: appDeploy.ServiceBindings,
	}

	err = s.runBinder.WaitStaging(appDeploy)
	if err != nil {
		return appResp, err
	}
	err = s.runBinder.WaitStart(appDeploy)
	if err != nil {
		return appResp, err
	}
	if appDeploy.AppV3.State == constant.ApplicationStopped {
		err := s.runBinder.Stop(appDeploy)
		return appResp, err
	}

	return appResp, nil
}

func (s StandardV3) IsCreateNewApp() bool {
	return false
}

func (s StandardV3) Names() []string {
	return []string{"standardv3", "v3"}
}

func (StandardV3) StrategyType() DeployStrategyType {
	return V3DeployStrategy
}
