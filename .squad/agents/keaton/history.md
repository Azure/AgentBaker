# Keaton — Session History

## Initial Context (seeded)

### AgentBaker Maintainability Landscape
- **Multi-language codebase**: Go (service + tests), Bash (Linux provisioning), PowerShell (Windows provisioning), JSON (component configs, packer). Each language has different style norms.
- **Shared `parts/` directory**: Used by both VHD builder (packer) and AgentBaker service (CSE generation). Changes here ripple in two directions — a major coupling surface.
- **Cross-OS concerns**: Scripts must work on Ubuntu AND Azure Linux/Mariner. Package managers (apt vs dnf/tdnf), paths, and service names differ. Implicit OS assumptions are a common brittleness source.
- **6-month VHD support window**: Code must be forward and backward compatible. New CSE scripts must work on old VHDs and vice versa. This makes breaking changes especially dangerous.
- **Shell script variable scoping**: Functions should use `local` variables. Variables declared in one function are visible to others in bash — a major source of subtle bugs. The project guidelines explicitly warn against this.
- **Source chain dependencies**: Shell scripts `source` other scripts for shared functions. Missing or reordered `source` statements cause silent failures at provisioning time.
- **Snapshot test data**: Generated via `make generate`. Must be re-run after `parts/` or `pkg/` changes. Stale snapshots cause test failures that look unrelated to the actual change.

### Key Fragility Areas
- `parts/linux/cloud-init/artifacts/cse_helpers.sh` — heavily sourced by other scripts; changes here have wide blast radius
- `parts/common/components.json` — version strings coupled to download URLs, install scripts, and VHD content
- `pkg/agent/const.go` — embeds CSE scripts into Go service; Go string escaping can mask shell syntax errors
- `staging/cse/windows/` — CSE scripts that must work with VHDs from different release cycles
- `vhdbuilder/packer/` — packer configs that map and rename scripts from `parts/`; renaming a source file without updating the mapping breaks VHD build

---
*No sessions yet.*
