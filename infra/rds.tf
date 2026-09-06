data "aws_partition" "current" {}

resource "aws_iam_role" "rds_monitoring" {
  name = "${var.name_prefix}-rds-monitoring"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Action    = "sts:AssumeRole"
      Principal = { Service = "monitoring.rds.amazonaws.com" }
    }]
  })

  tags = {
    Name       = "${var.name_prefix}-rds-monitoring"
    experiment = local.experiment
  }
}

resource "aws_iam_role_policy_attachment" "rds_monitoring" {
  role       = aws_iam_role.rds_monitoring.name
  policy_arn = "arn:${data.aws_partition.current.partition}:iam::aws:policy/service-role/AmazonRDSEnhancedMonitoringRole"
}

# Identical twins except identifier + arm tag.
resource "aws_db_instance" "arm" {
  for_each = toset(["v4", "v7"])

  identifier     = "${var.name_prefix}-${each.key}"
  engine         = local.engine
  engine_version = local.engine_version
  instance_class = local.db_instance_class
  license_model  = "general-public-license"
  port           = local.mysql_port

  allocated_storage  = local.allocated_storage
  storage_type       = "gp3"
  iops               = local.iops
  storage_throughput = local.storage_throughput

  db_name  = local.db_name
  username = local.db_user
  password = random_password.exp_app.result

  db_subnet_group_name   = aws_db_subnet_group.this.name
  vpc_security_group_ids = [aws_security_group.rds.id]
  availability_zone      = local.chosen_az
  multi_az               = local.multi_az
  publicly_accessible    = local.publicly_accessible

  parameter_group_name = aws_db_parameter_group.shared.name

  backup_retention_period = local.backup_retention_period
  skip_final_snapshot     = local.skip_final_snapshot
  deletion_protection     = local.deletion_protection

  performance_insights_enabled = local.performance_insights
  monitoring_interval          = local.monitoring_interval
  monitoring_role_arn          = aws_iam_role.rds_monitoring.arn

  apply_immediately          = true
  auto_minor_version_upgrade = false

  tags = {
    Name       = "${var.name_prefix}-${each.key}"
    experiment = local.experiment
    arm        = each.key
  }

  depends_on = [aws_iam_role_policy_attachment.rds_monitoring]

  timeouts {
    create = "60m"
    update = "60m"
    delete = "30m"
  }
}
