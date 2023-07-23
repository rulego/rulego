package main

import (
	"io/ioutil"
	"log"
	"net/http"
	"rulego"
	"rulego/api/types"
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
