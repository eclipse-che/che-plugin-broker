#!/bin/bash

set -e

ROOT_DIR=$(cd "$(dirname "$0")"; pwd)/..

TAG=$(echo $GIT_COMMIT | cut -c1-7)
docker login -u ${DOCKER_HUB_LOGIN} -p ${DOCKER_HUB_PASSWORD}

# Run tests and all the other checks on repo
docker build -f ${ROOT_DIR}/build/CI/Dockerfile ${ROOT_DIR}

# Metadata Plugin Broker
docker build -t eclipse/che-plugin-metadata-broker:latest -f ${ROOT_DIR}/build/metadata/Dockerfile ${ROOT_DIR}
docker tag eclipse/che-plugin-metadata-broker:latest eclipse/che-plugin-metadata-broker:${TAG}
docker push eclipse/che-plugin-metadata-broker:latest
docker push eclipse/che-plugin-metadata-broker:${TAG}

# Artifacts Plugin Broker
docker build -t eclipse/che-plugin-artifacts-broker:latest -f ${ROOT_DIR}/build/artifacts/Dockerfile ${ROOT_DIR}
docker tag eclipse/che-plugin-artifacts-broker:latest eclipse/che-plugin-artifacts-broker:${TAG}
docker push eclipse/che-plugin-artifacts-broker:latest
docker push eclipse/che-plugin-artifacts-broker:${TAG}

# Development image
docker build -t eclipse/che-plugin-broker-dev:latest -f ${ROOT_DIR}/build/dev/Dockerfile ${ROOT_DIR}
docker tag eclipse/che-plugin-broker-dev:latest eclipse/che-plugin-broker-dev:${TAG}
docker push eclipse/che-plugin-broker-dev:latest
docker push eclipse/che-plugin-broker-dev:${TAG}
