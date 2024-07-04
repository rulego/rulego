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

package types

// RuleEngineOption defines a function type for configuring a RuleEngine.
type RuleEngineOption func(RuleEngine) error

// WithConfig creates a RuleEngineOption to set the configuration of a RuleEngine.
func WithConfig(config Config) RuleEngineOption {
	return func(re RuleEngine) error {
		re.SetConfig(config) // Apply the provided configuration to the RuleEngine.
		return nil           // Return no error.
	}
}

// WithAspects creates a RuleEngineOption to set the aspects of a RuleEngine.
func WithAspects(aspects ...Aspect) RuleEngineOption {
	return func(re RuleEngine) error {
		re.SetAspects(aspects...) // Apply the provided aspects to the RuleEngine.
		return nil                // Return no error.
	}
}

func WithRuleEnginePool(ruleEnginePool RuleEnginePool) RuleEngineOption {
	return func(re RuleEngine) error {
		re.SetRuleEnginePool(ruleEnginePool)
		return nil
	}
}

// RuleEngine is an interface for a rule engine.
type RuleEngine interface {
	// Id returns the identifier of the RuleEngine.
	Id() string
	// SetConfig sets the configuration for the RuleEngine.
	SetConfig(config Config)
	// SetAspects sets the aspects for the RuleEngine.
	SetAspects(aspects ...Aspect)
	// SetRuleEnginePool sets the rule engine pool for the RuleEngine.
	SetRuleEnginePool(ruleEnginePool RuleEnginePool)
	// Reload reloads the RuleEngine with the given options.
	Reload(opts ...RuleEngineOption) error
	// ReloadSelf reloads the RuleEngine itself with the given definition and options.
	ReloadSelf(def []byte, opts ...RuleEngineOption) error
	// ReloadChild reloads a child node within the RuleEngine.
	ReloadChild(ruleNodeId string, dsl []byte) error
	// DSL returns the DSL (Domain Specific Language) representation of the RuleEngine.
	DSL() []byte
	// Definition returns the definition of the rule chain.
	Definition() RuleChain
	// RootRuleChainCtx returns the context of the root rule chain.
	RootRuleChainCtx() ChainCtx
	// NodeDSL returns the DSL of a specific node within the rule chain.
	NodeDSL(chainId RuleNodeId, childNodeId RuleNodeId) []byte
	// Initialized checks if the RuleEngine is initialized.
	Initialized() bool
	// Stop stops the RuleEngine.
	Stop()
	// OnMsg processes a message with the given context options.
	OnMsg(msg RuleMsg, opts ...RuleContextOption)
	// OnMsgAndWait processes a message and waits for completion with the given context options.
	OnMsgAndWait(msg RuleMsg, opts ...RuleContextOption)
}

// RuleEnginePool is an interface for a pool of rule engines.
type RuleEnginePool interface {
	// Load loads all rule chain configurations from a specified folder and its subfolders into the rule engine instance pool.
	Load(folderPath string, opts ...RuleEngineOption) error
	// New creates a new RuleEngine and stores it in the RuleGo rule chain pool.
	New(id string, rootRuleChainSrc []byte, opts ...RuleEngineOption) (RuleEngine, error)
	// Get retrieves a RuleEngine by its ID.
	Get(id string) (RuleEngine, bool)
	// Del deletes a RuleEngine instance by its ID.
	Del(id string)
	// Stop stops and releases all RuleEngine instances.
	Stop()
	// OnMsg invokes all RuleEngine instances to process a message.
	OnMsg(msg RuleMsg)
	// Reload reloads all RuleEngine instances.
	Reload(opts ...RuleEngineOption)
	// Range iterates over all RuleEngine instances.
	Range(f func(key, value any) bool)
}
