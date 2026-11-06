# --------------------------------------------------------------------
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied. See the License for the
# specific language governing permissions and limitations
# under the License.
# -----------------------------------------------------------------------

# Builds Platform API images from source in one bake run, joining the compile-only
# Dockerfile.build to the image Dockerfile:
#   compile   Dockerfile.build: compiles the binary (exported at /linux-<arch>/...).
#   image     Dockerfile: the image, with its `bin` stage replaced by `compile`.
#   coverage  Dockerfile.coverage: the image plus a writable /coverage (set
#             ENABLE_COVERAGE=true so `compile` instruments the binary).
#
# Release images are not built here: `make build` / `make build-and-push-multiarch`
# build them from the binary zip with the same image Dockerfile.
#
#   docker buildx bake -f docker-bake.hcl --allow=fs.read=.. image
#
# Variables come from the environment (VERSION=… docker buildx bake …), and tags or
# platforms from --set, e.g. --set image.tags=<ref> --set image.platform=linux/amd64.

variable "VERSION" { default = "0.0.1-SNAPSHOT" }
variable "GIT_COMMIT" { default = "unknown" }
variable "PORT" { default = "9243" }
variable "EXPERIMENTAL" { default = "false" }
variable "ENABLE_COVERAGE" { default = "false" }
variable "COVERAGE_PACKAGES" { default = "" }

target "compile" {
  context    = "."
  dockerfile = "Dockerfile.build"
  target     = "binary"
  contexts = {
    common  = "../common"
    httpkit = "../httpkit"
  }
  args = {
    VERSION           = VERSION
    GIT_COMMIT        = GIT_COMMIT
    EXPERIMENTAL      = EXPERIMENTAL
    ENABLE_COVERAGE   = ENABLE_COVERAGE
    COVERAGE_PACKAGES = COVERAGE_PACKAGES
  }
}

target "image" {
  context    = "."
  dockerfile = "Dockerfile"
  contexts = {
    bin = "target:compile"
  }
  args = {
    PORT         = PORT
    EXPERIMENTAL = EXPERIMENTAL
  }
}

target "coverage" {
  context    = "."
  dockerfile = "Dockerfile.coverage"
  contexts = {
    image = "target:image"
  }
}
