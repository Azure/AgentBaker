# Docker Testing for NPD Scripts

## Overview

This directory contains Docker-based tests for the Node Problem Detector (NPD) scripts. The tests run the actual production scripts in isolated Docker containers with mock data.

We successfully test the production scripts using Docker's `--privileged` mode, which allows us to mount mock data over system directories.

### How It Works

1. **Build Docker Image**: Contains the production scripts and required tools
2. **Create Mock Data**: Generates fake `/proc`, `/sys` etc files with test scenarios
3. **Run with --privileged**: Allows mounting mock data over system directories
4. **Custom Entrypoint**: Sets up bind mounts inside the container

### Test Coverage

**CPU Pressure Detection (`check_cpu_pressure_tests.sh`)**:

- Main detection logic (PSI metrics, load averages, iowait, steal time)
- iotop output validation with JSON parsing
- Edge cases (missing commands, permission errors, malformed output)

**Memory Pressure Detection (`check_memory_pressure_tests.sh`)**:

- PSI memory metrics and available memory thresholds
- OOM (Out of Memory) event detection from dmesg
- JSON structured logging for memory events
- Legacy kernel support (missing MemAvailable)

**RX Buffer Error Detection (`check_rx_buffer_errors_tests.sh`)**:

- Network interface rx_out_of_buffer metric monitoring
- Error rate calculations and thresholds
- State persistence between checks
- PCI interface filtering

**DNS Issues Detection (`check_dns_issues_tests.sh`)**:

- CoreDNS pod health checks via HTTP endpoints
- DNS resolution testing using nslookup
- UDP error tracking and delta detection
- RBAC permission failure handling
- Edge cases (HTTP/2 responses, wget output parsing, timeouts)

**NPD Startup Script (`check_npd_startup_tests.sh`)**:

- GPU detection and driver validation (nvidia-smi accessibility)
- NVLink hardware support detection and driver testing
- Azure IMDS response handling (timeout, malformed JSON, empty responses)
- Configuration management (kubeconfig location testing, container runtime endpoints)
- Toggle management (NPD validation, GPU checks enable/disable)
- Plugin configuration (GPU and NVLink health check inclusion/exclusion)
- Error handling (invalid JSON, missing config files, parse errors)
- Integration tests (complete startup flows for GPU and non-GPU VMs)

**XID Issues Detection (`check_xid_errors_tests.sh`)**:

- Check to see if the script works without any errors.
- GPU XID error with error code 48.
- GPU XID error with two errors: 48 and 56.

**Integration Tests (`integration-tests/`)**:

- **CPU Pressure Integration**: iotop output parsing and JSON validation
- **Pressure Common Functions**: Top, systemd-cgtop, and crictl stats testing

The tests successfully:

- Detect resource pressure via PSI metrics, system stats, and error rates
- Validate all logging functions and JSON output formatting
- Test error handling and edge cases
- Use mock data to simulate different scenarios

## Quick Start

```bash
# Ensure you're in the directory where the Makefile for these tests are
cd test/node-problem-detector

# Run all tests sequentially (see each test's output as it runs)
make test

# Run all tests in parallel (faster, buffers output until each test completes)
make test-parallel

# Run individual test suites
make test-check-cpu-pressure      # CPU pressure detection tests
make test-check-memory-pressure   # Memory pressure and OOM detection tests
make test-check-rx-buffer-errors  # RX buffer error detection tests
make test-check-dns-issues        # DNS issues detection tests (CoreDNS health, DNS resolution, UDP errors)
make test-check-npd-startup       # NPD startup tests (GPU detection, IMDS, configuration)
make test-check-xid-errors        # XID issues detection tests (GPU XID errors)
make test-pressure-common         # Pressure common function tests

# Run individual tests with debug output
make test-check-cpu-pressure DEBUG=true      # Show detailed test execution info
make test-check-memory-pressure DEBUG=true   # Show Docker commands and script output
make test-check-dns-issues DEBUG=true        # Show DNS test execution and validation details
make test-check-npd-startup DEBUG=true       # Show validation logic and failure details

# Run actual scripts with debug output
make debug

# Interactive shell for debugging
make shell

# Clean up
make clean
```

