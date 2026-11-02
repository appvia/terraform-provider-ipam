# Source

`SKILL.md` is copied from matt-FFFFFF/agent-skills under the MIT licence; the
full notice is in `LICENSE` in this folder.

- Upstream: https://github.com/matt-FFFFFF/agent-skills/blob/ad50fc4e3250f4006f1e470956ca193048e601fa/terraform-schema/skills/terraform-schema/SKILL.md
- Commit SHA: ad50fc4e3250f4006f1e470956ca193048e601fa
- Licence: MIT

Changes from upstream:

- A "Data rule" section is added at the end, which this repo requires.
- Pass the exact `.terraform.lock.hcl` version for existing configuration instead of the constraint.
- Module `version list` order is not relied on; the example uses `sort -V | tail -1`.

To update, copy the upstream file again at a new commit, re-apply the changes
above, and update this file.
