resource "aws_db_parameter_group" "shared" {
  name        = "${var.name_prefix}-mysql80"
  family      = "mysql8.0"
  description = "Shared MySQL 8.0 parameter group for uuid-insert twins (DE InnoDB + 12 GiB buffer pool)."

  # Locked BOM — 12 GiB. Static.
  parameter {
    name         = "innodb_buffer_pool_size"
    value        = local.innodb_buffer_pool_size
    apply_method = "pending-reboot"
  }

  # DE: match gp3 provisioned IOPS headroom (iops=12000).
  parameter {
    name         = "innodb_io_capacity"
    value        = "5000"
    apply_method = "immediate"
  }

  parameter {
    name         = "innodb_io_capacity_max"
    value        = "12000"
    apply_method = "immediate"
  }

  # DE: explicit durable pair (do not relax either).
  parameter {
    name         = "innodb_flush_log_at_trx_commit"
    value        = "1"
    apply_method = "immediate"
  }

  parameter {
    name         = "sync_binlog"
    value        = "1"
    apply_method = "immediate"
  }

  parameter {
    name         = "innodb_redo_log_capacity"
    value        = "2147483648" # 2 GiB
    apply_method = "immediate"
  }

  # Skip buffer-pool dump/load so twins start from a cold cache, not a leftover dump.
  parameter {
    name         = "innodb_buffer_pool_dump_at_shutdown"
    value        = "0"
    apply_method = "immediate"
  }

  parameter {
    name         = "innodb_buffer_pool_load_at_startup"
    value        = "0"
    apply_method = "pending-reboot"
  }

  parameter {
    name         = "innodb_flush_neighbors"
    value        = "0"
    apply_method = "immediate"
  }

  parameter {
    name         = "innodb_adaptive_hash_index"
    value        = "0"
    apply_method = "immediate"
  }

  # innodb_doublewrite is intentionally unset — leave RDS/MySQL default ON.
  # Do not set innodb_doublewrite=0.

  tags = {
    Name       = "${var.name_prefix}-mysql80"
    experiment = local.experiment
  }
}
