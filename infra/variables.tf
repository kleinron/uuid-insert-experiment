variable "aws_region" {
  type        = string
  description = "AWS region for all resources and the aws_region output (harness AWS_REGION)."
  default     = "us-east-1"
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

variable "vpc_id" {
  type        = string
  description = "Existing VPC that will hold both RDS twins and the loadgen instance."
}

variable "subnet_ids" {
  type        = list(string)
  description = "Existing subnet IDs for the RDS DB subnet group. AWS requires at least two subnets in different AZs. One subnet in the chosen AZ is also used for the loadgen EC2."

  validation {
    condition     = length(var.subnet_ids) >= 2
    error_message = "Provide at least two subnet IDs. RDS DB subnet groups require subnets in at least two AZs even for Single-AZ instances."
  }
}

variable "availability_zone" {
  type        = string
  default     = null
  nullable    = true
  description = "AZ for both RDS instances and the loadgen EC2. Defaults to the AZ of subnet_ids[0]. That subnet list must include a subnet in this AZ."
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
  description = "Associate a public IP with the loadgen instance so SSH from allowed_ssh_cidr works on a public subnet. Set false for private subnets."
}
