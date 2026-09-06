resource "aws_db_parameter_group" "shared" {
  name        = "${var.name_prefix}-mysql80"
  family      = "mysql8.0"
  description = "Shared MySQL 8.0 parameter group for uuid-insert twins (12 GiB InnoDB buffer pool)."

  parameter {
    name         = "innodb_buffer_pool_size"
    value        = local.innodb_buffer_pool_size
    apply_method = "pending-reboot"
  }

  tags = {
    Name       = "${var.name_prefix}-mysql80"
    experiment = local.experiment
  }
}
