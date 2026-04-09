# systemd Preset Modes and ACL First-Boot Behavior

## Background

When `/etc/machine-id` is removed (as happens during VHD build cleanup), systemd treats the next boot as a **first boot** and runs `systemctl preset-all`. This applies preset rules to every unit that has an `[Install]` section.

## Preset File Format

Preset files contain ordered rules. For each unit, systemd walks the rules top-down and applies the **first match**:

```
enable aks-node-controller.service   # explicit allow
enable disk_queue.service            # explicit allow
...
disable *                            # catch-all deny
```

## Preset Modes

`systemctl preset-all` supports three modes via `--preset-mode`:

| Mode | `enable` rules | `disable` rules | Unmatched units |
|------|---------------|-----------------|-----------------|
| `full` (default) | Applied | Applied | N/A (all match `disable *`) |
| `enable-only` | Applied | **Ignored** | Left untouched |
| `disable-only` | **Ignored** | Applied | Left untouched |

### `full` mode (default)

Both `enable` and `disable` rules are applied. A `disable *` catch-all **actively disables** every unit not explicitly listed with `enable`. Services enabled during VHD build will be disabled on first boot unless they appear in the allowlist.

### `enable-only` mode

Only `enable` rules are applied. `disable` lines (including `disable *`) are **completely ignored**. Units that don't match any `enable` rule are **left in their current state** — if they were enabled during VHD build, they stay enabled.

### `disable-only` mode

Only `disable` rules are applied. `enable` lines are ignored.

## Why ACL Needs `configureFirstBootPresets`

ACL's first-boot `preset-all` runs in **full mode**. This means the `disable *` catch-all actively disables every service not in the allowlist. Without the preset file at `/etc/systemd/system-preset/99-default-disable.preset`, services enabled during VHD build (like `aks-node-controller.service`) would be disabled on first boot, breaking node provisioning.

### Why the preset must live in `/etc/`

ACL already ships a `disable *` preset in its **oem-azure sysext** (under `/usr/lib/`), but sysexts are merged by `systemd-sysext` which runs **after** first-boot preset-all. Writing the preset to `/etc/systemd/system-preset/` ensures:

1. It is **visible at earliest boot** before sysext merging.
2. It **persists in the VHD** across reboots.
3. It **takes priority** over `/usr/lib/` per systemd's lookup order (`/etc/` > `/run/` > `/usr/lib/`).

## Why Other Distros (Flatcar, Ubuntu, Mariner) Don't Need This

These distros either:

- Run first-boot `preset-all` in **enable-only** mode, so `disable *` has no effect and VHD-enabled services stay enabled.
- Don't trigger first-boot detection in the same way.
- Handle service enablement through other mechanisms (e.g., Ignition on Flatcar).

## Maintaining the Allowlist

Any service explicitly enabled during VHD build (in `pre-install-dependencies.sh` or similar) **must** be added to the `enable` list in `configureFirstBootPresets()` **before** the `disable *` line. Forgetting to add a service will cause it to be silently disabled on first boot.
