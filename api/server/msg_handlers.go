package main

import (
	"io/ioutil"
	"log"
	"net/http"
	"rulego"
	"rulego/api/types"
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
