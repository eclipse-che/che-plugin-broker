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

	"github.com/gin-gonic/gin"
	"github.com/eclipse/che-plugin-broker/storage"
	"github.com/eclipse/che-plugin-broker/model"
	"github.com/eclipse/che-plugin-broker/broker"
)

func SetUpRouter(router *gin.Engine) {
	router.POST("/", Start)
	router.GET("/status", GetStatus)
	router.GET("/", GetResults)
	router.GET("/logs", GetResults)
}

func Start(c *gin.Context) {
	if ok, status := storage.SetStatus(model.StatusStarting); !ok {
		m := fmt.Sprintf("Starting broker in state '%s' is not allowed", status)
		c.JSON(http.StatusConflict, errors.New(m))
		return
	}

	var metas []model.PluginMeta
	if err := c.ShouldBindJSON(&metas); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := checkMetas(metas); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	go broker.ProcessPlugins(metas)
}

func GetStatus(c *gin.Context) {
	status := Status{
		storage.Status(),
	}
	c.JSON(http.StatusOK, status)
}

func GetResults(c *gin.Context) {
	status := storage.Status()
	errMess := fmt.Sprintf("Broker is in '%s' state. Plugins configuration resolution is not available", status)
	switch status {
	case model.StatusDone:
		result, err := storage.Tooling()
		if err != nil {
			errMess := fmt.Sprintf("Broker is in '%s' state but plugins configuration is broken", status)
			c.JSON(http.StatusInternalServerError, errors.New(errMess))
			return
		}
		c.JSON(http.StatusOK, result)
		return
	case model.StatusIdle:
		c.JSON(http.StatusNotFound, errors.New(errMess))
		return
	case model.StatusStarting:
		c.JSON(http.StatusNotFound, errors.New(errMess))
		return
	case model.StatusFailed:
		errMess := fmt.Sprintf("Broker is in '%s' state. Plugins configuration resolution failed with error: %s", status, storage.Err())
		c.JSON(http.StatusInternalServerError, errors.New(errMess))
		return
	default:
		c.JSON(http.StatusInternalServerError, errors.New(errMess))
		return
	}
}

func GetLogs(c *gin.Context) {
	logs := storage.Logs()
	c.JSON(http.StatusOK, logs)
}
