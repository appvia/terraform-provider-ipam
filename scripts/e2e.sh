#!/usr/bin/env bash
# Copyright 2026 Appvia Ltd.
# SPDX-License-Identifier: MPL-2.0

# Cross-repo end-to-end test: the real `ipam` CLI (built from ../ipam) and the
# real provider (built here, loaded by Terraform through dev_overrides) share
# one store per backend: local, Azurite (azblob) and the S3 emulator (MinIO).
#
# Environment:
#   IPAM_REPO                 client repo (default: ../ipam next to this repo)
#   IPAM_E2E_BACKENDS         space-separated subset of "local azblob s3" (default: all)
#   IPAM_E2E_FORCE_OVERLAP=1  local only: write an overlapping allocation straight
#                             into the reservations file and require the CLI to reject it
#   IPAM_EMULATOR_STATE_DIR   emulator state dir (default: a fresh temp dir)
#   IPAM_AZURITE_PORT, IPAM_S3_PORT, IPAM_S3_CONSOLE_PORT (default 10620, 9620, 9621)
#   IPAM_E2E_KEEP=1           keep the work dir for inspection
#   IPAM_TEST_S3_BUCKET (+IPAM_TEST_S3_REGION) / AZURE_STORAGE_ACCOUNT_URL (+AZURE_STORAGE_CONTAINER)
#                             also run the flow against a real bucket/container
#                             with ambient AWS / ARM_* credentials
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/.." && pwd)"
ipam_repo="$(cd "${IPAM_REPO:-$repo_root/../ipam}" && pwd)"
aws_module="$(cd "${IPAM_AWS_MODULE:-$repo_root/../terraform-aws-ipam}" && pwd)"
backends="${IPAM_E2E_BACKENDS:-local azblob s3}"

export IPAM_AZURITE_PORT="${IPAM_AZURITE_PORT:-10620}"
export IPAM_S3_PORT="${IPAM_S3_PORT:-9620}"
export IPAM_S3_CONSOLE_PORT="${IPAM_S3_CONSOLE_PORT:-9621}"

tmp_root="${TMPDIR:-/tmp}"
work="$(mktemp -d "${tmp_root%/}/ipam-e2e.XXXXXX")"
export IPAM_EMULATOR_STATE_DIR="${IPAM_EMULATOR_STATE_DIR:-$work/emulators}"
mkdir -p "$IPAM_EMULATOR_STATE_DIR" "$work/bin"

bin="$work/bin"
current="setup"
emulators_started=0
bg_pids=()

log() { printf '[e2e %s] %s\n' "$(date +%H:%M:%S)" "$*"; }

reported=0
last_error=""

fail() {
	reported=1
	printf 'E2E FAIL %s: %s\n' "$current" "$*" >&2
	exit 1
}

# bounded SECONDS CMD... runs CMD with a hard deadline (SIGALRM kills it).
bounded() {
	local secs=$1
	shift
	perl -e 'alarm shift; exec @ARGV or die "exec: $!"' "$secs" "$@"
}

kill_tree() {
	local child

	for child in $(pgrep -P "$1" 2>/dev/null || true); do
		kill_tree "$child"
	done

	kill "$1" 2>/dev/null || true
}

cleanup() {
	local rc=$? pid

	if ((rc != 0 && !reported)); then
		printf 'E2E FAIL %s: aborted (exit %s) at %s\n' "$current" "$rc" "${last_error:-an unchecked command}" >&2
	fi

	for pid in "${bg_pids[@]+"${bg_pids[@]}"}"; do
		kill_tree "$pid"
	done

	if ((emulators_started)); then
		if ! bash "$ipam_repo/scripts/emulators/stop.sh"; then
			printf 'E2E FAIL cleanup: emulator stop.sh failed\n' >&2
			rc=1
		fi
	fi

	if [[ "${IPAM_E2E_KEEP:-}" == 1 ]]; then
		log "work dir kept: $work"
	else
		rm -rf "$work"
	fi

	exit "$rc"
}
set -E
trap 'last_error="line $LINENO: $BASH_COMMAND"' ERR
trap cleanup EXIT
trap 'exit 130' INT TERM


