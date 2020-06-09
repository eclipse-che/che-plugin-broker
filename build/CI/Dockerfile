#
# Copyright (c) 2018-2020 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#
# Contributors:
#   Red Hat, Inc. - initial API and implementation
#

FROM golang:1.12
RUN  export ARCH="$(uname -m)" && export RACE="-race" && if [ ${ARCH} = "x86_64" ]; then export ARCH="amd64"; elif [ ${ARCH} = "s390x" ];then export RACE=""; fi \
&&  wget https://github.com/golangci/golangci-lint/releases/download/v1.20.0/golangci-lint-1.20.0-linux-${ARCH}.tar.gz \
&&  tar xf golangci-lint-1.20.0-linux-${ARCH}.tar.gz -C /usr/local/bin --strip-components 1
RUN adduser --disabled-password --gecos '' appuser
USER appuser
COPY . /go/src/github.com/eclipse/che-plugin-broker/
WORKDIR /go/src/github.com/eclipse/che-plugin-broker/
RUN  CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-w -s' -a -installsuffix cgo ./...
RUN golangci-lint run -v --timeout=5m
RUN go test -v $RACE ./...
