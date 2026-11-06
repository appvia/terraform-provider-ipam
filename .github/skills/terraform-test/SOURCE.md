# Source

`SKILL.md` is copied from matt-FFFFFF/agent-skills under the MIT licence; the
full notice is in `LICENSE` in this folder.

- Upstream: https://github.com/matt-FFFFFF/agent-skills/blob/ad50fc4e3250f4006f1e470956ca193048e601fa/terraform-test/skills/terraform-test/SKILL.md
- Commit SHA: ad50fc4e3250f4006f1e470956ca193048e601fa
- Licence: MIT

Changes from upstream:

- A "Data rule" section is added at the end, which this repo requires.
- Integration tests are marked as creating real infrastructure; agents write them but never run them.
- The unsupported `-no-cleanup` flag is replaced with `-verbose`.
- Apply-mode unit tests need a pre-run check (aliased providers mocked, no provisioners, no credentials) before they count as safe.

To update, copy the upstream file again at a new commit, re-apply the changes
above, and update this file.
