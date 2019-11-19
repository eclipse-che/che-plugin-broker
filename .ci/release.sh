#!/bin/bash

# Environment variables that are needed to be set:
# GITHUB_TAG - tag that will be pushed to Github. Example: v0.15.5
# IMAGE_TAG - container image tag that will be pushed to docker regsitry. Example: v0.15.5
# IMAGE_LATEST_TAG - container image for bugfixes that will be pushed to docker registry. Example: v0.15
# Note that tags folow v+Semiver naming. IMAGE_LATEST_TAG is supposed to be latest version of Minor component of Semiver. 

set -e

if [ -z "${GITHUB_TAG}" ]; then
  echo "Variable GITHUB_TAG is missing"
  exit 1
fi
if [ -z "${IMAGE_TAG}" ]; then
  echo "Variable IMAGE_TAG is missing"
  exit 1
fi
if [ -z "${IMAGE_LATEST_TAG}" ]; then
  echo "Variable IMAGE_LATEST_TAG is missing"
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

# Artifacts Plugin Broker
docker build -t eclipse/che-plugin-artifacts-broker:${IMAGE_TAG} -f ${ROOT_DIR}/build/artifacts/Dockerfile ${ROOT_DIR}
docker push eclipse/che-plugin-artifacts-broker:${IMAGE_TAG}
# Push latest bugfix image
docker tag eclipse/che-plugin-artifacts-broker:${IMAGE_TAG} eclipse/che-plugin-artifacts-broker:${IMAGE_LATEST_TAG}
docker push eclipse/che-plugin-artifacts-broker:${IMAGE_LATEST_TAG}

# Metadata Plugin Broker
docker build -t eclipse/che-plugin-metadata-broker:${IMAGE_TAG} -f ${ROOT_DIR}/build/metadata/Dockerfile ${ROOT_DIR}
docker push eclipse/che-plugin-metadata-broker:${IMAGE_TAG}
# Push latest bugfix image
docker tag eclipse/che-plugin-metadata-broker:${IMAGE_TAG} eclipse/che-plugin-metadata-broker:${IMAGE_LATEST_TAG}
docker push eclipse/che-plugin-metadata-broker:${IMAGE_LATEST_TAG}
