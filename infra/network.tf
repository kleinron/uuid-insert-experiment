# Existing VPC + subnets. Pick one AZ so both RDS twins and the loadgen share it.
data "aws_subnet" "provided" {
  for_each = toset(var.subnet_ids)
  id       = each.value
}

locals {
  experiment = "uuid-insert"

  db_name    = "payments_exp"
  db_user    = "exp_app"
  mysql_port = 3306
  mysql_tls  = false

  # Locked BOM
  engine                  = "mysql"
  engine_version          = "8.0"
  db_instance_class       = "db.r6g.large"
  allocated_storage       = 120
  iops                    = 12000
  storage_throughput      = 500
  innodb_buffer_pool_size = "12884901888" # 12 GiB
  loadgen_instance_type   = "c6i.large"
  monitoring_interval     = 60
  backup_retention_period = 0
  performance_insights    = true
  publicly_accessible     = false
  multi_az                = false
  skip_final_snapshot     = true
  deletion_protection     = false

  chosen_az = coalesce(var.availability_zone, data.aws_subnet.provided[var.subnet_ids[0]].availability_zone)

  subnets_in_az = [
    for id in var.subnet_ids : id
    if data.aws_subnet.provided[id].availability_zone == local.chosen_az
  ]

  loadgen_subnet_id = try(local.subnets_in_az[0], null)

  # Interface endpoints allow one subnet per AZ. Prefer the loadgen subnet in the chosen AZ.
  sm_endpoint_subnet_ids = [
    for az, ids in {
      for id, s in data.aws_subnet.provided : s.availability_zone => id...
    } : contains(ids, coalesce(local.loadgen_subnet_id, "")) ? local.loadgen_subnet_id : ids[0]
  ]
}

data "aws_vpc" "this" {
  id = var.vpc_id
}

# Always-on so associate_public_ip=false / no-NAT loadgen can still GetSecretValue.
resource "aws_vpc_endpoint" "secretsmanager" {
  vpc_id              = var.vpc_id
  service_name        = "com.amazonaws.${var.aws_region}.secretsmanager"
  vpc_endpoint_type   = "Interface"
  subnet_ids          = local.sm_endpoint_subnet_ids
  security_group_ids  = [aws_security_group.sm_endpoint.id]
  private_dns_enabled = true

  tags = {
    Name       = "${var.name_prefix}-sm-endpoint"
    experiment = local.experiment
    role       = "secretsmanager-endpoint"
  }

  lifecycle {
    precondition {
      condition     = length(local.sm_endpoint_subnet_ids) > 0
      error_message = "Secrets Manager VPC endpoint needs at least one subnet; none of subnet_ids resolved."
    }
    precondition {
      condition     = data.aws_vpc.this.enable_dns_support && data.aws_vpc.this.enable_dns_hostnames
      error_message = "VPC ${var.vpc_id} must have enable_dns_support and enable_dns_hostnames so the Secrets Manager interface endpoint can use private_dns_enabled=true."
    }
  }
}

resource "aws_db_subnet_group" "this" {
  name        = var.name_prefix
  description = "Shared subnet group for uuid-insert RDS twins (Single-AZ instances still need 2 AZs in the group)."
  subnet_ids  = var.subnet_ids

  tags = {
    Name       = var.name_prefix
    experiment = local.experiment
  }

  lifecycle {
    precondition {
      condition     = length(local.subnets_in_az) > 0
      error_message = "No subnet in subnet_ids is in availability zone ${local.chosen_az}. Add a subnet in that AZ or change availability_zone."
    }
  }
}
