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

func msgIndexHandler(ruleEngine *rulego.RuleEngine) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			postMsgHandler(ruleEngine, w, r)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
}

func postMsgHandler(ruleEngine *rulego.RuleEngine, w http.ResponseWriter, r *http.Request) {
	msgType := r.URL.Path[len(msgPath):]
	entry, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Print(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	msg := types.NewMsg(0, msgType, types.JSON, types.BuildMetadata(nil), string(entry))
	ruleEngine.OnMsg(msg)
	w.WriteHeader(http.StatusOK)
}