log "building provider, CLI and helpers"
(cd "$repo_root" && go build -o "$bin/terraform-provider-ipam" .)
(cd "$ipam_repo" && go build -o "$bin/ipam" ./cmd/ipam)
(cd "$repo_root" && go build -o "$bin/overlapcheck" ./scripts/overlapcheck)
(cd "$repo_root/scripts/e2e/tools" && go build -o "$bin/e2etools" .)
"$bin/ipam" version

cat >"$work/dev.tfrc" <<EOF
provider_installation {
  dev_overrides {
    "appvia/ipam" = "$bin"
  }
  direct {}
}
EOF
export TF_CLI_CONFIG_FILE="$work/dev.tfrc"
export TF_IN_AUTOMATION=1
export CHECKPOINT_DISABLE=1


# Set per backend: provider_body (HCL inside provider "ipam"), backend_url (CLI).
provider_body=""
backend_url=""
flow_dir=""

cli() { IPAM_BACKEND="$backend_url" bounded 120 "$bin/ipam" "$@"; }

# expect_rc WANT PATTERN CMD... runs CMD and requires exit WANT and, when
# PATTERN is non-empty, output matching the extended regex PATTERN.
expect_rc() {
	local want=$1 pattern=$2 out rc=0
	shift 2

	out="$("$@" 2>&1)" || rc=$?
	printf '%s\n' "$out" | sed 's/^/    | /'

	[[ "$rc" == "$want" ]] || fail "'$*' exited $rc, want $want"

	if [[ -n "$pattern" ]] && ! grep -Eqi -- "$pattern" <<<"$out"; then
		fail "'$*' output does not match /$pattern/"
	fi
}

# tf_dir NAME KIND writes a Terraform working dir for the current backend.
tf_dir() {
	local dir="$flow_dir/tf/$1" kind=$2

	mkdir -p "$dir"
	cat >"$dir/provider.tf" <<EOF
terraform {
  required_providers {
    ipam = {
      source = "appvia/ipam"
    }
  }
}

provider "ipam" {
$provider_body
  access_propagation_timeout = "20s"
}
EOF

	case "$kind" in
	platform)
		cat >"$dir/main.tf" <<'EOF'
variable "root_pool" {
  type = string
}

variable "pool" {
  type = string
}

resource "ipam_pool" "apps" {
  id            = var.pool
  parent_pool   = var.root_pool
  prefix_length = 18
  description   = "e2e platform-owned child pool"
  labels        = { owner = "platform" }
}

output "cidrs" {
  value = ipam_pool.apps.cidrs
}
EOF
		;;
	count)
		cat >"$dir/main.tf" <<'EOF'
variable "pool" {
  type = string
}

variable "n" {
  type = number
}

variable "prefix_length" {
  type    = number
  default = 24
}

data "ipam_pool" "this" {
  id = var.pool
}

resource "ipam_allocation" "this" {
  count         = var.n
  pool          = data.ipam_pool.this.id
  prefix_length = var.prefix_length
  description   = "e2e requestor ${count.index}"
}

output "ids" {
  value = ipam_allocation.this[*].id
}

output "cidrs" {
  value = ipam_allocation.this[*].cidr
}
EOF
		;;
	foreach)
		cat >"$dir/main.tf" <<'EOF'
variable "pool" {
  type = string
}

resource "ipam_allocation" "this" {
  for_each      = toset(["blue", "green", "red"])
  pool          = var.pool
  prefix_length = 24
  labels        = { colour = each.key }
}

output "ids" {
  value = [for a in ipam_allocation.this : a.id]
}
EOF
		;;
	explicit)
		cat >"$dir/main.tf" <<'EOF'
variable "pool" {
  type = string
}

variable "cidr" {
  type = string
}

resource "ipam_allocation" "this" {
  pool = var.pool
  cidr = var.cidr
}
EOF
		;;
	import)
		cat >"$dir/main.tf" <<'EOF'
variable "pool" {
  type = string
}

variable "id" {
  type = string
}

import {
  to = ipam_allocation.this
  id = "${var.pool}/${var.id}"
}

