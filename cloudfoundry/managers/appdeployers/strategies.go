package appdeployers

type DeployStrategyType string

const (
	V3DeployStrategy DeployStrategyType = "v3"
	V2DeployStrategy DeployStrategyType = "v2"
)

type Strategy interface {
	Deploy(appDeploy AppDeploy) (AppDeployResponse, error)
	Restage(appDeploy AppDeploy) (AppDeployResponse, error)
	IsCreateNewApp() bool
	Names() []string
	StrategyType() DeployStrategyType
}
