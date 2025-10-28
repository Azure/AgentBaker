# AgentBaker e2e/npd test Makefile

.PHONY: all test test-parallel test-check-cpu-pressure test-check-memory-pressure test-check-rx-buffer-errors test-check-dns-issues test-check-npd-startup test-pressure-common test-check-cpu-pressure-integration test-check-memory-pressure-integration test-check-dns-issues-integration build clean shell debug debug-tests show-mounts help test-check-xid-errors test-check-dns-vnetdns test-check-dns-coredns test-check-dns-localdns

# Default target - run all tests
all: test

# Build Docker image
build:
	@echo "Building Docker image for NPD tests..."
	@docker build --target without-ig -t npd-test-persistent -f docker/Dockerfile ../..

# Run all tests sequentially (default) - NPD core tests only
test: clean build
	@echo "\n\033[1;33mRunning NPD test files sequentially - each test will complete before the next begins...\033[0m"
	@fixtures/testdata/create_cpu_test_data.sh
	@fixtures/testdata/create_iotop_test_data.sh
	@fixtures/testdata/create_memory_test_data.sh
	@fixtures/testdata/create_rx_buffer_test_data.sh
	@fixtures/testdata/create_dns_test_data.sh
	@fixtures/testdata/create_dns_vnetdns_test_data.sh
	@fixtures/testdata/create_dns_coredns_test_data.sh
	@fixtures/testdata/create_dns_localdns_test_data.sh
	@fixtures/testdata/create_npd_startup_test_data.sh
	@fixtures/testdata/create_xid_test_data.sh
	@rm -f /tmp/npd_test_results.txt
	@SINGLE_TEST_MODE=true ./fixtures/check_cpu_pressure_tests.sh
	@SINGLE_TEST_MODE=true ./fixtures/check_memory_pressure_tests.sh
	@SINGLE_TEST_MODE=true ./fixtures/check_rx_buffer_errors_tests.sh
	@SINGLE_TEST_MODE=true ./fixtures/check_dns_issues_tests.sh
	@SINGLE_TEST_MODE=true ./fixtures/check_dns_vnetdns_tests.sh
	@SINGLE_TEST_MODE=true ./fixtures/check_dns_coredns_tests.sh
	@SINGLE_TEST_MODE=true ./fixtures/check_dns_localdns_tests.sh
	@SINGLE_TEST_MODE=true ./fixtures/check_npd_startup_tests.sh
	@SINGLE_TEST_MODE=true ./fixtures/check_xid_issues_tests.sh
	@SINGLE_TEST_MODE=true ./fixtures/integration-tests/pressure_common_tests.sh
	@SINGLE_TEST_MODE=true ./fixtures/integration-tests/check_cpu_pressure_integration_tests.sh
	@SINGLE_TEST_MODE=true ./fixtures/integration-tests/check_memory_pressure_integration_tests.sh
	@SINGLE_TEST_MODE=true ./fixtures/integration-tests/check_dns_issues_integration_tests.sh
	@echo ""
	@bash -c 'source fixtures/common/test_common.sh && print_overall_summary'

# Run all tests in parallel - NPD core tests only
test-parallel: clean build
	@echo "\n\033[1;33mRunning NPD test files in parallel - results will be displayed when each test file completes...\033[0m"
	@fixtures/testdata/create_cpu_test_data.sh
	@fixtures/testdata/create_iotop_test_data.sh
	@fixtures/testdata/create_memory_test_data.sh
	@fixtures/testdata/create_rx_buffer_test_data.sh
	@fixtures/testdata/create_dns_test_data.sh
	@fixtures/testdata/create_dns_vnetdns_test_data.sh
	@fixtures/testdata/create_dns_coredns_test_data.sh
	@fixtures/testdata/create_dns_localdns_test_data.sh
	@fixtures/testdata/create_npd_startup_test_data.sh
	@fixtures/testdata/create_xid_test_data.sh
	@rm -f /tmp/npd_test_results.txt
	@./fixtures/check_cpu_pressure_tests.sh & \
	./fixtures/check_memory_pressure_tests.sh & \
	./fixtures/check_rx_buffer_errors_tests.sh & \
	./fixtures/check_dns_issues_tests.sh & \
	./fixtures/check_dns_vnetdns_tests.sh & \
	./fixtures/check_dns_coredns_tests.sh & \
	./fixtures/check_dns_localdns_tests.sh & \
	./fixtures/check_npd_startup_tests.sh & \
	./fixtures/check_xid_issues_tests.sh & \
	./fixtures/integration-tests/pressure_common_tests.sh & \
	./fixtures/integration-tests/check_cpu_pressure_integration_tests.sh & \
	./fixtures/integration-tests/check_memory_pressure_integration_tests.sh & \
	./fixtures/integration-tests/check_dns_issues_integration_tests.sh & \
	wait; \
	echo ""; \
	bash -c 'source fixtures/common/test_common.sh && print_overall_summary'

