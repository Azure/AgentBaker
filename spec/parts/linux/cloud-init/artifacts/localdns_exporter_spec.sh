#!/bin/bash

Describe 'localdns-exporter@.service security hardening'
    UNIT_FILE="./parts/linux/cloud-init/artifacts/localdns-exporter@.service"

    It 'should have DynamicUser=yes'
        When run grep -q "^DynamicUser=yes$" "$UNIT_FILE"
        The status should be success
    End

    It 'should have PrivateTmp=yes'
        When run grep -q "^PrivateTmp=yes$" "$UNIT_FILE"
        The status should be success
    End

    It 'should have ProtectSystem=strict'
        When run grep -q "^ProtectSystem=strict$" "$UNIT_FILE"
        The status should be success
    End

    It 'should have ProtectHome=yes'
        When run grep -q "^ProtectHome=yes$" "$UNIT_FILE"
        The status should be success
    End

    It 'should have ReadOnlyPaths=/'
        When run grep -q "^ReadOnlyPaths=/$" "$UNIT_FILE"
        The status should be success
    End

    It 'should have NoNewPrivileges=yes'
        When run grep -q "^NoNewPrivileges=yes$" "$UNIT_FILE"
        The status should be success
    End

    It 'should have ProtectKernelTunables=yes'
        When run grep -q "^ProtectKernelTunables=yes$" "$UNIT_FILE"
        The status should be success
    End

    It 'should have ProtectKernelModules=yes'
        When run grep -q "^ProtectKernelModules=yes$" "$UNIT_FILE"
        The status should be success
    End

    It 'should have ProtectControlGroups=yes'
        When run grep -q "^ProtectControlGroups=yes$" "$UNIT_FILE"
        The status should be success
    End

    It 'should have RestrictAddressFamilies with AF_UNIX AF_INET AF_INET6'
        When run grep -q "^RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6$" "$UNIT_FILE"
        The status should be success
    End

    It 'should have RestrictNamespaces=yes'
        When run grep -q "^RestrictNamespaces=yes$" "$UNIT_FILE"
        The status should be success
    End

    It 'should have LockPersonality=yes'
        When run grep -q "^LockPersonality=yes$" "$UNIT_FILE"
        The status should be success
    End

    It 'should have RestrictRealtime=yes'
        When run grep -q "^RestrictRealtime=yes$" "$UNIT_FILE"
        The status should be success
    End

    It 'should have RestrictSUIDSGID=yes'
        When run grep -q "^RestrictSUIDSGID=yes$" "$UNIT_FILE"
        The status should be success
    End

    It 'should have RemoveIPC=yes'
        When run grep -q "^RemoveIPC=yes$" "$UNIT_FILE"
        The status should be success
    End

    It 'should have PrivateMounts=yes'
        When run grep -q "^PrivateMounts=yes$" "$UNIT_FILE"
        The status should be success
    End
End

Describe 'localdns_exporter.sh HTTP request routing'
    SCRIPT_PATH="./parts/linux/cloud-init/artifacts/localdns_exporter.sh"

    It 'should return 404 for root path'
        When run bash -c "echo 'GET / HTTP/1.1' | $SCRIPT_PATH"
        The status should be success
        The output should include "HTTP/1.1 404 Not Found"
        The output should include "Content-Type: text/plain"
        The output should include "404 Not Found - Metrics available at /metrics"
        The output should not include "localdns_service_status"
        The output should not include "localdns_memory_usage_bytes"
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

    It 'should exit cleanly when client closes during metrics response'
        When run bash -o pipefail -c '
            tmp_dir=$(mktemp -d)
            trap "rm -rf ${tmp_dir}" EXIT
            {
                echo "# HELP localdns_service_status CoreDNS process status (1=active, 0=inactive)"
                echo "# TYPE localdns_service_status gauge"
                for i in $(seq 1 1000); do
                    echo "localdns_service_status{status=\"running\",sample=\"${i}\"} 1"
                done
            } > "${tmp_dir}/resources.prom"
            {
                echo "# HELP localdns_vnetdns_forward_info VnetDNS forward plugin IP address from corefile"
                echo "# TYPE localdns_vnetdns_forward_info gauge"
                echo "localdns_vnetdns_forward_info{ip=\"168.63.129.16\",block=\".:53\",status=\"ok\"} 1"
                echo "# HELP localdns_kubedns_forward_info KubeDNS forward plugin IP address from corefile"
                echo "# TYPE localdns_kubedns_forward_info gauge"
                echo "localdns_kubedns_forward_info{ip=\"10.0.0.10\",block=\"cluster.local:53\",status=\"ok\"} 1"
            } > "${tmp_dir}/forward_ips.prom"
            printf "GET /metrics HTTP/1.1\r\n\r\n" | LOCALDNS_SCRIPT_PATH="${tmp_dir}" '"$SCRIPT_PATH"' | head -n 1 >/dev/null
        '
        The status should be success
    End

    It 'should return 200 and Prometheus metrics for /metrics path'
        When run bash -c "echo 'GET /metrics HTTP/1.1' | $SCRIPT_PATH"
        The status should be success
        The output should include "HTTP/1.1 200 OK"
        The output should include "Content-Type: text/plain; version=0.0.4"
        # Verify metric type declarations are present
        The output should include "# TYPE localdns_service_status gauge"
        The output should include "# TYPE localdns_memory_usage_bytes gauge"
        The output should include "# TYPE localdns_cpu_usage_seconds_total counter"
        The output should include "# TYPE localdns_metrics_last_update_timestamp_seconds gauge"
        # Verify metric names are present (values will vary)
        The output should include "localdns_service_status"
        The output should include "localdns_memory_usage_bytes"
        The output should include "localdns_cpu_usage_seconds_total"
        The output should include "localdns_metrics_last_update_timestamp_seconds"
        # Verify forward-info metrics are present (either actual metrics or fallback)
        The output should include "localdns_vnetdns_forward_info"
        The output should include "localdns_kubedns_forward_info"
    End
