TF ?= terraform

.PHONY: init apply destroy destroy-auto fmt validate output

init:
	$(TF) init

apply: init
	$(TF) apply

destroy: init
	$(TF) destroy

# Same-day teardown. Instances are ephemeral: skip_final_snapshot, no deletion
# protection, backup_retention_period=0, secret recovery_window_in_days=0.
destroy-auto: init
	$(TF) destroy -auto-approve

fmt:
	$(TF) fmt -recursive

validate: init
	$(TF) validate

output:
	$(TF) output -json
