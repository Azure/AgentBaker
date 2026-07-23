# Changelog

## 2026-07-23

- Goal: Assign Renovate-managed GPU and NVIDIA package updates to GPU SIG members.
- Findings: Seven ownership rules cover the managed GPU packages; `nvidia-container-toolkit` is excluded with `<DO_NOT_UPDATE>`.
- Failed attempts: Exact-name GitHub searches missed some members, so their logins were resolved from AgentBaker commit metadata.
- Files changed: `.github/renovate.json`, `CHANGELOG.md`.
- Next step: Open a PR from a branch in `Azure/AgentBaker`.
