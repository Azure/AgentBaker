#!/bin/bash

Describe 'localdns_exporter.sh HTTP request routing'
    SCRIPT_PATH="./parts/linux/cloud-init/artifacts/localdns_exporter.sh"

    It 'should return 404 for root path'
        When run bash -c "echo 'GET / HTTP/1.1' | $SCRIPT_PATH"
        The status should be success
        The output should include "HTTP/1.1 404 Not Found"
        The output should include "Content-Type: text/plain"
        The output should include "404 Not Found - Metrics available at /metrics"
        The output should not include "localdns_service_status"
        The output should not include "localdns_memory_usage_mb"
    End

    It 'should return 404 for /health path'
        When run bash -c "echo 'GET /health HTTP/1.1' | $SCRIPT_PATH"
        The status should be success
        The output should include "HTTP/1.1 404 Not Found"
        The output should include "Content-Type: text/plain"
        The output should include "404 Not Found - Metrics available at /metrics"
    End

    It 'should return 404 for /status path'
        When run bash -c "echo 'GET /status HTTP/1.1' | $SCRIPT_PATH"
        The status should be success
        The output should include "HTTP/1.1 404 Not Found"
        The output should include "404 Not Found - Metrics available at /metrics"
    End

    It 'should return 404 for /api/v1/metrics path'
        When run bash -c "echo 'GET /api/v1/metrics HTTP/1.1' | $SCRIPT_PATH"
        The status should be success
        The output should include "HTTP/1.1 404 Not Found"
        The output should include "404 Not Found - Metrics available at /metrics"
    End

    It 'should return 404 for invalid path'
        When run bash -c "echo 'GET /invalid HTTP/1.1' | $SCRIPT_PATH"
        The status should be success
        The output should include "HTTP/1.1 404 Not Found"
        The output should include "404 Not Found - Metrics available at /metrics"
    End

    It 'should have proper CRLF line endings in 404 response'
        When run bash -c "echo 'GET / HTTP/1.1' | $SCRIPT_PATH | head -c 100 | od -A n -t x1"
        The status should be success
        # Check for CRLF (0d 0a) in the HTTP header - look across multiple lines
        The output should include "0d"
        The output should include "0a"
    End

    It 'should exit cleanly when client disconnects without sending request'
        # Simulate client disconnect by providing no input (EOF immediately)
        When run bash -c "$SCRIPT_PATH < /dev/null"
        The status should be success
        The output should equal ""
    End

    It 'should return 200 and Prometheus metrics for /metrics path'
        When run bash -c "echo 'GET /metrics HTTP/1.1' | $SCRIPT_PATH"
        The status should be success
        The output should include "HTTP/1.1 200 OK"
        The output should include "Content-Type: text/plain; version=0.0.4"
        # Verify metric type declarations are present
        The output should include "# TYPE localdns_service_status gauge"
        The output should include "# TYPE localdns_memory_usage_mb gauge"
        The output should include "# TYPE localdns_cpu_usage_seconds_total counter"
        # Verify metric names are present (values will vary)
        The output should include "localdns_service_status"
        The output should include "localdns_memory_usage_mb"
        The output should include "localdns_cpu_usage_seconds_total"
        # Verify forward-info metrics are present (either actual metrics or fallback)
        The output should include "localdns_vnetdns_forward_info"
        The output should include "localdns_kubedns_forward_info"
    End
End
