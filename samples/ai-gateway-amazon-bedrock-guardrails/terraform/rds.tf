# The database that the gateway controller replicas share. Its password is managed in AWS Secrets Manager.
resource "aws_db_subnet_group" "main" {
  name       = var.name_prefix
  subnet_ids = aws_subnet.private[*].id
}

resource "aws_security_group" "db" {
  name        = "${var.name_prefix}-db"
  description = "PostgreSQL from the EKS nodes only"
  vpc_id      = aws_vpc.main.id

  ingress {
    description     = "PostgreSQL from the EKS cluster security group"
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    security_groups = [aws_eks_cluster.main.vpc_config[0].cluster_security_group_id]
  }
}

resource "aws_db_instance" "main" {
  #checkov:skip=CKV_AWS_161:The gateway authenticates with a password from Secrets Manager.
  #checkov:skip=CKV_AWS_293:Off so that terraform destroy removes the sample completely.
  #checkov:skip=CKV_AWS_353:Not needed for a sample. Turn it on in production.
  #checkov:skip=CKV_AWS_157:Controlled by db_multi_az, off by default to limit cost.
  #checkov:skip=CKV_AWS_118:Not needed for a sample. Turn it on in production.
  #checkov:skip=CKV2_AWS_30:Query logs can contain configuration data. Turn it on only with a retention plan.
  identifier                      = var.name_prefix
  engine                          = "postgres"
  engine_version                  = "16"
  instance_class                  = var.db_instance_class
  allocated_storage               = 20
  storage_encrypted               = true
  db_name                         = "gateway"
  username                        = "gateway"
  manage_master_user_password     = true
  multi_az                        = var.db_multi_az
  db_subnet_group_name            = aws_db_subnet_group.main.name
  vpc_security_group_ids          = [aws_security_group.db.id]
  publicly_accessible             = false
  deletion_protection             = false # So that terraform destroy removes the sample completely.
  skip_final_snapshot             = true
  backup_retention_period         = 1
  copy_tags_to_snapshot           = true
  auto_minor_version_upgrade      = true
  enabled_cloudwatch_logs_exports = ["postgresql", "upgrade"]

  # The log groups must exist first, or Amazon RDS creates its own without a retention period.
  depends_on = [aws_cloudwatch_log_group.rds]
}
