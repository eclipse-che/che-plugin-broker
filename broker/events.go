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

package broker

import (
	"bytes"
	"fmt"
	"github.com/eclipse/che-plugin-broker/cfg"
	"github.com/eclipse/che-plugin-broker/model"
	"log"
	"time"
)

func pubStarted() {
	bus.Pub(&model.StartedEvent{
		Status:    model.StatusStarted,
		RuntimeID: cfg.RuntimeID,
	})
}

func pubFailed(err string) {
	bus.Pub(&model.ErrorEvent{
		Status:    model.StatusFailed,
		Error:     err,
		RuntimeID: cfg.RuntimeID,
	})
}

func pubDone(tooling string) {
	bus.Pub(&model.SuccessEvent{
		Status:    model.StatusDone,
		RuntimeID: cfg.RuntimeID,
		Tooling:   tooling,
	})
}

func pubLog(text string) {
	bus.Pub(&model.PluginBrokerLogEvent{
		RuntimeID: cfg.RuntimeID,
		Text:      text,
		Time:      time.Now(),
	})
}

func printPlan(metas []model.PluginMeta) {
	var buffer bytes.Buffer

	buffer.WriteString("List of plugins and editors to install\n")
	for _, plugin := range metas {
		buffer.WriteString(fmt.Sprintf("- %s:%s - %s\n", plugin.ID, plugin.Version, plugin.Description))
	}

	printInfo(buffer.String())
}

func printDebug(format string, v ...interface{}) {
	log.Printf(format, v...)
}

func printInfo(format string, v ...interface{}) {
	message := fmt.Sprintf(format, v...)
	pubLog(message)
	log.Print(message)
}

func printFatal(format string, v ...interface{}) {
	message := fmt.Sprintf(format, v...)
	pubLog(message)
	log.Fatal(message)
}
