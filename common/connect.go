//
// Copyright (c) 2018-2019 Red Hat, Inc.
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
	"net/http"
	"log"

	"github.com/eclipse/che-go-jsonrpc"
	"github.com/eclipse/che-go-jsonrpc/event"
	"github.com/eclipse/che-go-jsonrpc/jsonrpcws"
	"crypto/x509"
	"io/ioutil"
	"crypto/tls"
)

type tunnelBroadcaster struct {
	tunnel *jsonrpc.Tunnel
}

func (tb *tunnelBroadcaster) Close() { tb.tunnel.Close() }

func (tb *tunnelBroadcaster) Accept(e event.E) {
	if err := tb.tunnel.Notify(e.Type(), e); err != nil {
		log.Fatalf("Trying to send event of type '%s' to closed tunnel '%s'", e.Type(), tb.tunnel.ID())
	}
}

func ConfigureCertPool(customCertificateFilePath string) {
	// Get the SystemCertPool, continue with an empty pool on error
	rootCAs, _ := x509.SystemCertPool()
	if rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}

	// Read in the cert file
	certs, err := ioutil.ReadFile(customCertificateFilePath)
	if err != nil {
		log.Fatalf("Failed to read custom certificate %q. Error: %v", customCertificateFilePath, err)
	}

	// Append our cert to the system pool
	if ok := rootCAs.AppendCertsFromPEM(certs); !ok {
		log.Fatalf("Failed to append %q to RootCAs: %v", customCertificateFilePath, err)
	}

	// Trust the augmented cert pool in our client
	jsonrpcws.DefaultDialer.TLSClientConfig = &tls.Config{
		RootCAs: rootCAs,
	}

	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{
		RootCAs: rootCAs,
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
