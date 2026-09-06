data "aws_ssm_parameter" "al2023" {
  name = "/aws/service/ami-amazon-linux-latest/al2023-ami-kernel-default-x86_64"
}

data "aws_iam_policy_document" "loadgen_assume" {
  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["ec2.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "loadgen" {
  name               = "${var.name_prefix}-loadgen"
  assume_role_policy = data.aws_iam_policy_document.loadgen_assume.json

  tags = {
    Name       = "${var.name_prefix}-loadgen"
    experiment = local.experiment
    role       = "loadgen"
  }
}

data "aws_iam_policy_document" "loadgen_secret" {
  statement {
    sid       = "ReadExpAppSecret"
    effect    = "Allow"
    actions   = ["secretsmanager:GetSecretValue"]
    resources = [aws_secretsmanager_secret.exp_app.arn]
  }
}

resource "aws_iam_role_policy" "loadgen_secret" {
  name   = "read-exp-app-secret"
  role   = aws_iam_role.loadgen.id
  policy = data.aws_iam_policy_document.loadgen_secret.json
}

resource "aws_iam_instance_profile" "loadgen" {
  name = "${var.name_prefix}-loadgen"
  role = aws_iam_role.loadgen.name
}

resource "aws_instance" "loadgen" {
  ami                         = data.aws_ssm_parameter.al2023.value
  instance_type               = local.loadgen_instance_type
  subnet_id                   = local.loadgen_subnet_id
  vpc_security_group_ids      = [aws_security_group.ec2.id]
  iam_instance_profile        = aws_iam_instance_profile.loadgen.name
  key_name                    = var.key_name
  associate_public_ip_address = var.associate_public_ip

  metadata_options {
    http_endpoint = "enabled"
    http_tokens   = "required"
  }

  tags = {
    Name       = "${var.name_prefix}-loadgen"
    experiment = local.experiment
    role       = "loadgen"
  }

  lifecycle {
    precondition {
      condition     = local.loadgen_subnet_id != null
      error_message = "Loadgen subnet was not created in availability zone ${coalesce(local.chosen_az, "(unset)")}. Both RDS twins and the loadgen must share that AZ."
    }
  }
}
