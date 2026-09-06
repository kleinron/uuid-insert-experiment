# uuid-insert experiment — Terraform

Apply-ready twin RDS + loadgen stack for the Go harness in this repo. Same-day apply / measure / **destroy-on-success**.

Password happy path: **`MYSQL_SECRET_ARN`** (JSON `username` / `password`). Terraform never emits `mysql_password` or `rds_endpoint_*` aliases. Map outputs with [`scripts/export_env_from_tf.sh`](../scripts/export_env_from_tf.sh).

## What you get

| Resource | Locked BOM |
| --- | --- |
| 2× RDS MySQL 8.0 | `db.r6g.large`, engine **`8.0.46`** (exact minor; current RDS MySQL 8.0.x per AWS docs / us-east-1), Single-AZ, **same AZ**, identical except identifier + `arm=v4\|v7` tags |
| Shared parameter group | `innodb_buffer_pool_size` = `12884901888` (12 GiB); `innodb_io_capacity=5000`, `innodb_io_capacity_max=12000`; durable pair `innodb_flush_log_at_trx_commit=1` + `sync_binlog=1`; `innodb_redo_log_capacity=2147483648`; buffer-pool dump/load at shutdown/startup = 0; `innodb_flush_neighbors=0`; `innodb_adaptive_hash_index=0`. **`innodb_doublewrite` is not set** — remains default **ON**. |
| Storage | gp3, `allocated_storage=120`, `iops=12000`, `storage_throughput=500` each |
| RDS flags | `multi_az=false`, `publicly_accessible=false`, `backup_retention_period=0`, `skip_final_snapshot=true`, `deletion_protection=false`, `performance_insights_enabled=true`, `monitoring_interval=60` |
| 1× EC2 loadgen | `c6i.large`, Amazon Linux 2023, **same AZ** (and a subnet in that AZ) |
| Database | name `payments_exp`, master/app user `exp_app` |
| Secret | Secrets Manager JSON `{username,password}` for `exp_app`; loadgen instance role can `GetSecretValue` |
| Network | Existing VPC (`vpc_id` + `subnet_ids`). `sg_ec2` → `sg_rds` on **3306 only**. SSH to EC2 only from `allowed_ssh_cidr`. Always-on Secrets Manager **interface** VPC endpoint (`private_dns_enabled=true`, 443 from `sg_ec2`) so a private loadgen can `GetSecretValue` without NAT. yum/dnf still needs NAT or a repo mirror. |

RDS DB subnet groups still need **two subnets in different AZs**. Both instances are pinned to one AZ via `availability_zone` (default: AZ of `subnet_ids[0]`).

`innodb_doublewrite` is left at the RDS/MySQL default (**ON**); do not disable it. Engine is pinned to **8.0.46** (widely available RDS MySQL 8.0.x as of 2026; `auto_minor_version_upgrade=false` so the twins stay identical). After 2026-07-31, MySQL 8.0 create uses RDS Extended Support by default.

## Temporary IAM user

Dedicated short-lived IAM user for `terraform apply` / `destroy` of this stack: [IAM-TEMP-USER.md](IAM-TEMP-USER.md).

## Same-day path

```
apply → export env → harness (config-check → schema → preload → warmup → measure) → destroy
```

### 1. Apply

```bash
cd infra
cp terraform.tfvars.example terraform.tfvars
# set vpc_id, subnet_ids, allowed_ssh_cidr, optional key_name / availability_zone
make apply
```

`make apply` runs `terraform init` then `terraform apply`. Confirm the plan; two `db.r6g.large` plus a `c6i.large` are not cheap — do not leave them overnight.

### 2. Export env (harness contract)

From the **repo root**, append locked outputs to `.env` (no plaintext password):

```bash
terraform -chdir=infra output -json | ./scripts/export_env_from_tf.sh >> .env
```

Or: `./scripts/export_env_from_tf.sh infra >> .env`

That sets `MYSQL_HOST_V4` / `MYSQL_HOST_V7`, `MYSQL_PORT`, `MYSQL_DATABASE`, `MYSQL_USER`, **`MYSQL_SECRET_ARN`**, `MYSQL_TLS=false`, `AWS_REGION`, `LOADGEN_INSTANCE_HINT`. Infra-only ids (`loadgen_subnet_id`, `loadgen_sg_id`, `rds_sg_id`) are skipped.

`LOADGEN_AZ` is an ops note (unused by Go). Set it to the AZ you chose (`availability_zone`, or the AZ of `subnet_ids[0]`) — it must match both RDS twins.

Copy `.env` onto the loadgen (or export the same variables there). The instance role can read the secret; you do not need `MYSQL_PASSWORD` on the real path.

### 3. Harness preload / measure

SSH to the loadgen from `allowed_ssh_cidr` (needs `key_name` and usually a public subnet / `associate_public_ip=true`). Install Go 1.22+, clone this repo, then from the repo root:

```bash
make config-check
make schema BOTH=1
make preload BOTH=1
make tablespace-report BOTH=1
EXPERIMENT_ARM=v4 make run-arm
EXPERIMENT_ARM=v7 make run-arm
make export-metrics
```

See the root [README](../README.md) and [DESIGN.md](../DESIGN.md). After preload, if measured tablespace ≥ 50 GiB, revisit instance class / buffer pool before measure.

### 4. Destroy on success

Tear down as soon as results are copied off the instance. Settings are ephemeral on purpose (`skip_final_snapshot`, `deletion_protection=false`, `backup_retention_period=0`, secret `recovery_window_in_days=0`).

```bash
cd infra
make destroy          # interactive
make destroy-auto     # terraform destroy -auto-approve
```

State is local (`*.tfstate` is gitignored) and contains the generated DB password — do not commit it.

## Makefile

| Target | Action |
| --- | --- |
| `apply` | `terraform init` + `terraform apply` |
| `destroy` | `terraform destroy` |
| `destroy-auto` | `terraform destroy -auto-approve` |
| `fmt` / `validate` / `output` | helpers |

## Outputs (exact names)

`mysql_host_v4`, `mysql_host_v7`, `mysql_port`, `mysql_database`, `mysql_user`, `mysql_secret_arn`, `mysql_tls`, `aws_region`, `loadgen_instance_id`, `loadgen_subnet_id`, `loadgen_sg_id`, `rds_sg_id`

## Variables

| Name | Required | Notes |
| --- | --- | --- |
| `vpc_id` | yes | Existing VPC |
| `subnet_ids` | yes | ≥ 2, different AZs; one must be in the chosen AZ |
| `allowed_ssh_cidr` | yes | e.g. `203.0.113.10/32` |
| `aws_region` | no | default `us-east-1` |
| `availability_zone` | no | default AZ of `subnet_ids[0]` |
| `key_name` | no | existing key pair |
| `associate_public_ip` | no | default `true` |
| `name_prefix` | no | default `payments-exp` |

BOM values (instance class, storage, user, db name, buffer pool) are hardcoded, not variables.

## Tags

`experiment=uuid-insert` on everything (provider `default_tags`). RDS twins also get `arm=v4` or `arm=v7`. Loadgen is tagged `role=loadgen`.
