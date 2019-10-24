# Copyright (c) 2019 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#
# Contributors:
#   Red Hat, Inc. - initial API and implementation
#

# https://access.redhat.com/containers/?tab=tags#/registry.access.redhat.com/devtools/go-toolset-rhel7
FROM registry.access.redhat.com/devtools/go-toolset-rhel7:1.11.13-11 as builder
ENV PATH=/opt/rh/go-toolset-1.11/root/usr/bin:$PATH \
    GOPATH=/go/
USER root
WORKDIR /go/src/github.com/eclipse/che-plugin-broker/brokers/init/cmd/
COPY . /go/src/github.com/eclipse/che-plugin-broker/
RUN adduser appuser && \
    CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-w -s' -installsuffix cgo -o init-plugin-broker main.go

FROM scratch

USER appuser
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /go/src/github.com/eclipse/che-plugin-broker/brokers/init/cmd/init-plugin-broker /
ENTRYPOINT ["/init-plugin-broker"]

# append Brew metadata here
