/*
 * Copyright 2024 The RuleGo Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package endpoint

import (
	"context"
	"github.com/rulego/rulego/api/types"
)

// RouterOption is a function type for setting options on routing components.
type RouterOption func(OptionsSetter) error

// RouterOptions holds the default router options.
var RouterOptions = routerOptions{}

type routerOptions struct {
}

// WithRuleGoFunc sets a function to dynamically retrieve the rule engine pool.
func (r routerOptions) WithRuleGoFunc(f func(exchange *Exchange) types.RuleEnginePool) RouterOption {
	return func(re OptionsSetter) error {
		re.SetRuleEnginePoolFunc(f)
		return nil
	}
}

// WithRuleGo sets the rule engine pool, defaulting to `rulego.DefaultPool`.
func (r routerOptions) WithRuleGo(ruleGo types.RuleEnginePool) RouterOption {
	return func(re OptionsSetter) error {
		re.SetRuleEnginePool(ruleGo)
		return nil
	}
}

// WithRuleConfig sets the rule engine configuration.
func (r routerOptions) WithRuleConfig(config types.Config) RouterOption {
	return func(re OptionsSetter) error {
		re.SetConfig(config)
		return nil
	}
}

// WithContextFunc sets the context function for the routing operation.
func (r routerOptions) WithContextFunc(f func(ctx context.Context, exchange *Exchange) context.Context) RouterOption {
	return func(re OptionsSetter) error {
		re.SetContextFunc(f)
		return nil
	}
}

func (r routerOptions) WithDefinition(def *types.RouterDsl) RouterOption {
	return func(re OptionsSetter) error {
		re.SetDefinition(def)
		return nil
	}
}

// DynamicEndpointOption is a function type for setting options on dynamic endpoints.
type DynamicEndpointOption func(DynamicEndpoint) error

// DynamicEndpointOptions holds the default dynamic endpoint options.
var DynamicEndpointOptions = dynamicEndpointOptions{}

type dynamicEndpointOptions struct {
}

// WithConfig sets the configuration for the dynamic endpoint.
func (d dynamicEndpointOptions) WithConfig(config types.Config) DynamicEndpointOption {
	return func(re DynamicEndpoint) error {
		re.SetConfig(config)
		return nil
	}
}

// WithRouterOpts sets the routerOptions for the dynamic endpoint.
func (d dynamicEndpointOptions) WithRouterOpts(opts ...RouterOption) DynamicEndpointOption {
	return func(re DynamicEndpoint) error {
		re.SetRouterOptions(opts...)
		return nil
	}
}

// WithOnEvent sets the event handler for the dynamic endpoint.
func (d dynamicEndpointOptions) WithOnEvent(onEvent OnEvent) DynamicEndpointOption {
	return func(re DynamicEndpoint) error {
		re.SetOnEvent(onEvent)
		return nil
	}
}

// WithRestart sets restart sign for the dynamic endpoint.
func (d dynamicEndpointOptions) WithRestart(restart bool) DynamicEndpointOption {
	return func(re DynamicEndpoint) error {
		re.SetRestart(restart)
		return nil
	}
}

// WithInterceptors sets the interceptors for the dynamic endpoint.
func (d dynamicEndpointOptions) WithInterceptors(interceptors ...Process) DynamicEndpointOption {
	return func(re DynamicEndpoint) error {
		re.SetInterceptors(interceptors...)
		return nil
	}
}
