# Locked 1:1 names for scripts/export_env_from_tf.sh.
# Do not add mysql_password or rds_endpoint_* aliases.

output "mysql_host_v4" {
  description = "RDS address for the v4 twin (MYSQL_HOST_V4)."
  value       = aws_db_instance.arm["v4"].address
}

output "mysql_host_v7" {
  description = "RDS address for the v7 twin (MYSQL_HOST_V7)."
  value       = aws_db_instance.arm["v7"].address
}

output "mysql_port" {
  description = "MySQL port (MYSQL_PORT)."
  value       = local.mysql_port
}

output "mysql_database" {
  description = "Database name (MYSQL_DATABASE)."
  value       = local.db_name
}

output "mysql_user" {
  description = "App user (MYSQL_USER). Password is only in Secrets Manager."
  value       = local.db_user
}

output "mysql_secret_arn" {
  description = "Secrets Manager ARN for exp_app JSON {username,password} (MYSQL_SECRET_ARN)."
  value       = aws_secretsmanager_secret.exp_app.arn
}

output "mysql_tls" {
  description = "Same-VPC default is false (MYSQL_TLS). Harness uses tls=skip-verify only when true."
  value       = local.mysql_tls
}

output "aws_region" {
  description = "AWS region (AWS_REGION)."
  value       = var.aws_region
}

output "loadgen_instance_id" {
  description = "Loadgen EC2 instance id (LOADGEN_INSTANCE_HINT)."
  value       = aws_instance.loadgen.id
}

output "loadgen_subnet_id" {
  description = "Subnet of the loadgen instance (infra-only; not harness env)."
  value       = aws_instance.loadgen.subnet_id
}

output "loadgen_sg_id" {
  description = "Loadgen security group id (infra-only; not harness env)."
  value       = aws_security_group.ec2.id
}

output "rds_sg_id" {
  description = "RDS security group id (infra-only; not harness env)."
  value       = aws_security_group.rds.id
}
