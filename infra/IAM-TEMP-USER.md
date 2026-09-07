# Temporary IAM user for `/infra` apply / destroy

Dedicated **short-lived** IAM user that can `terraform apply` and `terraform destroy` this locked stack, then be deleted. It is **not** the loadgen instance role.

Replace `ACCOUNT_ID` and `REGION` (stack default `eu-central-1`). Resource names assume `name_prefix` default `payments-exp`. If you change the prefix, replace that string throughout the policy.

This stack does **not** create CloudWatch Logs groups or `enabled_cloudwatch_logs_exports`. Do not add `logs:*`. CloudWatch **metrics** and Performance Insights reads are a separate post-apply need (measure-window pulls); those go on `CloudWatchAndPiRead` below.

The module **always creates** a dedicated VPC (default `10.42.0.0/16`), two public subnets, an Internet Gateway, and a public route table. There is no BYO-VPC path.

## Who creates what

| Stays on the **account owner** (pre-existing) | This user **creates** (and must be able to delete) |
| --- | --- |
| Optional EC2 key pair (`key_name`) | Dedicated VPC + 2 public subnets + IGW + public route table / `0.0.0.0/0` route / associations |
| RDS service-linked role `AWSServiceRoleForRDS` if the account has never used RDS | 2× RDS (`payments-exp-v4`, `payments-exp-v7`) |
| | 1× EC2 loadgen (`payments-exp-loadgen`) |
| | 3 security groups (`-sg-ec2`, `-sg-rds`, `-sg-sm-endpoint`) + rules |
| | Secrets Manager secret `payments-exp/exp_app` |
| | Secrets Manager **interface** VPC endpoint |
| | IAM roles `payments-exp-loadgen` + `payments-exp-rds-monitoring`, instance profile, inline `PutRolePolicy` |
| | DB parameter group `payments-exp-mysql80`, DB subnet group `payments-exp` |

## Policy (customer-managed)

