provider "aws" {
  region = var.aws_region

  default_tags {
    tags = {
      experiment = local.experiment
      ManagedBy  = "terraform"
    }
  }
}
