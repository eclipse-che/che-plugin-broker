//
// Copyright (c) 2012-2018 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package api

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/eclipse/che-plugin-broker/broker"
	"github.com/eclipse/che-plugin-broker/model"
	"github.com/eclipse/che-plugin-broker/resttemp"
	"github.com/eclipse/che-plugin-broker/storage"
	"github.com/eclipse/che/agents/go-agents/core/rest"
	"github.com/eclipse/che/agents/go-agents/core/rest/restutil"
)

// HTTPRoutes provides all routes that should be handled by the broker API
var HTTPRoutes = rest.RoutesGroup{
	Name: "Process Routes",
	Items: []rest.Route{
		{
			Method:     "POST",
			Name:       "Start broker",
			Path:       "/",
			HandleFunc: start,
		},
		{
			Method:     "GET",
			Name:       "Get status",
			Path:       "/status",
			HandleFunc: getStatus,
		},
		{
			Method:     "GET",
			Name:       "Get results",
			Path:       "/",
			HandleFunc: getResults,
		},
		{
			Method:     "GET",
			Name:       "Get results",
			Path:       "/logs",
			HandleFunc: getLogs,
		},
	},
}

func start(w http.ResponseWriter, r *http.Request, _ rest.Params) error {
	if ok, status := storage.SetStatus(model.StatusStarting); !ok {
		m := fmt.Sprintf("Starting broker in state '%s' is not allowed", status)
		return rest.Conflict(errors.New(m))
	}

	var metas []model.PluginMeta
	if err := restutil.ReadJSON(r, &metas); err != nil {
		return err
	}
	if err := checkMetas(metas); err != nil {
		return rest.BadRequest(err)
	}

	go broker.ProcessPlugins(metas)
	return nil
}

func getStatus(w http.ResponseWriter, r *http.Request, _ rest.Params) error {
	status := Status{
		storage.Status(),
	}
	return restutil.WriteJSON(w, status)
}

func getResults(w http.ResponseWriter, r *http.Request, _ rest.Params) error {
	status := storage.Status()
	errMess := fmt.Sprintf("Broker is in '%s' state. Plugins configuration resolution is not available", status)
	switch status {
	case model.StatusDone:
		result, err := storage.Tooling()
		if err != nil {
			errMess := fmt.Sprintf("Broker is in '%s' state but plugins configuration is broken", status)
			return resttemp.ServerError(errors.New(errMess))
		}
		return restutil.WriteJSON(w, result)
	case model.StatusIdle:
		return rest.NotFound(errors.New(errMess))
	case model.StatusStarting:
		return rest.NotFound(errors.New(errMess))
	case model.StatusFailed:
		errMess := fmt.Sprintf("Broker is in '%s' state. Plugins configuration resolution failed with error: %s", status, storage.Err())
		return resttemp.ServerError(errors.New(errMess))
	default:
		return resttemp.ServerError(errors.New(errMess))
	}
}

func getLogs(w http.ResponseWriter, r *http.Request, _ rest.Params) error {
	logs := storage.Logs()
	return restutil.WriteJSON(w, logs)
}
