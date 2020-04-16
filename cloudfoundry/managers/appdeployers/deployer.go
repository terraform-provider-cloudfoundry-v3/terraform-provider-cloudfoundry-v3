package appdeployers

import (
	"strings"
)

const DefaultV2Strategy = "default"
const DefaultV3Strategy = "standardv3"

type Deployer struct {
	strategies []Strategy
}

func NewDeployer(strategies ...Strategy) *Deployer {
	return &Deployer{
		strategies: strategies,
	}
}

func (d Deployer) Strategy(strategyName string, v3 bool) Strategy {
	strategyName = strings.ToLower(strategyName)
	var defaultStrategy Strategy
	for _, strategy := range d.strategies {
		for _, name := range strategy.Names() {
			if name == strategyName {
				return strategy
			}

			if name == DefaultV3Strategy && v3 {
				return strategy
			}

			if name == DefaultV2Strategy {
				defaultStrategy = strategy
			}
		}
	}
	return defaultStrategy
}

func ValidStrategy(strategyName string) ([]string, bool) {
	strategyName = strings.ToLower(strategyName)
	names := append(Standard{}.Names(), BlueGreenV2{}.Names()...)
	names = append(names, StandardV3{}.Names()...)
	for _, name := range names {
		if name == strategyName {
			return names, true
		}
	}
	return names, false
}
