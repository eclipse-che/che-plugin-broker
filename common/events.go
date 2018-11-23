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

package common

import (
	"bytes"
	"fmt"
	"github.com/eclipse/che-plugin-broker/cfg"
	"github.com/eclipse/che-plugin-broker/model"
	"log"
	"time"
)

func (broker *Broker) PubStarted() {
	broker.bus.Pub(&model.StartedEvent{
		Status:    model.StatusStarted,
		RuntimeID: cfg.RuntimeID,
	})
}

func (broker *Broker) PubFailed(err string) {
	broker.bus.Pub(&model.ErrorEvent{
		Status:    model.StatusFailed,
		Error:     err,
		RuntimeID: cfg.RuntimeID,
	})
}

func (broker *Broker) PubDone(tooling string) {
	broker.bus.Pub(&model.SuccessEvent{
		Status:    model.StatusDone,
		RuntimeID: cfg.RuntimeID,
		Tooling:   tooling,
	})
}

func (broker *Broker) PubLog(text string) {
	broker.bus.Pub(&model.PluginBrokerLogEvent{
		RuntimeID: cfg.RuntimeID,
		Text:      text,
		Time:      time.Now(),
	})
}

func (broker *Broker) PrintPlan(metas []model.PluginMeta) {
	var buffer bytes.Buffer

	buffer.WriteString("List of plugins and editors to install\n")
	for _, plugin := range metas {
		buffer.WriteString(fmt.Sprintf("- %s:%s - %s\n", plugin.ID, plugin.Version, plugin.Description))
	}

	broker.PrintInfo(buffer.String())
}

func (broker *Broker) PrintDebug(format string, v ...interface{}) {
	log.Printf(format, v...)
}

func (broker *Broker) PrintInfo(format string, v ...interface{}) {
	message := fmt.Sprintf(format, v...)
	broker.PubLog(message)
	log.Print(message)
}

func (broker *Broker) PrintFatal(format string, v ...interface{}) {
	message := fmt.Sprintf(format, v...)
	broker.PubLog(message)
	log.Fatal(message)
}