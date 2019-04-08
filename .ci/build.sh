#!/bin/bash

set -e

ROOT_DIR=$(cd "$(dirname "$0")"; pwd)/..

TAG=$(echo $GIT_COMMIT | cut -c1-7)
docker login -u ${DOCKER_HUB_LOGIN} -p ${DOCKER_HUB_PASSWORD}

# Run tests and all the other checks on repo
docker build -f ${ROOT_DIR}/build/CI/Dockerfile ${ROOT_DIR}

# Unified Plugin Broker
docker build -t eclipse/che-unified-plugin-broker:latest -f ${ROOT_DIR}/build/unified/Dockerfile ${ROOT_DIR}
docker tag eclipse/che-unified-plugin-broker:latest eclipse/che-unified-plugin-broker:${TAG}
docker push eclipse/che-unified-plugin-broker:latest
docker push eclipse/che-unified-plugin-broker:${TAG}

# Init Plugin Broker
docker build -t eclipse/che-init-plugin-broker:latest -f ${ROOT_DIR}/build/init/Dockerfile ${ROOT_DIR}
docker tag eclipse/che-init-plugin-broker:latest eclipse/che-init-plugin-broker:${TAG}
docker push eclipse/che-init-plugin-broker:latest
docker push eclipse/che-init-plugin-broker:${TAG}
