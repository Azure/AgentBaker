# whoami (used for health check of DNS)
health-check.akslocaldns.local:53 {
    bind {{$.NodeListenerIP}} {{$.ClusterListenerIP}}
    whoami
}
# VNET DNS traffic (Traffic from pods with dnsPolicy:default or kubelet)
{{- range $domain, $override := $.VnetDnsOverrides -}}
{{- $isRootDomain := eq $domain "." -}}
{{- $useClusterCoreDns := or (hasSuffix $domain "cluster.local") (eq $override.ForwardDestination "ClusterCoreDns")}}
{{$domain}}:53 {
    {{$override.QueryLogging}}
    bind {{$.NodeListenerIP}}
    {{- if $isRootDomain}}
    forward . Vnet_Dns_Servers {
    {{- else}}
    {{- if $useClusterCoreDns}}
    forward . {{$.CoreDnsServiceIP}} {
    {{- else}}
    forward . Vnet_Dns_Servers {
    {{- end}}
	{{- end}}
        {{- if $override.ForceTCP}}
        force_tcp
        {{- end}}
        policy {{$override.ForwardPolicy}}
        max_concurrent {{$override.MaxConcurrent}}
    }
    ready {{$.NodeListenerIP}}:8181
    cache {{$override.CacheDurationInSeconds}}s {
        success 9984
        denial 9984
        {{- if ne $override.ServeStale "Disabled"}}
        serve_stale {{$override.ServeStaleDurationInSeconds}}s {{$override.ServeStale}}
        {{- end}}
        servfail 0
    }
    loop
    nsid akslocaldns
    prometheus :9253
    {{- if $isRootDomain}}
    template ANY ANY internal.cloudapp.net {
        match "^(?:[^.]+\.){4,}internal\.cloudapp\.net\.$"
        rcode NXDOMAIN
        fallthrough
    }
    template ANY ANY reddog.microsoft.com {
        rcode NXDOMAIN
    }
    {{- end}}
}
{{- end}}
# Kube DNS traffic (Traffic from pods with dnsPolicy:ClusterFirst)
{{- range $domain, $override := $.KubeDnsOverrides}}
{{- $isRootDomain := eq $domain "." -}}
{{- $useClusterCoreDns := or (hasSuffix $domain "cluster.local") (eq $override.ForwardDestination "ClusterCoreDns")}}
{{$domain}}:53 {
    {{$override.QueryLogging}}
    bind {{$.ClusterListenerIP}}
    {{- if $useClusterCoreDns}}
    forward . {{$.CoreDnsServiceIP}} {
    {{- else}}
    forward . Vnet_Dns_Servers {
    {{- end}}
        {{- if $override.ForceTCP}}
        force_tcp
        {{- end}}
        policy {{$override.ForwardPolicy}}
        max_concurrent {{$override.MaxConcurrent}}
    }
    ready {{$.ClusterListenerIP}}:8181
    cache {{$override.CacheDurationInSeconds}}s {
        success 9984
        denial 9984
        {{- if ne $override.ServeStale "Disabled"}}
        serve_stale {{$override.ServeStaleDurationInSeconds}}s {{$override.ServeStale}}
        {{- end}}
        servfail 0
    }
    loop
    nsid akslocaldns-pod
    prometheus :9253
    {{- if $isRootDomain}}
    template ANY ANY internal.cloudapp.net {
        match "^(?:[^.]+\.){4,}internal\.cloudapp\.net\.$"
        rcode NXDOMAIN
        fallthrough
    }
    template ANY ANY reddog.microsoft.com {
        rcode NXDOMAIN
    }
    {{- end}}
}
{{- end}}