End

build_large_ss_listen_output() {
    awk 'BEGIN {
        print "State Recv-Q Send-Q Local Address:Port Peer Address:Port Process"
        print "LISTEN 0 4096 127.0.0.1:9353 0.0.0.0:*"
        for (i = 0; i < 200000; i++) {
            print "LISTEN 0 4096 127.0.0.1:53 0.0.0.0:*"
        }
    }'
}

build_large_metrics_payload() {
    awk 'BEGIN {
        print "# HELP localdns_cpu_usage_seconds_total CPU usage"
        print "# TYPE localdns_cpu_usage_seconds_total counter"
        print "localdns_cpu_usage_seconds_total 1.234567890"
        for (i = 0; i < 200000; i++) {
            print "localdns_dummy_metric_" i " 1"
        }
    }'
}

unsafe_grep_pipeline_fails_under_pipefail() {
    local metrics
    local pipeline_status

    metrics=$(build_large_metrics_payload)
    (
        set -o pipefail
        echo "$metrics" | grep -q '^localdns_cpu_usage_seconds_total '
    )
    pipeline_status=$?

    [ "$pipeline_status" -ne 0 ]
}

unsafe_head_pipeline_fails_under_pipefail() {
    local ss_listen_output
    local pipeline_status

    ss_listen_output=$(build_large_ss_listen_output)
    (
        set -o pipefail
        echo "$ss_listen_output" | head -n 1 > /dev/null
    )
    pipeline_status=$?

    [ "$pipeline_status" -ne 0 ]
}

cached_ss_port_check_succeeds_under_pipefail() {
    local ss_listen_output
    local pipeline_status

    ss_listen_output=$(build_large_ss_listen_output)
    (
        set -o pipefail
        grep -q ':9353[[:space:]]' <<< "$ss_listen_output"
    )
    pipeline_status=$?

    [ "$pipeline_status" -eq 0 ]
}

cached_ss_listen_addr_is_extracted_without_a_pipe() {
    local ss_listen_output

    ss_listen_output=$(build_large_ss_listen_output)
    awk '/:9353[[:space:]]/ {print $4; exit}' <<< "$ss_listen_output"
}

here_string_metric_lookup_succeeds_under_pipefail() {
    local metrics
    local pipeline_status

    metrics=$(build_large_metrics_payload)
    (
        set -o pipefail
        grep -q '^localdns_cpu_usage_seconds_total ' <<< "$metrics"
    )
    pipeline_status=$?

    [ "$pipeline_status" -eq 0 ]
}

Describe 'validate-localdns-exporter-metrics.sh pipefail regressions'
    SCRIPT_PATH="./e2e/localdns/validate-localdns-exporter-metrics.sh"

    It 'demonstrates why echo-grep pipelines are unsafe under pipefail for large metrics payloads'
        When call unsafe_grep_pipeline_fails_under_pipefail
        The status should be success
    End

    It 'demonstrates why echo-head pipelines are unsafe under pipefail for large cached ss payloads'
        When call unsafe_head_pipeline_fails_under_pipefail
        The status should be success
    End

    It 'checks the cached ss output successfully under pipefail'
        When call cached_ss_port_check_succeeds_under_pipefail
        The status should be success
    End

    It 'extracts the listen address from cached ss output without a pipe'
        When call cached_ss_listen_addr_is_extracted_without_a_pipe
        The status should be success
        The output should equal "127.0.0.1:9353"
    End

    It 'looks up the cpu metric successfully with a here-string under pipefail'
        When call here_string_metric_lookup_succeeds_under_pipefail
        The status should be success
    End

    It 'caches ss output before matching the exporter port'
        When run grep -Fq 'SS_LISTEN_OUTPUT=$(ss -tln)' "$SCRIPT_PATH"
        The status should be success
    End

    It 'checks the cached ss output with a here-string'
        When run grep -Fq "grep -q ':9353[[:space:]]' <<< \"\$SS_LISTEN_OUTPUT\"" "$SCRIPT_PATH"
        The status should be success
    End

    It 'extracts the listen address from cached ss output with awk'
        When run grep -Fq "LISTEN_ADDR=\$(awk '/:9353[[:space:]]/ {print \$4; exit}' <<< \"\$SS_LISTEN_OUTPUT\")" "$SCRIPT_PATH"
        The status should be success
    End

    It 'looks up cpu metrics with grep via a here-string'
        When run grep -Fq "CPU_LINE=\$(grep -E \"^localdns_cpu_usage_seconds_total \" <<< \"\$METRICS\" || true)" "$SCRIPT_PATH"
        The status should be success
    End
End
