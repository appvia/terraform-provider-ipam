# scripts

## OVERVIEW
Cross-repo end-to-end harness: real `ipam` CLI + real provider binary (Terraform `dev_overrides`) sharing one store per backend. Earned its file: distinct domain, nested Go module, 779-line hotspot (score ~9).

## STRUCTURE
| Path | Role |
|------|------|
| e2e.sh | driver: builds 4 binaries into a temp `bin/`, runs `run_flow` per backend, `run_restricted` (S3 IAM), `run_real_clouds` |
| overlapcheck/ | `overlapcheck [-within CIDR]... [-count N] < cidrs`; net/netip only, deliberately independent of ipam validation. Exit 0 disjoint, 1 overlap/outside/count, 2 usage. Part of root module (tested by `go test ./...`) |
| e2e/tools/ | `e2etools` emulator fixtures (`azurite-container`, `s3-versioning`, `minio-user`, `s3-put-denied`). OWN go.mod so MinIO admin deps never enter the provider graph |

## RUN
```bash
bash scripts/e2e.sh                                  # local azblob s3, then real-cloud legs if env set
IPAM_E2E_BACKENDS="local" bash scripts/e2e.sh        # subset of "local azblob s3"
IPAM_E2E_KEEP=1 ...                                  # keep $TMPDIR/ipam-e2e.XXXXXX (tf.log per tf dir)
```
Env: `IPAM_REPO` (default `../ipam`), `IPAM_AWS_MODULE` (default `../terraform-aws-ipam`), `IPAM_EMULATOR_STATE_DIR` (default `<work>/emulators`), `IPAM_AZURITE_PORT`/`IPAM_S3_PORT`/`IPAM_S3_CONSOLE_PORT` (10620/9620/9621), `IPAM_E2E_FORCE_OVERLAP=1` (local only), `IPAM_TEST_S3_BUCKET` (+`IPAM_TEST_S3_REGION`), `AZURE_STORAGE_ACCOUNT_URL` (+`AZURE_STORAGE_CONTAINER`, default `ipam`).

Requires: sibling `../ipam` (builds `./cmd/ipam`, uses `scripts/emulators/{start-azurite,start-s3,stop}.sh`), sibling `../terraform-aws-ipam` (renders the env IAM policy for `run_restricted`), `terraform`, `jq`, `perl` (`bounded` uses `alarm`), Azurite / MinIO binaries for those legs.

## FLOW FACTS
- Output contract: `E2E PASS <leg>`, `E2E SKIP <leg> (...)`, `E2E NOTE ...`, `E2E FAIL <leg>: <reason>`; `current` names the leg for failure messages; ERR trap records the failing line.
- S3 leg: missing emulator binary -> SKIP, unless `IPAM_E2E_BACKENDS` was set explicitly -> FAIL. Non-MinIO emulator -> NOTE that conditional DELETE is unproven.
- `run_flow` covers: CLI root pool + Terraform child pool, `doctor --expect-write`, two concurrent `count = 10` states, a `for_each` state racing CLI allocs, concurrent destroy, failures (explicit overlap, `pool_not_empty`, exhausted), residue recovery (state deleted -> import + CLI release), teardown to an empty store.
- This is the ONLY coverage of real-Terraform `for_each` (terraform-plugin-testing cannot run it).
- Real-cloud legs use ambient AWS / `ARM_*` creds with a unique `e2e-<ts>-<pid>` prefix; they never test module grants.

## CONVENTIONS
- CLI and terraform runs go through `bounded SECS` (`cli` 120s, `tf` 300s); wrap any new long-running command the same way.
- Assert with `expect_rc WANT PATTERN cmd...`, `tf_ok`, `tf_fails DIR PATTERN`; CLI exit codes matter (3 = not found, 4 = `pool_not_empty`).
- Cleanup trap kills background trees (`kill_tree`) and runs `stop.sh` when emulators started; keep new background PIDs in `bg_pids`.

## ANTI-PATTERNS
- Never `go build` in `e2e/tools` without `-o` (or use `-o /dev/null` to check it): the default `tools` binary is only gitignored, never commit it.
- Root `go test ./...` / `go vet ./...` skip `e2e/tools`; check it with `cd scripts/e2e/tools && go vet ./... && go build -o /dev/null .`.
- Do not import `github.com/appvia/ipam` from overlapcheck: its independence is the point.