resource "ipam_allocation" "this" {
  pool          = var.pool
  prefix_length = 24
}

output "id" {
  value = ipam_allocation.this.id
}
EOF
		;;
	*) fail "unknown tf kind $kind" ;;
	esac

	: >"$dir/terraform.tfvars"
	printf '%s\n' "$dir"
}

tfvar() { printf '%s = %s\n' "$2" "$3" >>"$1/terraform.tfvars"; }

# tf DIR ARGS... runs terraform in DIR with a deadline, logging to DIR/tf.log.
tf() {
	local dir=$1
	shift

	(cd "$dir" && bounded 300 terraform "$@" -no-color) >>"$dir/tf.log" 2>&1
}

tf_ok() {
	local dir=$1
	shift

	tf "$dir" "$@" || {
		tail -n 40 "$dir/tf.log" >&2
		fail "terraform $* in $(basename "$dir") failed"
	}
}

# tf_fails DIR PATTERN ARGS... requires terraform to fail with output matching PATTERN.
tf_fails() {
	local dir=$1 pattern=$2
	shift 2

	if tf "$dir" "$@"; then
		fail "terraform $* in $(basename "$dir") succeeded, want failure /$pattern/"
	fi

	grep -Eqi -- "$pattern" "$dir/tf.log" || {
		tail -n 40 "$dir/tf.log" >&2
		fail "terraform $* in $(basename "$dir") failed without /$pattern/"
	}

	grep -Ei -- "$pattern" "$dir/tf.log" | head -n 3 | sed 's/^/    | /'
}

# wait_both NAME_A PID_A NAME_B PID_B waits for both background terraform runs
# before judging either, so a failure never leaves the other one running.
wait_both() {
	local ra=0 rb=0

	wait "$2" || ra=$?
	wait "$4" || rb=$?
	bg_pids=()
	((ra == 0)) || { tail -n 40 "$flow_dir/tf/$1/tf.log" >&2; fail "$1 failed (exit $ra)"; }
	((rb == 0)) || { tail -n 40 "$flow_dir/tf/$3/tf.log" >&2; fail "$3 failed (exit $rb)"; }
}

tf_ids() { (cd "$1" && terraform output -json "${2:-ids}") | jq -r 'if type == "array" then .[] else . end'; }

alloc_count() { cli alloc list "$1" -o json | jq 'length'; }

# assert_store POOL WANT_IDS_FILE BOUND checks that the store holds exactly the
# allocations in WANT_IDS_FILE (one ID per line), all distinct, non-overlapping
# and inside BOUND.
assert_store() {
	local pool=$1 want=$2 bound=$3 list n

	list="$flow_dir/list.json"
	cli alloc list "$pool" -o json >"$list" || fail "ipam alloc list $pool failed"
	n="$(wc -l <"$want" | tr -d ' ')"

	[[ "$(sort -u "$want" | wc -l | tr -d ' ')" == "$n" ]] || fail "expected IDs are not distinct"
	[[ "$(jq 'length' "$list")" == "$n" ]] || fail "store holds $(jq 'length' "$list") allocations, want exactly $n"
	[[ "$(jq -r '.[].id' "$list" | sort -u | wc -l | tr -d ' ')" == "$n" ]] || fail "store IDs are not distinct"

	if ! diff <(sort "$want") <(jq -r '.[].id' "$list" | sort) >"$flow_dir/ids.diff"; then
		cat "$flow_dir/ids.diff" >&2
		fail "store allocation IDs differ from the IDs Terraform and the CLI reported"
	fi

	jq -r '.[].cidr' "$list" | "$bin/overlapcheck" -count "$n" -within "$bound" ||
		fail "allocations overlap, repeat or leave the pool"

	log "store check: $n allocations in $pool, IDs match Terraform+CLI, CIDRs distinct and inside $bound"
}