# Run CPU pressure tests (includes iotop validation)
test-check-cpu-pressure: clean build
	@fixtures/testdata/create_cpu_test_data.sh
	@fixtures/testdata/create_iotop_test_data.sh
	@$(if $(DEBUG),DEBUG_MODE=true )SINGLE_TEST_MODE=true ./fixtures/check_cpu_pressure_tests.sh

# Run Memory pressure tests (includes OOM validation)
test-check-memory-pressure: clean build
	@fixtures/testdata/create_memory_test_data.sh
	@$(if $(DEBUG),DEBUG_MODE=true )SINGLE_TEST_MODE=true ./fixtures/check_memory_pressure_tests.sh

# Run RX buffer error tests (includes network interface monitoring)
test-check-rx-buffer-errors: clean build
	@fixtures/testdata/create_rx_buffer_test_data.sh
	@$(if $(DEBUG),DEBUG_MODE=true )SINGLE_TEST_MODE=true ./fixtures/check_rx_buffer_errors_tests.sh

# Run DNS issues tests (includes CoreDNS health checks, DNS resolution, UDP errors)
test-check-dns-issues: clean build
	@fixtures/testdata/create_dns_test_data.sh
	@$(if $(DEBUG),DEBUG_MODE=true )SINGLE_TEST_MODE=true ./fixtures/check_dns_issues_tests.sh

# Run DNS VNet tests (includes VNet DNS, CoreDNS, LocalDNS monitoring)
test-check-dns-vnetdns: clean build
	@fixtures/testdata/create_dns_vnetdns_test_data.sh
	@$(if $(DEBUG),DEBUG_MODE=true )SINGLE_TEST_MODE=true ./fixtures/check_dns_vnetdns_tests.sh

# Run DNS CoreDNS tests (includes CoreDNS IP discovery, in-cluster domain resolution)
test-check-dns-coredns: clean build
	@fixtures/testdata/create_dns_coredns_test_data.sh
	@$(if $(DEBUG),DEBUG_MODE=true )SINGLE_TEST_MODE=true ./fixtures/check_dns_coredns_tests.sh

# Run DNS LocalDNS tests (includes LocalDNS service status, listener validation, label checking)
test-check-dns-localdns: clean build
	@fixtures/testdata/create_dns_localdns_test_data.sh
	@$(if $(DEBUG),DEBUG_MODE=true )SINGLE_TEST_MODE=true ./fixtures/check_dns_localdns_tests.sh

# Run pressure common function tests (top, systemd-cgtop, crictl)
test-pressure-common: clean build
	@fixtures/testdata/create_cpu_test_data.sh
	@fixtures/testdata/create_pressure_common_test_data.sh
	@$(if $(DEBUG),DEBUG_MODE=true )SINGLE_TEST_MODE=true ./fixtures/integration-tests/pressure_common_tests.sh

# Run CPU pressure integration tests - iotop JSON validation
test-check-cpu-pressure-integration: clean build
	@fixtures/testdata/create_cpu_test_data.sh
	@fixtures/testdata/create_iotop_test_data.sh
	@$(if $(DEBUG),DEBUG_MODE=true )SINGLE_TEST_MODE=true ./fixtures/integration-tests/check_cpu_pressure_integration_tests.sh

# Run Memory pressure integration tests (OOM event detection and validation)
test-check-memory-pressure-integration: clean build
	@fixtures/testdata/create_memory_test_data.sh
	@$(if $(DEBUG),DEBUG_MODE=true )SINGLE_TEST_MODE=true ./fixtures/integration-tests/check_memory_pressure_integration_tests.sh

