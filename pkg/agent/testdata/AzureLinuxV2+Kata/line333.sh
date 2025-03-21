
# ***********************************************************************************
# WARNING: Changes to this file will be overwritten and not persisted.
# ***********************************************************************************
# whoami (used for health check of DNS)
health-check.localdns.local:53 {
    bind Node_Listener_IP Cluster_Listener_IP
    whoami
}
# VnetDNS overrides apply to DNS traffic from pods with dnsPolicy:default or kubelet (referred to as VnetDNS traffic).
.:53 {
    log
    bind Node_Listener_IP
    forward . VnetDNS_Server_IP {
        policy sequential
        max_concurrent 1000
    }
    ready Node_Listener_IP:8181
    cache 3600s {
        success 9984
        denial 9984
        serve_stale 3600s verify
        servfail 0
    }
    loop
    nsid localdns
    prometheus :9253
    template ANY ANY internal.cloudapp.net {
        match "^(?:[^.]+\.){4,}internal\.cloudapp\.net\.$"
        rcode NXDOMAIN
        fallthrough
    }
    template ANY ANY reddog.microsoft.com {
        rcode NXDOMAIN
    }
}
cluster.local:53 {
    errors
    bind Node_Listener_IP
    forward . CoreDNS_Service_IP {
        force_tcp
        policy sequential
        max_concurrent 1000
    }
    ready Node_Listener_IP:8181
    cache 3600s {
        success 9984
        denial 9984
        servfail 0
    }
    loop
    nsid localdns
    prometheus :9253
}
testdomain456.com:53 {
    log
    bind Node_Listener_IP
    forward . CoreDNS_Service_IP {
        policy sequential
        max_concurrent 1000
    }
    ready Node_Listener_IP:8181
    cache 3600s {
        success 9984
        denial 9984
        serve_stale 3600s verify
        servfail 0
    }
    loop
    nsid localdns
    prometheus :9253
}
# KubeDNS overrides apply to DNS traffic from pods with dnsPolicy:ClusterFirst (referred to as KubeDNS traffic).
.:53 {
    errors
    bind Cluster_Listener_IP
    forward . CoreDNS_Service_IP {
        policy sequential
        max_concurrent 1000
    }
    ready Cluster_Listener_IP:8181
    cache 3600s {
        success 9984
        denial 9984
        serve_stale 3600s verify
        servfail 0
    }
    loop
    nsid localdns-pod
    prometheus :9253
    template ANY ANY internal.cloudapp.net {
        match "^(?:[^.]+\.){4,}internal\.cloudapp\.net\.$"
        rcode NXDOMAIN
        fallthrough
    }
    template ANY ANY reddog.microsoft.com {
        rcode NXDOMAIN
    }
}
cluster.local:53 {
    log
    bind Cluster_Listener_IP
    forward . CoreDNS_Service_IP {
        force_tcp
        policy round_robin
        max_concurrent 1000
    }
    ready Cluster_Listener_IP:8181
    cache 3600s {
        success 9984
        denial 9984
        servfail 0
    }
    loop
    nsid localdns-pod
    prometheus :9253
}
testdomain567.com:53 {
    errors
    bind Cluster_Listener_IP
    forward . VnetDNS_Server_IP {
        policy random
        max_concurrent 1000
    }
    ready Cluster_Listener_IP:8181
    cache 3600s {
        success 9984
        denial 9984
        serve_stale 3600s immediate
        servfail 0
    }
    loop
    nsid localdns-pod
    prometheus :9253
}