run_flow() {
	local name=$1 root="e2e-root" pool="e2e-apps" tiny="e2e-tiny"
	local platform a b c pa pb pc pool_cidr explicit cli_next cli_explicit cli_x r imp d x o id residue

	current="$name"
	flow_dir="$work/$name"
	mkdir -p "$flow_dir"
	log "=== $name: backend $backend_url"

	# Platform: the CLI creates the root pool, a platform Terraform state
	# carves the child pool requestors allocate from.
	expect_rc 0 '"cidrs":\["10\.0\.0\.0/16"\]' cli pool create "$root" --cidr 10.0.0.0/16 -o json
	platform="$(tf_dir platform platform)"
	tfvar "$platform" root_pool "\"$root\""
	tfvar "$platform" pool "\"$pool\""
	tf_ok "$platform" apply -auto-approve
	pool_cidr="$(tf_ids "$platform" cidrs)"
	[[ "$(cli pool get "$pool" -o json | jq -r '.cidrs[0] + " " + .parent.pool_id')" == "$pool_cidr $root" ]] ||
		fail "CLI sees pool $pool differently from the platform state ($pool_cidr)"
	printf '%s\n' "$pool_cidr" | "$bin/overlapcheck" -within 10.0.0.0/16 >/dev/null ||
		fail "platform pool $pool_cidr is outside the root"
	log "platform state created $pool = $pool_cidr; CLI agrees"

	log "doctor --expect-write $pool"
	expect_rc 0 '"check":"write-probe","pool":"'"$pool"'","status":"PASS"' cli doctor --expect-write "$pool" -o json

	# Two identical requestor states applied concurrently (count = 10 each).
	a="$(tf_dir requestor-a count)"
	b="$(tf_dir requestor-b count)"
	for d in "$a" "$b"; do
		tfvar "$d" pool "\"$pool\""
		tfvar "$d" n 10
	done
	if ! cmp -s "$a/main.tf" "$b/main.tf" || ! cmp -s "$a/terraform.tfvars" "$b/terraform.tfvars"; then
		fail "requestor configurations differ"
	fi

	log "concurrent apply: requestor-a and requestor-b (count = 10 each)"
	tf "$a" apply -auto-approve &
	pa=$!
	bg_pids+=("$pa")
	tf "$b" apply -auto-approve &
	pb=$!
	bg_pids+=("$pb")
	wait_both requestor-a "$pa" requestor-b "$pb"

	{ tf_ids "$a"; tf_ids "$b"; } >"$flow_dir/want.ids"
	[[ "$(wc -l <"$flow_dir/want.ids" | tr -d ' ')" == 20 ]] || fail "Terraform reported $(wc -l <"$flow_dir/want.ids") IDs, want 20"
	assert_store "$pool" "$flow_dir/want.ids" "$pool_cidr"

	expect_rc 0 'No changes' tf_plan_show "$a"

	# Cross-tool: CLI allocations race a for_each requestor state.
	explicit="$(awk -F'[./]' '{printf "%s.%s.%d.0/24", $1, $2, $3 + 63}' <<<"$pool_cidr")"
	c="$(tf_dir requestor-c foreach)"
	tfvar "$c" pool "\"$pool\""
	log "concurrent: requestor-c (for_each x3) apply while the CLI allocates next + explicit $explicit"
	tf "$c" apply -auto-approve &
	pc=$!
	bg_pids+=("$pc")
	local rn=0 re=0 rc_c=0
	cli_next="$(cli alloc create "$pool" --prefix-length 24 --description "e2e cli next" -o json | jq -r .id)" || rn=$?
	cli_explicit="$(cli alloc create "$pool" --cidr "$explicit" --label source=cli -o json | jq -r .id)" || re=$?
	wait "$pc" || rc_c=$?
	bg_pids=()
	((rn == 0 && re == 0)) || fail "CLI allocations during the requestor-c apply failed (next $rn, explicit $re)"
	((rc_c == 0)) || { tail -n 40 "$c/tf.log" >&2; fail "requestor-c apply failed"; }

	{ cat "$flow_dir/want.ids"; tf_ids "$c"; printf '%s\n%s\n' "$cli_next" "$cli_explicit"; } >"$flow_dir/want2.ids"
	assert_store "$pool" "$flow_dir/want2.ids" "$pool_cidr"
	[[ "$(cli alloc get "$pool" "$cli_explicit" -o json | jq -r .cidr)" == "$explicit" ]] ||
		fail "CLI explicit allocation does not hold $explicit"

	expect_rc 0 'No changes' tf_plan_show "$c"

	# Release and destroy: Terraform and the CLI each release their own.
	log "destroy requestor-a and requestor-b concurrently, requestor-c, CLI releases"
	tf "$a" destroy -auto-approve &
	pa=$!
	bg_pids+=("$pa")
	tf "$b" destroy -auto-approve &
	pb=$!
	bg_pids+=("$pb")
	wait_both requestor-a "$pa" requestor-b "$pb"
	tf_ok "$c" destroy -auto-approve
	expect_rc 0 '"released":true' cli alloc release "$pool" "$cli_next" -o json
	expect_rc 0 '"released":true' cli alloc release "$pool" "$cli_explicit" -o json
	[[ "$(alloc_count "$pool")" == 0 ]] || fail "allocations remain after destroy and release"
	log "store empty after destroy/release"

	# Failure: overlap (CLI-made allocation blocks a provider explicit request).
	cli_x="$(cli alloc create "$pool" --prefix-length 24 -o json | jq -r .id)"
	x="$(cli alloc get "$pool" "$cli_x" -o json | jq -r .cidr)"
	o="$(tf_dir overlap explicit)"
	tfvar "$o" pool "\"$pool\""
	tfvar "$o" cidr "\"$x\""
	log "failure: provider explicit $x over a CLI allocation"
	tf_fails "$o" 'overlap' apply -auto-approve
	expect_rc 4 'overlap' cli alloc create "$pool" --cidr "${x%.0/24}.128/25"

	# Failure: pool_not_empty from the CLI and from the provider.
	log "failure: pool_not_empty"
	expect_rc 4 'pool_not_empty' cli pool delete "$pool"
	expect_rc 4 'pool_not_empty' cli pool delete "$root"
	tf_fails "$platform" 'pool_not_empty' destroy -auto-approve
	expect_rc 0 '"id":"'"$pool"'"' cli pool get "$pool" -o json
	expect_rc 0 '"released":true' cli alloc release "$pool" "$cli_x" -o json

	# Failure: exhausted (/30 root holds two /31s).
	log "failure: exhausted"
	expect_rc 0 '' cli pool create "$tiny" --cidr 192.168.250.0/30
	x="$(tf_dir exhausted count)"
	tfvar "$x" pool "\"$tiny\""
	tfvar "$x" n 3
	tfvar "$x" prefix_length 31
	tf_fails "$x" 'exhausted' apply -auto-approve
	[[ "$(alloc_count "$tiny")" == 2 ]] || fail "exhausted pool holds $(alloc_count "$tiny") allocations, want 2"
	expect_rc 5 'exhausted' cli alloc create "$tiny" --prefix-length 31
	tf_ok "$x" destroy -auto-approve
	[[ "$(alloc_count "$tiny")" == 0 ]] || fail "exhausted pool not empty after destroy"
	expect_rc 0 '"deleted":true' cli pool delete "$tiny" -o json

	# Residue: a lost state leaves allocations behind; recover with import and
	# ipam alloc release, after which ipam pool delete succeeds.
	log "residue recovery"
	r="$(tf_dir residue count)"
	tfvar "$r" pool "\"$pool\""
	tfvar "$r" n 2
	tf_ok "$r" apply -auto-approve
	rm -f "$r"/terraform.tfstate*
	[[ "$(alloc_count "$pool")" == 2 ]] || fail "residue allocations not found"
	expect_rc 4 'pool_not_empty' cli pool delete "$pool"
	residue=()
	while IFS= read -r id; do residue+=("$id"); done < <(cli alloc list "$pool" -o json | jq -r '.[].id')
	imp="$(tf_dir residue-import import)"
	tfvar "$imp" pool "\"$pool\""
	tfvar "$imp" id "\"${residue[0]}\""
	tf_ok "$imp" apply -auto-approve
	[[ "$(tf_ids "$imp" id)" == "${residue[0]}" ]] || fail "import adopted the wrong allocation"
	tf_ok "$imp" destroy -auto-approve
	expect_rc 0 '"released":true' cli alloc release "$pool" "${residue[1]}" -o json
	[[ "$(alloc_count "$pool")" == 0 ]] || fail "residue remains after recovery"

	if [[ "$name" == local && "${IPAM_E2E_FORCE_OVERLAP:-}" == 1 ]]; then
		force_overlap "$pool" "$pool_cidr"
	fi

	# Teardown: provider deletes the child, the CLI deletes the root.
	tf_ok "$platform" destroy -auto-approve
	expect_rc 3 '' cli pool get "$pool"
	expect_rc 0 '"deleted":true' cli pool delete "$root" -o json
	[[ "$(cli pool list -o json | jq 'length')" == 0 ]] || fail "pools remain after teardown"

	printf 'E2E PASS %s\n' "$name"
}

