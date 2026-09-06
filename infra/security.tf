resource "aws_security_group" "ec2" {
  name        = "${var.name_prefix}-sg-ec2"
  description = "Loadgen EC2: SSH from allowed_ssh_cidr; egress for package repos and Secrets Manager."
  vpc_id      = local.vpc_id

  tags = {
    Name       = "${var.name_prefix}-sg-ec2"
    experiment = local.experiment
    role       = "loadgen"
  }
}

resource "aws_vpc_security_group_ingress_rule" "ec2_ssh" {
  security_group_id = aws_security_group.ec2.id
  description       = "SSH only from allowed_ssh_cidr"
  cidr_ipv4         = var.allowed_ssh_cidr
  from_port         = 22
  to_port           = 22
  ip_protocol       = "tcp"
}

resource "aws_vpc_security_group_egress_rule" "ec2_all" {
  security_group_id = aws_security_group.ec2.id
  description       = "Loadgen egress (yum, Secrets Manager, MySQL to sg_rds)"
  cidr_ipv4         = "0.0.0.0/0"
  ip_protocol       = "-1"
}

resource "aws_security_group" "rds" {
  name        = "${var.name_prefix}-sg-rds"
  description = "Twin RDS: MySQL 3306 from loadgen sg only; not public."
  vpc_id      = local.vpc_id

  tags = {
    Name       = "${var.name_prefix}-sg-rds"
    experiment = local.experiment
    role       = "rds"
  }
}

resource "aws_security_group" "sm_endpoint" {
  name        = "${var.name_prefix}-sg-sm-endpoint"
  description = "Secrets Manager interface VPC endpoint; HTTPS from loadgen sg_ec2."
  vpc_id      = local.vpc_id

  tags = {
    Name       = "${var.name_prefix}-sg-sm-endpoint"
    experiment = local.experiment
    role       = "secretsmanager-endpoint"
  }
}

resource "aws_vpc_security_group_ingress_rule" "sm_endpoint_https_from_ec2" {
  security_group_id            = aws_security_group.sm_endpoint.id
  description                  = "HTTPS from loadgen sg_ec2"
  referenced_security_group_id = aws_security_group.ec2.id
  from_port                    = 443
  to_port                      = 443
  ip_protocol                  = "tcp"
}

resource "aws_vpc_security_group_ingress_rule" "rds_mysql_from_ec2" {
  security_group_id            = aws_security_group.rds.id
  description                  = "MySQL only from loadgen sg_ec2"
  referenced_security_group_id = aws_security_group.ec2.id
  from_port                    = local.mysql_port
  to_port                      = local.mysql_port
  ip_protocol                  = "tcp"
}
