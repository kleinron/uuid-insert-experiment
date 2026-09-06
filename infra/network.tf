# Dedicated experiment VPC. No BYO network: apply creates VPC, two public
# subnets in different AZs, IGW, and a public route table. Loadgen + both
# RDS twins are pinned to one AZ; the second AZ exists only so the DB subnet
# group satisfies AWS (Single-AZ instances still need >= 2 AZs in the group).

data "aws_availability_zones" "available" {
  state = "available"
}

locals {
  experiment = "uuid-insert"

  db_name    = "payments_exp"
  db_user    = "exp_app"
  mysql_port = 3306
  mysql_tls  = false

  # Locked BOM
  engine                  = "mysql"
  engine_version          = "8.0.46" # exact minor; current RDS MySQL 8.0.x (eu-central-1 / AWS docs)
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

  available_azs = data.aws_availability_zones.available.names
  chosen_az     = coalesce(var.availability_zone, try(local.available_azs[0], null))
  other_azs     = [for az in local.available_azs : az if az != local.chosen_az]
  second_az     = try(local.other_azs[0], null)
  public_azs    = compact([local.chosen_az, local.second_az])

  vpc_id            = aws_vpc.this.id
  loadgen_subnet_id = aws_subnet.public[local.chosen_az].id
  subnet_ids        = [for az in local.public_azs : aws_subnet.public[az].id]

  # Interface endpoints allow one subnet per AZ. Include the loadgen AZ first.
  sm_endpoint_subnet_ids = local.subnet_ids
}

resource "aws_vpc" "this" {
  cidr_block           = var.vpc_cidr
  enable_dns_support   = true
  enable_dns_hostnames = true

  tags = {
    Name       = "${var.name_prefix}-vpc"
    experiment = local.experiment
  }

  lifecycle {
    precondition {
      condition     = length(local.available_azs) >= 2
      error_message = "Region ${var.aws_region} needs at least two availability zones for the RDS DB subnet group."
    }
    precondition {
      condition     = local.chosen_az != null && contains(local.available_azs, local.chosen_az)
      error_message = "availability_zone ${coalesce(local.chosen_az, "(unset)")} is not available in ${var.aws_region}."
    }
  }
}

resource "aws_internet_gateway" "this" {
  vpc_id = aws_vpc.this.id

  tags = {
    Name       = "${var.name_prefix}-igw"
    experiment = local.experiment
  }
}

resource "aws_subnet" "public" {
  for_each = {
    for idx, az in local.public_azs : az => {
      az   = az
      cidr = cidrsubnet(var.vpc_cidr, 8, idx)
    }
  }

  vpc_id                  = aws_vpc.this.id
  availability_zone       = each.value.az
  cidr_block              = each.value.cidr
  map_public_ip_on_launch = true

  tags = {
    Name       = "${var.name_prefix}-public-${each.value.az}"
    experiment = local.experiment
    role       = "public"
  }
}

resource "aws_route_table" "public" {
  vpc_id = aws_vpc.this.id

  tags = {
    Name       = "${var.name_prefix}-public"
    experiment = local.experiment
  }
}

resource "aws_route" "public_internet" {
  route_table_id         = aws_route_table.public.id
  destination_cidr_block = "0.0.0.0/0"
  gateway_id             = aws_internet_gateway.this.id
}

resource "aws_route_table_association" "public" {
  for_each = aws_subnet.public

  subnet_id      = each.value.id
  route_table_id = aws_route_table.public.id
}

# Always-on so the loadgen can GetSecretValue via private DNS (no NAT required
# for Secrets Manager). yum/dnf still uses the public subnet + IGW.
resource "aws_vpc_endpoint" "secretsmanager" {
  vpc_id              = local.vpc_id
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
}

resource "aws_db_subnet_group" "this" {
  name        = var.name_prefix
  description = "Shared subnet group for uuid-insert RDS twins (Single-AZ instances still need 2 AZs in the group)."
  subnet_ids  = local.subnet_ids

  tags = {
    Name       = var.name_prefix
    experiment = local.experiment
  }
}
