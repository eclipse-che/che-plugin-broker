#!/bin/bash

set -e

if [ -z "${GITHUB_TAG}" ]; then
  echo "Variable GITHUB_TAG is missing"
  exit 1
fi
if [ -z "${IMAGE_TAG}" ]; then
  echo "Variable IMAGE_TAG is missing"
  exit 1
fi

ROOT_DIR=$(cd "$(dirname "$0")"; pwd)/..

docker login -u ${DOCKER_HUB_LOGIN} -p ${DOCKER_HUB_PASSWORD}

# Run tests and all the other checks on repo
docker build -f ${ROOT_DIR}/build/CI/Dockerfile ${ROOT_DIR}

# create and push new tag
git tag $GITHUB_TAG
git push origin $GITHUB_TAG

# checkout to new tag
git checkout $GITHUB_TAG

# Unified Plugin Broker
docker build -t eclipse/che-unified-plugin-broker:${IMAGE_TAG} -f ${ROOT_DIR}/build/unified/Dockerfile ${ROOT_DIR}
docker push eclipse/che-unified-plugin-broker:${IMAGE_TAG}

# Init Plugin Broker
docker build -t eclipse/che-init-plugin-broker:${IMAGE_TAG} -f ${ROOT_DIR}/build/init/Dockerfile ${ROOT_DIR}
docker push eclipse/che-init-plugin-broker:${IMAGE_TAG}
