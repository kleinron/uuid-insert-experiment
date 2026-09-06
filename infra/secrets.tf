# One shared secret for exp_app on both twins. JSON {username,password} — harness
# happy path is MYSQL_SECRET_ARN (see scripts/export_env_from_tf.sh). Never output the password.
resource "random_password" "exp_app" {
  length           = 32
  special          = true
  override_special = "!#$%&*()-_=+[]{}" # RDS MySQL forbids / @ " ' and space
}

resource "aws_secretsmanager_secret" "exp_app" {
  name                    = "${var.name_prefix}/exp_app"
  description             = "exp_app credentials for uuid-insert twin RDS (JSON username/password)."
  recovery_window_in_days = 0 # immediate delete so same-day destroy + re-apply works

  tags = {
    Name       = "${var.name_prefix}/exp_app"
    experiment = local.experiment
    role       = "db-credentials"
  }
}

resource "aws_secretsmanager_secret_version" "exp_app" {
  secret_id = aws_secretsmanager_secret.exp_app.id
  secret_string = jsonencode({
    username = local.db_user
    password = random_password.exp_app.result
  })
}
