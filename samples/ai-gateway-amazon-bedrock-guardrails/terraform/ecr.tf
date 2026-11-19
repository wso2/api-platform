# Repositories for the gateway images. The ap CLI names images <repository>/<gateway-name>-gateway-<component>.
locals {
  gateway_image_name = "bedrock-guardrails"
  image_repositories = {
    controller = "${var.name_prefix}/${local.gateway_image_name}-gateway-controller"
    runtime    = "${var.name_prefix}/${local.gateway_image_name}-gateway-runtime"
  }
}

resource "aws_ecr_repository" "gateway" {
  #checkov:skip=CKV_AWS_51:The ap CLI tags images with the gateway version, so a rebuild reuses the tag.
  #checkov:skip=CKV_AWS_136:AES-256 default encryption is enough for images built from public sources.
  for_each             = local.image_repositories
  name                 = each.value
  force_delete         = true      # So that terraform destroy removes the sample completely.
  image_tag_mutability = "MUTABLE" # The ap CLI tags images with the gateway version, so a rebuild reuses the tag.

  image_scanning_configuration {
    scan_on_push = true
  }
}
