variable "aws_region" {
  type        = string
  description = "AWS region for all resources and the aws_region output (harness AWS_REGION)."
  default     = "eu-central-1"
}

variable "name_prefix" {
  type        = string
  description = "Lowercase prefix for resource identifiers (RDS ids become {prefix}-v4 / {prefix}-v7)."
  default     = "payments-exp"

  validation {
    condition     = can(regex("^[a-z][a-z0-9-]{0,36}$", var.name_prefix))
    error_message = "name_prefix must be a lowercase RDS-safe prefix (start with a letter; letters, digits, hyphens; max 37 chars)."
  }
}

variable "vpc_cidr" {
  type        = string
  description = "CIDR for the dedicated experiment VPC this stack always creates (no BYO VPC)."
  default     = "10.42.0.0/16"

  validation {
    condition     = can(cidrnetmask(var.vpc_cidr)) && tonumber(split("/", var.vpc_cidr)[1]) <= 16
    error_message = "vpc_cidr must be a valid IPv4 CIDR of /16 or larger (default 10.42.0.0/16) so two /24 public subnets fit."
  }
}

variable "availability_zone" {
  type        = string
  default     = null
  nullable    = true
  description = "AZ for both RDS instances and the loadgen EC2. Defaults to the first AZ returned in aws_region. A second public subnet is still created in another AZ for the RDS DB subnet group."
}

variable "allowed_ssh_cidr" {
  type        = string
  description = "IPv4 CIDR allowed to SSH (tcp/22) to the loadgen security group. Use a /32 for a single operator IP."

  validation {
    condition     = can(cidrhost(var.allowed_ssh_cidr, 0))
    error_message = "allowed_ssh_cidr must be a valid IPv4 CIDR (for example 203.0.113.10/32)."
  }
}

variable "key_name" {
  type        = string
  default     = null
  nullable    = true
  description = "Existing EC2 key pair name for SSH to the loadgen instance. Leave null if you will not SSH (or will use another access path)."
}

variable "associate_public_ip" {
  type        = bool
  default     = true
  description = "Associate a public IP with the loadgen instance so SSH from allowed_ssh_cidr works on the public subnet (default true)."
}
