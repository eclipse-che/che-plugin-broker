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
	"github.com/eclipse/che-go-jsonrpc"
	"github.com/eclipse/che-go-jsonrpc/event"
	"github.com/eclipse/che-go-jsonrpc/jsonrpcws"
	"log"
)

func closeConsumers() {
	for _, candidates := range bus.Clear() {
		for _, candidate := range candidates {
			if broadcaster, ok := candidate.(*tunnelBroadcaster); ok {
				broadcaster.Close()
			}
		}
	}
}

func (tb *tunnelBroadcaster) Close() { tb.tunnel.Close() }

type tunnelBroadcaster struct {
	tunnel *jsonrpc.Tunnel
}

func (tb *tunnelBroadcaster) Accept(e event.E) {
	if err := tb.tunnel.Notify(e.Type(), e); err != nil {
		log.Fatalf("Trying to send event of type '%s' to closed tunnel '%s'", e.Type(), tb.tunnel.ID())
	}
}

func ConnectOrFail(endpoint string, token string) *jsonrpc.Tunnel {
	tunnel, err := Connect(endpoint, token)
	if err != nil {
		log.Fatalf("Couldn't connect to endpoint '%s', due to error '%s'", endpoint, err)
	}
	return tunnel
}

func Connect(endpoint string, token string) (*jsonrpc.Tunnel, error) {
	conn, err := jsonrpcws.Dial(endpoint, token)
	if err != nil {
		return nil, err
	}
	return jsonrpc.NewManagedTunnel(conn), nil
}

type WSDialConnector struct {
	Endpoint string
	Token    string
}

func (c *WSDialConnector) Connect() (*jsonrpc.Tunnel, error) { return Connect(c.Endpoint, c.Token) }