### Test Execution Modes

**Sequential (default: `make test`)**

- Tests run one after another
- Immediate output as each test executes
- Easier to debug and see which test is running
- Will take longer to run

**Parallel (`make test-parallel`)**

- All test files run simultaneously
- Output is buffered until each test suite completes to prevent interleaving
- Faster overall execution time
- Trade-off: no immediate feedback on test progress

**Debug Mode (`DEBUG=true`)**

- Add `DEBUG=true` to any individual test target for verbose output
- Shows Docker commands being executed, script output, and validation logic
- Helpful for troubleshooting test failures and understanding test behavior
- Example: `make test-check-cpu-pressure DEBUG=true`
- Debug output includes:
  - Test parameters and expected results
  - Full Docker command being run
  - Raw script output (first 20 lines)
  - Validation logic and results
  - Detailed failure analysis

## Test Structure

```
fixtures/
├── check_cpu_pressure_tests.sh         # CPU pressure detection tests
├── check_memory_pressure_tests.sh      # Memory pressure and OOM tests
├── check_rx_buffer_errors_tests.sh     # RX buffer error tests
├── check_dns_issues_tests.sh           # DNS issues detection tests
├── check_npd_startup_tests.sh          # NPD startup script tests
├── integration-tests/                  # Integration tests directory
│   ├── check_cpu_pressure_integration_tests.sh  # CPU integration tests (iotop and IG)
│   └── pressure_common_tests.sh        # pressure_common.sh function tests
├── common/                             # Common test utilities
│   ├── test_common.sh                  # Shared testing functions
│   └── event_log_validation.sh         # Event log validation functions
└── testdata/
    ├── create_cpu_test_data.sh         # Generates CPU test scenarios
    ├── create_memory_test_data.sh      # Generates memory test scenarios
    ├── create_rx_buffer_test_data.sh   # Generates RX buffer test scenarios
    ├── create_dns_test_data.sh         # Generates DNS test scenarios
    ├── create_pressure_common_test_data.sh  # Generates pressure common test mocks
    ├── create_iotop_test_data.sh       # Generates iotop test mocks
    ├── mock-data/                      # Test data for different scenarios
    │   ├── cpu-high-pressure/
    │   ├── memory-high-pressure/
    │   ├── rx-buffer-high-errors/
    │   ├── dns-healthy/
    │   ├── dns-unhealthy/
    │   └── [other scenarios]/
    └── mock-commands/                  # Mock system commands
        ├── dmesg-*                     # dmesg mocks for OOM detection
        ├── top-test-*                  # Top command mocks
        ├── systemd-cgtop-test-*        # Systemd-cgtop mocks
        ├── crictl-test-*               # Crictl mocks
        ├── iotop-test-*                # iotop mocks
        ├── mpstat-*                    # mpstat mocks
        ├── ip-*                        # ip command mocks
        ├── ethtool-*                   # ethtool mocks
        └── dns/                        # DNS command mocks
            ├── kubectl-*               # kubectl mocks (healthy, unhealthy, RBAC forbidden)
            ├── wget-*                  # wget mocks (HTTP/1.1, HTTP/2, timeouts)
            ├── nslookup-*              # nslookup mocks (success, failure)
            └── iptables-save           # iptables-save mock for CoreDNS IP discovery
```

## Technical Details

### Mounting Limitations

Even with `--privileged`, Docker doesn't allow directly mounting regular directories as `/proc` or `/sys`. Our solution uses a custom entrypoint that:

1. Copies mock files into `/proc` where possible
2. Bind mounts mock `/sys` subdirectories

For CPU integration tests (check_cpu_pressure_integration_tests.sh) but, in particular, for Inspektor Gadget (IG) tests, the whole filesystem is mocked because it's not possible to mount individual process folders in `/proc/<pid>` to simulate a process with information we control.
