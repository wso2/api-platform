# Interface VPC endpoint for Amazon Bedrock Runtime, powered by AWS PrivateLink. With private DNS,
# the gateway's Converse and ApplyGuardrail calls stay on the AWS network instead of going through
# the NAT gateway.

resource "aws_security_group" "bedrock_endpoint" {
  name        = "${var.name_prefix}-bedrock-endpoint"
  description = "HTTPS to the Amazon Bedrock Runtime endpoint from inside the VPC"
  vpc_id      = aws_vpc.main.id

  ingress {
    description = "HTTPS from the VPC"
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = [var.vpc_cidr]
  }
}

resource "aws_vpc_endpoint" "bedrock_runtime" {
  vpc_id              = aws_vpc.main.id
  service_name        = "com.amazonaws.${var.region}.bedrock-runtime"
  vpc_endpoint_type   = "Interface"
  subnet_ids          = aws_subnet.private[*].id
  security_group_ids  = [aws_security_group.bedrock_endpoint.id]
  private_dns_enabled = true
  tags                = { Name = "${var.name_prefix}-bedrock-runtime" }
}
