
# whoami (used for health check of DNS)
health-check.akslocaldns.local:53 {
    bind 169.254.10.10 169.254.10.11
    whoami
}
# VNET DNS traffic (Traffic from pods with dnsPolicy:default or kubelet)
.:53 {
    log
    bind 169.254.10.10
    forward . Vnet_Dns_Servers {
        policy sequential
        max_concurrent 1000
    }
    ready 169.254.10.10:8181
    cache 3600s {
        success 9984
        denial 9984
        serve_stale 3600s verify
        servfail 0
    }
    loop
    nsid akslocaldns
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
    bind 169.254.10.10
    forward . 10.0.0.10 {
        force_tcp
        policy sequential
        max_concurrent 1000
    }
    ready 169.254.10.10:8181
    cache 3600s {
        success 9984
        denial 9984
        serve_stale 3600s verify
        servfail 0
    }
    loop
    nsid akslocaldns
    prometheus :9253
}
testdomain.com:53 {
    log
    bind 169.254.10.10
    forward . 10.0.0.10 {
        policy sequential
        max_concurrent 1000
    }
    ready 169.254.10.10:8181
    cache 3600s {
        success 9984
        denial 9984
        serve_stale 3600s verify
        servfail 0
    }
    loop
    nsid akslocaldns
    prometheus :9253
}
# Kube DNS traffic (Traffic from pods with dnsPolicy:ClusterFirst)
.:53 {
    log
    bind 169.254.10.11
    forward . 10.0.0.10 {
        force_tcp
        policy sequential
        max_concurrent 1000
    }
    ready 169.254.10.11:8181
    cache 3600s {
        success 9984
        denial 9984
        serve_stale 3600s verify
        servfail 0
    }
    loop
    nsid akslocaldns-pod
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
    bind 169.254.10.11
    forward . 10.0.0.10 {
        policy sequential
        max_concurrent 1000
    }
    ready 169.254.10.11:8181
    cache 3600s {
        success 9984
        denial 9984
        serve_stale 3600s verify
        servfail 0
    }
    loop
    nsid akslocaldns-pod
    prometheus :9253
}
testdomain.com:53 {
    log
    bind 169.254.10.11
    forward . Vnet_Dns_Servers {
        policy sequential
        max_concurrent 1000
    }
    ready 169.254.10.11:8181
    cache 3600s {
        success 9984
        denial 9984
        serve_stale 3600s verify
        servfail 0
    }
    loop
    nsid akslocaldns-pod
    prometheus :9253
}