tf_plan_show() {
	tf "$1" plan -detailed-exitcode || return $?
	tail -n 5 "$1/tf.log"
}

force_overlap() {
	local pool=$1 pool_cidr=$2 doc id a overlap

	doc="$flow_dir/store/reservations/$pool.json"
	id="$(cli alloc create "$pool" --prefix-length 24 -o json | jq -r .id)"
	a="$(cli alloc get "$pool" "$id" -o json | jq -r .cidr)"
	overlap="${a%.0/24}.128/25"
	cp "$doc" "$flow_dir/reservations.bak"

	log "IPAM_E2E_FORCE_OVERLAP: writing $overlap (inside $a) straight into $doc"
	jq --arg c "$overlap" '.allocations["01a00000-0000-7000-8000-000000000001"] = {
		request: {kind: "explicit", cidr: $c}, cidr: $c, description: "forced", labels: {},
		created_at: "2026-01-01T00:00:00Z"}' "$flow_dir/reservations.bak" >"$doc"
	expect_rc 2 'invalid CIDR' cli alloc list "$pool" -o json

	cp "$flow_dir/reservations.bak" "$doc"
	expect_rc 0 '"released":true' cli alloc release "$pool" "$id" -o json
	printf 'E2E PASS local-force-overlap (decode validation rejects %s)\n' "$overlap"
}