# Run DNS issues integration tests (RBAC handling and event log validation)
test-check-dns-issues-integration: clean build
	@fixtures/testdata/create_dns_test_data.sh
	@$(if $(DEBUG),DEBUG_MODE=true )SINGLE_TEST_MODE=true ./fixtures/integration-tests/check_dns_issues_integration_tests.sh

# Run NPD startup tests (GPU detection, IMDS, configuration management)
test-check-npd-startup: clean build
	@fixtures/testdata/create_npd_startup_test_data.sh
	@$(if $(DEBUG),DEBUG_MODE=true )SINGLE_TEST_MODE=true ./fixtures/check_npd_startup_tests.sh

# Run XID issues tests (GPU XID errors)
test-check-xid-errors: clean build
	@fixtures/testdata/create_xid_test_data.sh
	@$(if $(DEBUG),DEBUG_MODE=true )SINGLE_TEST_MODE=true ./fixtures/check_xid_issues_tests.sh

# Help target
help:
	@echo "Available targets:"
	@echo "  make build                                      - Build the Docker test image"
	@echo "  make test                                       - Run all tests sequentially (CPU + Memory + RX buffer + DNS + NPD startup + integrations)"
	@echo "  make test-parallel                              - Run all tests in parallel (faster but mixed output)"
	@echo "  make test-check-cpu-pressure                    - Run CPU pressure detection unit tests"
	@echo "  make test-check-memory-pressure                 - Run Memory pressure detection unit tests"
	@echo "  make test-check-rx-buffer-errors                - Run RX buffer error detection tests"
	@echo "  make test-check-dns-issues                      - Run DNS issues detection tests (CoreDNS health, DNS resolution, UDP errors)"
	@echo "  make test-check-dns-vnetdns                     - Run DNS VNet tests (VNet DNS, CoreDNS, LocalDNS monitoring)"
	@echo "  make test-check-dns-coredns                     - Run DNS CoreDNS tests (CoreDNS IP discovery, in-cluster domain resolution)"
	@echo "  make test-check-dns-localdns                    - Run DNS LocalDNS tests (LocalDNS service status, listener validation, label checking)"
	@echo "  make test-check-npd-startup                     - Run NPD startup tests (GPU detection, IMDS, configuration)"
	@echo "  make test-check-xid-errors                      - Run XID issues detection tests (GPU XID errors)"
	@echo "  make test-pressure-common                       - Run pressure_common.sh function tests (top, systemd-cgtop, crictl)"
	@echo "  make test-check-cpu-pressure-integration        - Run CPU pressure integration tests (iotop JSON validation)"
	@echo "  make test-check-memory-pressure-integration     - Run Memory pressure integration tests (OOM event detection)"
	@echo "  make test-check-dns-issues-integration          - Run DNS issues integration tests (RBAC handling and event log validation)"
	@echo ""
	@echo "  Add DEBUG=true to any individual test for verbose output:"
	@echo "  make test-check-cpu-pressure DEBUG=true         - Run CPU tests with debug output"
	@echo "  make test-check-memory-pressure DEBUG=true      - Run memory tests with debug output"
	@echo "  make test-check-dns-issues DEBUG=true           - Run DNS tests with debug output"
	@echo "  make test-check-npd-startup DEBUG=true          - Run startup tests with debug output"
	@echo ""
	@echo "  make shell                                      - Start interactive shell in test container"
	@echo "  make debug                                      - Run script directly with debug output"
	@echo "  make debug-tests                                - Run test fixtures with detailed debug output"
	@echo "  make clean                                      - Remove Docker image and generated files"
	@echo "  make show-mounts                                - Display mock data file structure"

