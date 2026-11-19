---
name: terraform-module
description: Use when creating or changing a Terraform module that follows the appvia/terraform-aws-module-template layout. Covers fmt, validate, tflint, terraform test and trivy checks. Never runs apply.
---

# Terraform modules

## Layout (appvia/terraform-aws-module-template)

```
main.tf          resources
variables.tf     inputs, each with type, description and validation where useful
outputs.tf       outputs, each with a description
terraform.tf     required_version and required_providers with version constraints
locals.tf        derived values (optional)
examples/basic/  a runnable example that calls the module from ../..
tests/           *.tftest.hcl files
.tflint.hcl      tflint rules
README.md        generated inputs and outputs (terraform-docs)
```

Copy the template's structure. Don't add top-level files it doesn't have without a reason in the plan.

## Checks

Run these from the module root, in order:

1. `terraform fmt -recursive -check`
2. `terraform init -backend=false`
3. `terraform validate`
4. `tflint --init && tflint --recursive`
5. `terraform test`. Write and run the tests with the `terraform-test` skill. Bare `terraform test` only reads `tests/`; when tests live in a subfolder such as `tests/unit`, run `terraform init -backend=false -test-directory=tests/unit` and `terraform test -test-directory=tests/unit`, or no tests run.
6. `trivy config .`

If the repo has a Makefile target for any of these, use it.

## Hard rules

- **Never run `terraform apply` or `terraform destroy`**, and never run a test with `command = apply` against a real account.
- Never commit `.terraform/`, state files or `*.tfvars` holding real values.
- New variables need a description and a type. Breaking input or output changes need a major version note in the PR.

## Data rule

Never put real data, client identifiers or credentials in any issue, spec, plan, test fixture, commit or log. Use made-up names (`tenant-a`, `user@example.com`) and obviously fake secrets (`dummy-token`).
