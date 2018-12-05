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
	"github.com/eclipse/che-go-jsonrpc"
	"github.com/eclipse/che-go-jsonrpc/event"
	"github.com/eclipse/che-plugin-broker/model"
)

// Broker holds utilities to interact with Che master to push different events
type Broker interface {
	CloseConsumers()
	Bus() *event.Bus
	PushEvents(tun *jsonrpc.Tunnel, types ...string)
	PubStarted()
	PubFailed(err string)
	PubDone(tooling string)
	PubLog(text string)
	PrintPlan(metas []model.PluginMeta)
	PrintDebug(format string, v ...interface{})
	PrintInfo(format string, v ...interface{})
	PrintFatal(format string, v ...interface{})
}

type brokerImpl struct {
	bus *event.Bus
}

func NewBroker() Broker {
	return &brokerImpl{event.NewBus()}
}

// PushEvents sets given tunnel as consumer of broker events.
func (broker brokerImpl) PushEvents(tun *jsonrpc.Tunnel, types ...string) {
	broker.Bus().SubAny(&tunnelBroadcaster{tunnel: tun}, types...)
}

func (broker brokerImpl) Bus() *event.Bus {
	return broker.bus
}

func (broker brokerImpl) CloseConsumers() {
	for _, candidates := range broker.bus.Clear() {
		for _, candidate := range candidates {
			if broadcaster, ok := candidate.(*tunnelBroadcaster); ok {
				broadcaster.Close()
			}
		}
	}
}