# Clean up
clean:
	@echo "Cleaning up prior test data..."
	@docker rmi npd-test-persistent 2>/dev/null || true
	@rm -rf fixtures/testdata/mock-data/*
	@rm -rf fixtures/testdata/mock-commands/*
	@rm -rf fixtures/testdata/event-logs/*
	@echo "Clean complete"

# Debug: Run interactive shell in test container
shell: build
	@fixtures/testdata/create_cpu_test_data.sh
	@docker run --rm -it --privileged \
		-v "$$(pwd)/fixtures/testdata/mock-data/cpu-high-pressure/proc:/mock-proc:ro" \
		-v "$$(pwd)/fixtures/testdata/mock-data/cpu-high-pressure/sys:/mock-sys:ro" \
		-v "$$(pwd)/docker/docker_entrypoint.sh:/entrypoint.sh:ro" \
		-v "$$(pwd)/fixtures/testdata/mock-commands:/mock-commands:ro" \
		--entrypoint /entrypoint.sh \
		npd-test-persistent \
		/bin/bash

# Debug: Run script with verbose output
debug: build
	@fixtures/testdata/create_cpu_test_data.sh
	@docker run --rm --privileged \
		-v "$$(pwd)/fixtures/testdata/mock-data/cpu-high-pressure/proc:/mock-proc:ro" \
		-v "$$(pwd)/fixtures/testdata/mock-data/cpu-high-pressure/sys:/mock-sys:ro" \
		-v "$$(pwd)/docker/docker_entrypoint.sh:/entrypoint.sh:ro" \
		-v "$$(pwd)/fixtures/testdata/mock-commands/mpstat-high:/mock-commands/mpstat:ro" \
		-e "CONSECUTIVE_CHECKS=1" \
		-e "PSI_CPU_SOME_THRESHOLD=20" \
		-e "MAX_COMMAND_LENGTH=30" \
		--entrypoint /entrypoint.sh \
		npd-test-persistent \
		bash -x /etc/node-problem-detector.d/plugin/check_cpu_pressure.sh

# Debug: Run test fixtures with verbose output
debug-tests: build-with-ig
	@echo "Running test fixtures with debug output..."
	@fixtures/testdata/create_cpu_test_data.sh
	@fixtures/testdata/create_iotop_test_data.sh
	@fixtures/testdata/create_ig_test_data.sh
	@fixtures/testdata/create_memory_test_data.sh
	@fixtures/testdata/create_rx_buffer_test_data.sh
	@fixtures/testdata/create_dns_test_data.sh
	@fixtures/testdata/create_npd_startup_test_data.sh
	@fixtures/testdata/create_xid_test_data.sh
	@rm -f /tmp/npd_test_results.txt
	@echo "Running check_cpu_pressure_tests.sh with debug output..."
	@DEBUG_MODE=true ./fixtures/check_cpu_pressure_tests.sh; echo ""
	@echo "Running check_memory_pressure_tests.sh with debug output..."
	@DEBUG_MODE=true ./fixtures/check_memory_pressure_tests.sh; echo ""
	@echo "Running check_rx_buffer_errors_tests.sh with debug output..."
	@DEBUG_MODE=true ./fixtures/check_rx_buffer_errors_tests.sh; echo ""
	@echo "Running check_dns_issues_tests.sh with debug output..."
	@DEBUG_MODE=true ./fixtures/check_dns_issues_tests.sh; echo ""
	@echo "Running check_npd_startup_tests.sh with debug output..."
	@DEBUG_MODE=true ./fixtures/check_npd_startup_tests.sh; echo ""
	@echo "Running check_xid_issues_tests.sh with debug output..."
	@DEBUG_MODE=true ./fixtures/check_xid_issues_tests.sh; echo ""
	@echo "Running pressure_common_tests.sh with debug output..."
	@DEBUG_MODE=true ./fixtures/integration-tests/pressure_common_tests.sh; echo ""
	@echo "Running check_cpu_pressure_integration_tests.sh with debug output..."
	@DEBUG_MODE=true ./fixtures/integration-tests/check_cpu_pressure_integration_tests.sh; echo ""
	@echo "Running check_memory_pressure_integration_tests.sh with debug output..."
	@DEBUG_MODE=true ./fixtures/integration-tests/check_memory_pressure_integration_tests.sh; echo ""
	@echo "Running check_dns_issues_integration_tests.sh with debug output..."
	@DEBUG_MODE=true ./fixtures/integration-tests/check_dns_issues_integration_tests.sh; echo ""
	@echo ""
	@bash -c 'source fixtures/common/test_common.sh && print_overall_summary'

# Show what files would be mounted
show-mounts:
	@echo "Mock data structure:"
	@find fixtures/testdata/mock-data -type f 2>/dev/null | sort || echo "No mock data created yet. Run 'make test' first."
	@echo ""
	@echo "Mock test commands:"
	@find fixtures/testdata/mock-commands -name "*test*" -type f 2>/dev/null | sort || echo "No test commands created yet. Run 'make test' first."
