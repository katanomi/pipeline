#!/usr/bin/env bash

# Copyright 2019 The Tekton Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o errexit
set -o nounset
set -o pipefail

export KO_DOCKER_REPO="gcr.io/tekton-nightly"
# Build the base image for git images.
CONTAINER_ENGINE="${CONTAINER_ENGINE:-}"
if [[ -z "${CONTAINER_ENGINE}" ]]; then
  if command -v podman >/dev/null 2>&1; then
    CONTAINER_ENGINE=podman
  elif command -v docker >/dev/null 2>&1; then
    CONTAINER_ENGINE=docker
  else
    echo "container engine not found (podman/docker)" >&2
    exit 1
  fi
fi

${CONTAINER_ENGINE} build -t "${KO_DOCKER_REPO}/github.com/tektoncd/pipeline/base" -f images/Dockerfile images/
${CONTAINER_ENGINE} push "${KO_DOCKER_REPO}/github.com/tektoncd/pipeline/base"