render_env_policy() {
	local pool=$1 bucket=$2 dir="$work/aws-policy" mirror="$aws_module/.terraform/providers"

	mkdir -p "$dir"

	if [[ -d "$mirror/registry.terraform.io/hashicorp/aws" ]]; then
		cat >"$dir/render.tfrc" <<EOF
provider_installation {
  filesystem_mirror {
    path    = "$mirror"
    include = ["registry.terraform.io/hashicorp/aws"]
  }
  direct {
    exclude = ["registry.terraform.io/hashicorp/aws"]
  }
}
EOF
	else
		printf 'provider_installation {\n  direct {}\n}\n' >"$dir/render.tfrc"
	fi

	cat >"$dir/main.tf" <<EOF
provider "aws" {
  region                      = "us-east-1"
  access_key                  = "e2e"
  secret_key                  = "e2e"
  skip_credentials_validation = true
  skip_requesting_account_id  = true
  skip_metadata_api_check     = true
  skip_region_validation      = true
}

module "ipam" {
  source              = "$aws_module"
  bucket_name         = "$bucket"
  region              = "us-east-1"
  platform_role_names = ["e2e-platform"]
  environments = {
    e2e = { pool_ids = ["$pool"], role_names = ["e2e-deployer"] }
  }
}
EOF

	(
		cd "$dir"
		export TF_CLI_CONFIG_FILE="$dir/render.tfrc"
		bounded 300 terraform init -input=false -no-color >"$dir/tf.log" 2>&1 &&
			bounded 300 terraform plan -input=false -no-color -out=tfplan >>"$dir/tf.log" 2>&1 &&
			terraform show -json tfplan |
			jq -er '.planned_values.root_module.child_modules[] | select(.address == "module.ipam")
				| .resources[] | select(.address == "module.ipam.aws_iam_policy.environment[\"e2e\"]")
				| .values.policy'
	)
}

