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
