# --------------------------------------------------------------------
# Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied.  See the License for the
# specific language governing permissions and limitations
# under the License.
# --------------------------------------------------------------------

# Builds the integration tests' coverage image: the release image (Dockerfile) with
# instrumented browser scripts and a /coverage folder on top (Dockerfile.coverage).
#
# Both are built in one bake run, so the coverage build receives the release build
# directly ("target:release"). That works with every buildx driver; a plain
# `FROM <tag>` of a just-loaded image would not resolve on CI's docker-container
# builder and could pull an older published image of the same tag instead.
#
#   docker buildx bake -f docker-bake.coverage.hcl \
#     --allow=fs.read=.. --allow=fs.read=../../tests/framework/tools \
#     --set coverage.tags=<image> --load coverage
#
# `make build ENABLE_COVERAGE=true` runs this.

# The release Dockerfile reads api-portal/..., so it builds from portals/, the
# folder that holds this one.
target "release" {
  context    = ".."
  dockerfile = "api-portal/Dockerfile"
}

target "coverage" {
  context    = "."
  dockerfile = "Dockerfile.coverage"
  contexts = {
    release          = "target:release"
    coverage-scripts = "../../tests/framework/tools"
  }
}