run_restricted() {
	local bucket=$1 root="e2e-r-root" pool="e2e-r-apps" other="e2e-r-other" policy user=e2e-deployer secret=e2e-deployer-secret d id

	current="s3-restricted"
	flow_dir="$work/s3-restricted"
	mkdir -p "$flow_dir"

	if ! policy="$(render_env_policy "$pool" "$bucket")"; then
		tail -n 20 "$work/aws-policy/tf.log" >&2 || true
		fail "could not render the terraform-aws-ipam environment policy"
	fi

	printf '%s\n' "$policy" >"$flow_dir/policy.json"
	log "environment policy rendered by terraform-aws-ipam for pool $pool:"
	jq -c '.Statement[] | {Sid, Action, Resource}' "$flow_dir/policy.json" | sed 's/^/    | /'

	expect_rc 0 '' cli pool create "$root" --cidr 172.16.0.0/16
	expect_rc 0 '' cli pool create "$pool" --parent "$root" --prefix-length 20
	expect_rc 0 '' cli pool create "$other" --cidr 172.31.0.0/16

	if ! "$bin/e2etools" minio-user "$s3_endpoint" "$user" "$secret" "$flow_dir/policy.json"; then
		fail "could not create the restricted MinIO user/policy"
	fi

	AWS_ACCESS_KEY_ID="$user" AWS_SECRET_ACCESS_KEY="$secret"

	# The environment policy must not let a deployer rewrite pool definitions,
	# neither its own pool's nor another's: probe the documents directly.
	log "as $user: direct PutObject on pools/$pool.json and pools/$other.json must be 403 AccessDenied"
	"$bin/e2etools" s3-put-denied "$s3_endpoint" "$bucket" "pools/$pool.json" "pools/$other.json" ||
		fail "restricted identity was not denied a direct write to a pool document"

	log "as $user: doctor --expect-write $pool --expect-no-write $other"
	expect_rc 0 '"pool":"'"$other"'","status":"PASS"' cli doctor --expect-write "$pool" --expect-no-write "$other" -o json
	log "as $user: pool create must be denied"
	expect_rc 1 'denied' cli pool create e2e-r-evil --cidr 192.0.2.0/24
	log "as $user: allocation in a pool outside the policy must be denied"
	expect_rc 1 'denied' cli alloc create "$other" --prefix-length 24

	d="$(tf_dir restricted count)"
	tfvar "$d" pool "\"$pool\""
	tfvar "$d" n 3
	log "as $user: terraform apply (3 allocations)"
	tf_ok "$d" apply -auto-approve
	tf_ids "$d" >"$flow_dir/want.ids"
	assert_store "$pool" "$flow_dir/want.ids" "$(cli pool get "$pool" -o json | jq -r '.cidrs[0]')"
	tf_ok "$d" destroy -auto-approve
	[[ "$(alloc_count "$pool")" == 0 ]] || fail "restricted destroy left allocations"
	AWS_ACCESS_KEY_ID=minioadmin AWS_SECRET_ACCESS_KEY=minioadmin

	expect_rc 3 '' cli pool get e2e-r-evil
	for id in "$pool" "$root" "$other"; do
		expect_rc 0 '"deleted":true' cli pool delete "$id" -o json
	done

	printf 'E2E PASS s3-restricted\n'
}


run_local() {
	mkdir -p "$work/local/store"
	provider_body="  local = {
    path = \"$work/local/store\"
  }"
	backend_url="file://$work/local/store"
	run_flow local
}

run_azblob() {
	current="azblob"
	emulators_started=1
	log "starting Azurite on :$IPAM_AZURITE_PORT (state $IPAM_EMULATOR_STATE_DIR)"
	AZURE_STORAGE_CONNECTION_STRING="$(bash "$ipam_repo/scripts/emulators/start-azurite.sh")"
	export AZURE_STORAGE_CONNECTION_STRING
	"$bin/e2etools" azurite-container ipam
	provider_body="  azure_blob = {
    use_connection_string_env = true
    container                 = \"ipam\"
  }"
	backend_url="azblob://ipam/"
	run_flow azblob
	unset AZURE_STORAGE_CONNECTION_STRING
}

