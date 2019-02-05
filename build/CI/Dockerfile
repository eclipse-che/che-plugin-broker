#
# Copyright (c) 2018-2019 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#
# Contributors:
#   Red Hat, Inc. - initial API and implementation
#

FROM golang:1.10
RUN wget -O - -q https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s v1.10.2
RUN adduser --disabled-password --gecos '' appuser
USER appuser
COPY . /go/src/github.com/eclipse/che-plugin-broker/
WORKDIR /go/src/github.com/eclipse/che-plugin-broker/
RUN  CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-w -s' -a -installsuffix cgo ./...
RUN golangci-lint run -v
RUN go test -v -race ./...
