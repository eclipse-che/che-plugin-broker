#
# Copyright (c) 2012-2018 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#
# Contributors:
#   Red Hat, Inc. - initial API and implementation
#

FROM golang:1.10.3 as builder
WORKDIR /go/src/github.com/eclipse/che-plugin-broker/
COPY . .
WORKDIR /go/src/github.com/eclipse/che-plugin-broker/brokers/che-plugin-broker/
RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-w -s' -a -installsuffix cgo -o che-plugin-broker main.go


FROM alpine:3.7
RUN apk add --no-cache ca-certificates
COPY --from=builder /go/src/github.com/eclipse/che-plugin-broker/brokers/che-plugin-broker/che-plugin-broker /usr/local/bin
ENTRYPOINT ["che-plugin-broker"]
