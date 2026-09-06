# uuid-insert experiment — Terraform

Apply-ready twin RDS + loadgen stack for the Go harness in this repo. Same-day apply / measure / **destroy-on-success**.

Password happy path: **`MYSQL_SECRET_ARN`** (JSON `username` / `password`). Terraform never emits `mysql_password` or `rds_endpoint_*` aliases. Map outputs with [`scripts/export_env_from_tf.sh`](../scripts/export_env_from_tf.sh).

This stack **creates its own VPC**. You do not need a pre-existing VPC or subnets.

## What you get

| Resource | Locked BOM |
| --- | --- |
| 2× RDS MySQL 8.0 | `db.r6g.large`, engine **`8.0.46`** (exact minor; current RDS MySQL 8.0.x per AWS docs / eu-central-1), Single-AZ, **same AZ**, identical except identifier + `arm=v4\|v7` tags |
| Shared parameter group | `innodb_buffer_pool_size` = `12884901888` (12 GiB); `innodb_io_capacity=5000`, `innodb_io_capacity_max=12000`; durable pair `innodb_flush_log_at_trx_commit=1` + `sync_binlog=1`; `innodb_redo_log_capacity=2147483648`; buffer-pool dump/load at shutdown/startup = 0; `innodb_flush_neighbors=0`; `innodb_adaptive_hash_index=0`. **`innodb_doublewrite` is not set** — remains default **ON**. |
| Storage | gp3, `allocated_storage=400`, `iops=12000`, `storage_throughput=500` each |
| RDS flags | `multi_az=false`, `publicly_accessible=false`, `backup_retention_period=0`, `skip_final_snapshot=true`, `deletion_protection=false`, `performance_insights_enabled=true`, `monitoring_interval=60` |
| 1× EC2 loadgen | `c6i.large`, Amazon Linux 2023, **same AZ** (and the public subnet in that AZ) |
| Database | name `payments_exp`, master/app user `exp_app` |
| Secret | Secrets Manager JSON `{username,password}` for `exp_app`; loadgen instance role can `GetSecretValue` |
| Network | Dedicated VPC (default CIDR `10.42.0.0/16`) with **two public subnets in different AZs**, Internet Gateway, and a public route table (`0.0.0.0/0` → IGW) associated to both subnets. DNS support + DNS hostnames are enabled (required for the Secrets Manager interface endpoint private DNS). `sg_ec2` → `sg_rds` on **3306 only**. SSH to EC2 only from `allowed_ssh_cidr` (public subnet + `associate_public_ip=true`). Always-on Secrets Manager **interface** VPC endpoint (`private_dns_enabled=true`, 443 from `sg_ec2`) so `GetSecretValue` works without NAT. yum/dnf uses the public subnet + IGW. |

RDS DB subnet groups still need **two subnets in different AZs**. Both instances and the loadgen are pinned to one AZ via `availability_zone` (default: first AZ in `aws_region`).

`innodb_doublewrite` is left at the RDS/MySQL default (**ON**); do not disable it. Engine is pinned to **8.0.46** (widely available RDS MySQL 8.0.x as of 2026; `auto_minor_version_upgrade=false` so the twins stay identical). After 2026-07-31, MySQL 8.0 create uses RDS Extended Support by default.

### gp3 size floor (MySQL IOPS / throughput)

AWS does **not** allow provisioned IOPS or throughput on RDS MySQL gp3 unless `allocated_storage` is **≥ 400 GiB**. Below that floor, `CreateDBInstance` fails if `iops` / `storage_throughput` are set (the previous `120` GiB BOM hit this). IOPS (`12000`) and throughput (`500` MiB/s) stay as locked; only capacity moves to `400`.

**Cost (storage capacity only, ~$0.115/GB-mo class in us-east-1 / eu-central-1):** two volumes go from 2×120 = 240 GB (~$28/mo) to 2×400 = 800 GB (~$92/mo) — about **+$64/mo** or **+$2.1/day** while both exist. Provisioned IOPS / throughput charges are unchanged. A same-day 8–16 h experiment is roughly **+$0.70–$1.40** extra on capacity vs 120 GB. Destroy when measure finishes.

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
# set allowed_ssh_cidr; optional key_name / availability_zone / vpc_cidr
make apply
```

Same-day apply needs **`allowed_ssh_cidr`** (and usually `key_name` if you will SSH). The VPC, public subnets, IGW, and route table are created for you. Default `aws_region` is **`eu-central-1`** (Frankfurt).

`make apply` runs `terraform init` then `terraform apply`. Confirm the plan; two `db.r6g.large` plus a `c6i.large` are not cheap — do not leave them overnight.

### 2. Export env (harness contract)

From the **repo root**, append locked outputs to `.env` (no plaintext password):

```bash
terraform -chdir=infra output -json | ./scripts/export_env_from_tf.sh >> .env
```

Or: `./scripts/export_env_from_tf.sh infra >> .env`

That sets `MYSQL_HOST_V4` / `MYSQL_HOST_V7`, `MYSQL_PORT`, `MYSQL_DATABASE`, `MYSQL_USER`, **`MYSQL_SECRET_ARN`**, `MYSQL_TLS=false`, `AWS_REGION`, `LOADGEN_INSTANCE_HINT`. Infra-only ids (`loadgen_subnet_id`, `loadgen_sg_id`, `rds_sg_id`) are skipped.

`LOADGEN_AZ` is an ops note (unused by Go). Set it to the AZ you chose (`availability_zone`, or the first AZ in the region) — it must match both RDS twins.

Copy `.env` onto the loadgen (or export the same variables there). The instance role can read the secret; you do not need `MYSQL_PASSWORD` on the real path.

### 3. Harness preload / measure

SSH to the loadgen from `allowed_ssh_cidr`. The loadgen sits on a **public** subnet with `associate_public_ip=true` by default, so a public IPv4 is assigned for SSH. You still need `key_name` (or another access path). Install Go 1.22+, clone this repo, then from the repo root:

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

Tear down as soon as results are copied off the instance. Settings are ephemeral on purpose (`skip_final_snapshot`, `deletion_protection=false`, `backup_retention_period=0`, secret `recovery_window_in_days=0`). Destroy also removes the dedicated VPC and its public subnets / IGW.

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
| `allowed_ssh_cidr` | yes | e.g. `203.0.113.10/32` |
| `aws_region` | no | default **`eu-central-1`** (Frankfurt) |
| `vpc_cidr` | no | default `10.42.0.0/16`; stack always creates this VPC |
| `availability_zone` | no | default first AZ in the region; loadgen + both RDS twins pin here. A second public subnet is still created in another AZ for the DB subnet group |
| `key_name` | no | existing key pair |
| `associate_public_ip` | no | default `true` (needed for SSH on the public subnet) |
| `name_prefix` | no | default `payments-exp` |

There is **no** `vpc_id` / `subnet_ids` input. BOM values (instance class, storage, user, db name, buffer pool) are hardcoded, not variables.

## Tags

`experiment=uuid-insert` on everything (provider `default_tags`). RDS twins also get `arm=v4` or `arm=v7`. Loadgen is tagged `role=loadgen`.
