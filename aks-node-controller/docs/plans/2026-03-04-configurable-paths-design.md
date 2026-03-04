# Configurable Paths via CLI Flags

**Date**: 2026-03-04

## Problem

All file paths in `aks-node-controller` are hardcoded constants. The comment in `const.go` described this as intentional, but it makes testing painful (requires `/var/log/azure/` to exist) and the non-configurability is now considered a mistake.

## Design

### Flag surface

Each subcommand declares all flags it needs in its own `flag.FlagSet`. Logging flags are repeated across subcommands — verbose but self-contained and consistent with the existing dispatch model.

**`provision`**
```
--provision-config        string  (required) path to provision config file
--dry-run                 bool    print command without executing
--log-path                string  (default: /var/log/azure/aks-node-controller.log)
--events-dir              string  (default: /var/log/azure/Microsoft.Azure.Extensions.CustomScript/events)
--provision-json-file     string  (default: /var/log/azure/aks/provision.json)
--provision-complete-file string  (default: /opt/azure/containers/provision.complete)
```

**`provision-wait`**
```
--log-path                string  (default: /var/log/azure/aks-node-controller.log)
--events-dir              string  (default: /var/log/azure/Microsoft.Azure.Extensions.CustomScript/events)
--provision-json-file     string  (default: /var/log/azure/aks/provision.json)
--provision-complete-file string  (default: /opt/azure/containers/provision.complete)
```

### `const.go`

Remove the "intentionally non-configurable" comment. Keep the default values as named constants — used as flag defaults, not referenced directly in logic.

```go
const (
    defaultLogPath                   = "/var/log/azure/aks-node-controller.log"
    defaultEventsDir                 = "/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events"
    defaultProvisionJSONFilePath     = "/var/log/azure/aks/provision.json"
    defaultProvisionCompleteFilePath = "/opt/azure/containers/provision.complete"
)
```

### `main.go`

`configureLogging()` moves out of `main()` and into each subcommand handler, called after flag parsing so `--log-path` takes effect. `main()` becomes:

```go
func main() {
    app := App{cmdRun: cmdRunner}
    exitCode := app.Run(context.Background(), os.Args)
    os.Exit(exitCode)
}
```

The `App` struct gains an `eventLogger` field initialized per-subcommand after flags are parsed (same as today but wired through flags instead of hardcoded path).

### `app.go`

`run()` dispatch logic unchanged — still `args[1]` subcommand lookup via `getCommandRegistry()`.

Each subcommand handler:
1. Parses its own `FlagSet`
2. Calls `configureLogging(logPath)` — returns cleanup func
3. Initializes `eventLogger` with `eventsDir`
4. Executes business logic

### Data flow

```
main()
  └── app.Run(ctx, os.Args)
        └── run() dispatches on args[1]
              └── handler(args[2:])
                    ├── parse FlagSet (all flags including --log-path, --events-dir)
                    ├── configureLogging(logPath)  ← moved here from main()
                    ├── init eventLogger(eventsDir)
                    └── business logic (unchanged)
```

### `ProvisionFlags` struct

Gains the new path fields:

```go
type ProvisionFlags struct {
    ProvisionConfig       string
    ProvisionJSONFile     string
    ProvisionCompleteFile string
}
```

`writeCompleteFileOnError` and `ProvisionWait` receive paths through their existing struct params rather than the package-level constants.

## What does not change

- `App` struct fields (`cmdRun`, `eventLogger`)
- `Provision()`, `ProvisionWait()`, `writeCompleteFileOnError()` signatures (already accept structs)
- `errToExitCode()` logic
- All tests — `ProvisionStatusFiles` already accepts paths, tests already use temp dirs
- Exit code preservation behavior
