/*
 * Copyright 2023 The RuleGo Authors.
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

package main

import (
	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	"io/ioutil"
	"log"
	"net/http"
)

func ruleIndexHandler(ruleEngine *rulego.RuleEngine) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			getRuleHandler(ruleEngine, w, r)
		case http.MethodPut:
			putRuleHandler(ruleEngine, w, r)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
}

// handles get requests.
func getRuleHandler(ruleEngine *rulego.RuleEngine, w http.ResponseWriter, r *http.Request) {
	nodeId := r.URL.Path[len(rulePath):]
	var def []byte
	if nodeId == "" {
		def = ruleEngine.DSL()
		_, _ = w.Write(def)
	} else {
		def = ruleEngine.NodeDSL(types.EmptyRuleNodeId, types.RuleNodeId{Id: nodeId, Type: types.NODE})
		if def == nil {
			def = ruleEngine.NodeDSL(types.EmptyRuleNodeId, types.RuleNodeId{Id: nodeId, Type: types.CHAIN})
		}
	}
	_, _ = w.Write(def)
}

func putRuleHandler(ruleEngine *rulego.RuleEngine, w http.ResponseWriter, r *http.Request) {
	nodeId := r.URL.Path[len(rulePath):]
	entry, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Print(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if nodeId == "" {
		err = ruleEngine.ReloadSelf(entry)
	} else {
		err = ruleEngine.ReloadChild(types.RuleNodeId{}, types.RuleNodeId{Id: nodeId, Type: types.NODE}, entry)
	}
	if err != nil {
		log.Print(err)
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		w.WriteHeader(http.StatusCreated)
	}
}