s3_endpoint=""

run_s3() {
	local state kind pid bucket

	current="s3"
	emulators_started=1
	log "starting the S3 emulator on :$IPAM_S3_PORT"
	bash "$ipam_repo/scripts/emulators/start-s3.sh"
	state="$IPAM_EMULATOR_STATE_DIR/s3.json"
	kind="$(jq -r .kind "$state")"
	pid="$(jq -r .pid "$state")"

	if [[ "$pid" == null ]]; then
		if [[ -n "${IPAM_E2E_BACKENDS:-}" ]]; then
			fail "IPAM_E2E_BACKENDS requested s3 but no S3 emulator binary could be started; see $state"
		fi

		printf 'E2E SKIP s3 (no S3 emulator binary could be started; see %s)\n' "$state"
		printf 'E2E SKIP s3-restricted (no S3 emulator)\n'
		return 0
	fi

	if [[ "$kind" != minio ]]; then
		printf 'E2E NOTE s3: MinIO started but was NOT selected by the conditional-write probe (failed: %s).\n' \
			"$(jq -r '.failed_probes | join(", ")' "$state")"
		printf 'E2E NOTE s3: UNPROVEN on this emulator: conditional DELETE (If-Match), which pool delete relies on\n'
		printf 'E2E NOTE s3:   to lose cleanly against a concurrent writer. Conditional PUT (allocations) is proven.\n'
	fi

	s3_endpoint="$(jq -r .endpoint "$state")"
	bucket="$(jq -r .bucket "$state")"
	export AWS_ACCESS_KEY_ID=minioadmin AWS_SECRET_ACCESS_KEY=minioadmin AWS_REGION=us-east-1
	"$bin/e2etools" s3-versioning "$s3_endpoint" "$bucket"
	provider_body="  s3 = {
    bucket         = \"$bucket\"
    region         = \"us-east-1\"
    endpoint       = \"$s3_endpoint\"
    use_path_style = true
  }"
	backend_url="s3://$bucket/?region=us-east-1&path_style=true&endpoint=$s3_endpoint"
	run_flow s3
	run_restricted "$bucket"
	unset AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY AWS_REGION
}

run_real_clouds() {
	local suffix
	suffix="$(date +%s)-$$"

	if [[ -n "${IPAM_TEST_S3_BUCKET:-}" ]]; then
		provider_body="  s3 = {
    bucket = \"$IPAM_TEST_S3_BUCKET\"
    region = \"${IPAM_TEST_S3_REGION:-us-east-1}\"
    prefix = \"e2e-$suffix\"
  }"
		backend_url="s3://$IPAM_TEST_S3_BUCKET/e2e-$suffix?region=${IPAM_TEST_S3_REGION:-us-east-1}"
		run_flow s3-aws
	else
		printf 'E2E SKIP s3-aws (IPAM_TEST_S3_BUCKET not set; real AWS credential path and conditional PUT UNVERIFIED; this leg never tests module IAM grants)\n'
	fi

	if [[ -n "${AZURE_STORAGE_ACCOUNT_URL:-}" ]]; then
		local host="${AZURE_STORAGE_ACCOUNT_URL#https://}" container="${AZURE_STORAGE_CONTAINER:-ipam}"
		host="${host%/}"
		provider_body="  azure_blob = {
    account_url = \"$AZURE_STORAGE_ACCOUNT_URL\"
    container   = \"$container\"
    prefix      = \"e2e-$suffix\"
  }"
		backend_url="azblob://$host/$container/e2e-$suffix"
		run_flow azblob-azure
	else
		printf 'E2E SKIP azblob-azure (AZURE_STORAGE_ACCOUNT_URL not set; real Azure credential path and conditional PUT UNVERIFIED; this leg never tests module RBAC/ABAC grants)\n'
	fi
}

for backend in $backends; do
	case "$backend" in
	local) run_local ;;
	azblob) run_azblob ;;
	s3) run_s3 ;;
	*) fail "unknown backend $backend" ;;
	esac
done

run_real_clouds

current="done"
log "all selected flows passed"