Create a customer-managed policy (e.g. `payments-exp-tf-apply-destroy`) with this document.

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "ReadDiscovery",
      "Effect": "Allow",
      "Action": [
        "ec2:Describe*",
        "rds:Describe*",
        "rds:ListTagsForResource",
        "iam:GetRole",
        "iam:GetRolePolicy",
        "iam:GetInstanceProfile",
        "iam:GetPolicy",
        "iam:GetPolicyVersion",
        "iam:ListRoles",
        "iam:ListRolePolicies",
        "iam:ListAttachedRolePolicies",
        "iam:ListInstanceProfiles",
        "iam:ListInstanceProfilesForRole",
        "iam:ListRoleTags",
        "iam:ListInstanceProfileTags",
        "secretsmanager:DescribeSecret",
        "secretsmanager:ListSecrets",
        "secretsmanager:ListSecretVersionIds",
        "secretsmanager:GetResourcePolicy",
        "ssm:GetParameter",
        "ssm:GetParameters",
        "sts:GetCallerIdentity"
      ],
      "Resource": "*"
    },
    {
      "Sid": "CloudWatchAndPiRead",
      "Effect": "Allow",
      "Action": [
        "cloudwatch:GetMetricStatistics",
        "cloudwatch:GetMetricData",
        "cloudwatch:ListMetrics",
        "pi:GetResourceMetrics",
        "pi:DescribeDimensionKeys",
        "pi:ListAvailableResourceMetrics"
      ],
      "Resource": "*"
    },
    {
      "Sid": "Ec2MutateInRegion",
      "Effect": "Allow",
      "Action": [
        "ec2:CreateVpc",
        "ec2:DeleteVpc",
        "ec2:ModifyVpcAttribute",
        "ec2:CreateSubnet",
        "ec2:DeleteSubnet",
        "ec2:ModifySubnetAttribute",
        "ec2:CreateInternetGateway",
        "ec2:AttachInternetGateway",
        "ec2:DetachInternetGateway",
        "ec2:DeleteInternetGateway",
        "ec2:CreateRouteTable",
        "ec2:DeleteRouteTable",
        "ec2:CreateRoute",
        "ec2:DeleteRoute",
        "ec2:AssociateRouteTable",
        "ec2:DisassociateRouteTable",
        "ec2:CreateSecurityGroup",
        "ec2:DeleteSecurityGroup",
        "ec2:AuthorizeSecurityGroupIngress",
        "ec2:AuthorizeSecurityGroupEgress",
        "ec2:RevokeSecurityGroupIngress",
        "ec2:RevokeSecurityGroupEgress",
        "ec2:ModifySecurityGroupRules",
        "ec2:CreateVpcEndpoint",
        "ec2:DeleteVpcEndpoints",
        "ec2:ModifyVpcEndpoint",
        "ec2:CreateNetworkInterface",
        "ec2:DeleteNetworkInterface",
        "ec2:ModifyNetworkInterfaceAttribute",
        "ec2:AttachNetworkInterface",
        "ec2:DetachNetworkInterface",
        "ec2:CreateTags",
        "ec2:DeleteTags",
        "ec2:RunInstances",
        "ec2:TerminateInstances",
        "ec2:ModifyInstanceAttribute"
      ],
      "Resource": "*",
      "Condition": {
        "StringEquals": {
          "aws:RequestedRegion": "REGION"
        }
      }
    },
    {
      "Sid": "RdsMutatePrefixed",
      "Effect": "Allow",
      "Action": [
        "rds:CreateDBInstance",
        "rds:DeleteDBInstance",
        "rds:ModifyDBInstance",
        "rds:CreateDBParameterGroup",
        "rds:DeleteDBParameterGroup",
        "rds:ModifyDBParameterGroup",
        "rds:CreateDBSubnetGroup",
        "rds:DeleteDBSubnetGroup",
        "rds:ModifyDBSubnetGroup",
        "rds:AddTagsToResource",
        "rds:RemoveTagsFromResource"
      ],
      "Resource": [
        "arn:aws:rds:REGION:ACCOUNT_ID:db:payments-exp-*",
        "arn:aws:rds:REGION:ACCOUNT_ID:pg:payments-exp-*",
        "arn:aws:rds:REGION:ACCOUNT_ID:subgrp:payments-exp",
        "arn:aws:rds:REGION:ACCOUNT_ID:subgrp:payments-exp-*",
        "arn:aws:rds:REGION:ACCOUNT_ID:og:default:mysql-8-0"
      ]
    },
    {
      "Sid": "SecretsManagerPrefixed",
      "Effect": "Allow",
      "Action": [
        "secretsmanager:CreateSecret",
        "secretsmanager:DeleteSecret",
        "secretsmanager:DescribeSecret",
        "secretsmanager:GetSecretValue",
        "secretsmanager:PutSecretValue",
        "secretsmanager:UpdateSecret",
        "secretsmanager:TagResource",
        "secretsmanager:UntagResource",
        "secretsmanager:GetResourcePolicy"
      ],
      "Resource": "arn:aws:secretsmanager:REGION:ACCOUNT_ID:secret:payments-exp/*"
    },
    {
      "Sid": "IamRolesAndProfilesPrefixed",
      "Effect": "Allow",
      "Action": [
        "iam:CreateRole",
        "iam:DeleteRole",
        "iam:UpdateAssumeRolePolicy",
        "iam:TagRole",
        "iam:UntagRole",
        "iam:PutRolePolicy",
        "iam:DeleteRolePolicy",
        "iam:CreateInstanceProfile",
        "iam:DeleteInstanceProfile",
        "iam:AddRoleToInstanceProfile",
        "iam:RemoveRoleFromInstanceProfile",
        "iam:TagInstanceProfile",
        "iam:UntagInstanceProfile"
      ],
      "Resource": [
        "arn:aws:iam::ACCOUNT_ID:role/payments-exp-*",
        "arn:aws:iam::ACCOUNT_ID:instance-profile/payments-exp-*"
      ]
    },
    {
      "Sid": "PassRoleLoadgenToEc2",
      "Effect": "Allow",
      "Action": "iam:PassRole",
      "Resource": "arn:aws:iam::ACCOUNT_ID:role/payments-exp-loadgen",
      "Condition": {
        "StringEquals": {
          "iam:PassedToService": "ec2.amazonaws.com"
        }
      }
    },
    {
      "Sid": "PassRoleMonitoringToRds",
      "Effect": "Allow",
      "Action": "iam:PassRole",
      "Resource": "arn:aws:iam::ACCOUNT_ID:role/payments-exp-rds-monitoring",
      "Condition": {
        "StringEquals": {
          "iam:PassedToService": "monitoring.rds.amazonaws.com"
        }
      }
    },
    {
      "Sid": "AttachEnhancedMonitoringToRdsRole",
      "Effect": "Allow",
      "Action": [
        "iam:AttachRolePolicy",
        "iam:DetachRolePolicy"
      ],
      "Resource": "arn:aws:iam::ACCOUNT_ID:role/payments-exp-rds-monitoring",
      "Condition": {
        "ArnEquals": {
          "iam:PolicyARN": "arn:aws:iam::aws:policy/service-role/AmazonRDSEnhancedMonitoringRole"
        }
      }
    }
  ]
}
```

`CreateDBInstance` also evaluates the default MySQL 8.0 option group (`og:default:mysql-8-0`); this stack does not create a custom option group. The DB subnet group name is exactly `payments-exp` (no suffix), so both the exact `subgrp` ARN and `payments-exp-*` are listed.

`rds:DescribeDBInstances` is already covered by `rds:Describe*` in `ReadDiscovery`. If that wildcard is dropped, add `rds:DescribeDBInstances` explicitly — it is needed to resolve instance identifiers / `DbiResourceId` before a CloudWatch / PI measure-window pull.

VPC networking actions (`CreateVpc`, `CreateSubnet`, `CreateInternetGateway` / attach / detach / delete, route table create / delete / associate / disassociate, `CreateRoute` / `DeleteRoute`, `ModifyVpcAttribute`, plus `CreateTags` / `DeleteTags`) live on the region-scoped EC2 mutate statement. `ModifySubnetAttribute` is included so Terraform can set `map_public_ip_on_launch` on the public subnets. Least privilege is otherwise unchanged for the locked BOM (prefixed RDS / Secrets Manager / IAM).

## Console steps

1. **Create user** — IAM → Users → Create user. Name e.g. `payments-exp-tf-temp`. **No** console password (CLI only).
2. **Attach the customer-managed policy** from above (Permissions → Attach policies directly). Do not attach `AdministratorAccess`.
3. **Create access key** — user → Security credentials → Create access key → **Command Line Interface**.
4. **Hand off** the `AKIA…` key and secret over a **secure channel** (password manager, encrypted secret). **Never** paste them into chat, email, GitHub, or Terraform files.
5. On the operator machine that will hold local `*.tfstate`, export `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` (and `AWS_REGION=eu-central-1`) and run `make apply` / later `make destroy` as in [README.md](README.md).
6. **After destroy succeeds:** delete the access key → delete the user → delete the customer-managed policy.

Do not delete the user while the stack (or a failed apply) is still up.

## Gotchas

- **Short-lived.** Create the user for the same-day apply / measure / destroy window. Delete key, user, and policy as soon as destroy completes.
- **EC2 `Resource: "*"`** on the mutate statement is **intentional**. VPC, subnets, IGW, route tables, security groups, interface VPC endpoints, ENIs, `CreateTags`, and `RunInstances` / `TerminateInstances` do not usefully constrain to a `payments-exp-*` name prefix. The region condition is the bound.
- **Secret `recovery_window_in_days = 0`.** Destroy force-deletes `payments-exp/exp_app` with no recovery window so the same name can be re-applied the same day. There is no undelete.
- **RDS service-linked role (first-time RDS).** If the account has never created an RDS instance, `AWSServiceRoleForRDS` is missing. The account owner must let AWS create that SLR (or create it once). This user is not granted `iam:CreateServiceLinkedRole`.
- **CloudWatch / PI after apply.** Without `CloudWatchAndPiRead`, post-run measure-window metric pulls fail with `AccessDenied` on `cloudwatch:GetMetricStatistics` / `pi:GetResourceMetrics`. This is metrics + Performance Insights, not CloudWatch Logs (`logs:*` stays omitted).
