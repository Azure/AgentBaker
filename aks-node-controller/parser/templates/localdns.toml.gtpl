# ***********************************************************************************
# WARNING: Changes to this file will be overwritten and not persisted.
# ***********************************************************************************
# whoami (used for health check of DNS)
health-check.localdns.local:53 {
    bind {{getLocalDnsNodeListenerIp}} {{getLocalDnsClusterListenerIp}}
    whoami
}
# VnetDNS overrides apply to DNS traffic from pods with dnsPolicy:default or kubelet (referred to as VnetDNS traffic).
{{- range $domain, $override := $.LocalDnsProfile.VnetDnsOverrides -}}
{{- $isRootDomain := eq $domain "." -}}
{{- $fwdToClusterCoreDNS := or (hasSuffix $domain "cluster.local") (eq $override.ForwardDestination "ClusterCoreDNS")}}
{{- $forwardPolicy := "sequential" -}}
{{- if eq $override.ForwardPolicy "RoundRobin" -}}
    {{- $forwardPolicy = "round_robin" -}}
{{- else if eq $override.ForwardPolicy "Random" -}}
    {{- $forwardPolicy = "random" -}}
{{- end }}
{{$domain}}:53 {
	{{- if eq $override.QueryLogging "Error" }}
    errors
    {{- else if eq $override.QueryLogging "Log" }}
    log
    {{- end }}
    bind {{getLocalDnsNodeListenerIp}}
    {{- if $isRootDomain}}
    forward . {{getAzureDnsIp}} {
    {{- else}}
    {{- if $fwdToClusterCoreDNS}}
    forward . {{getCoreDnsServiceIp $}} {
    {{- else}}
    forward . {{getAzureDnsIp}} {
    {{- end}}
	{{- end}}
        {{- if eq $override.Protocol "ForceTCP"}}
        force_tcp
        {{- end}}
        policy {{$forwardPolicy}}
        max_concurrent {{$override.MaxConcurrent}}
    }
    ready {{getLocalDnsNodeListenerIp}}:8181
    cache {{$override.CacheDurationInSeconds}} {
        success 9984
        denial 9984
        {{- if ne $override.ServeStale "Disable"}}
        {{- if eq $override.ServeStale "Verify"}}
        serve_stale {{$override.ServeStaleDurationInSeconds}}s verify
        {{- else if eq $override.ServeStale "Immediate"}}
        serve_stale {{$override.ServeStaleDurationInSeconds}}s immediate
        {{- end }}
        {{- end }}
        servfail 0
    }
    loop
    nsid localdns
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
# KubeDNS overrides apply to DNS traffic from pods with dnsPolicy:ClusterFirst (referred to as KubeDNS traffic).
{{- range $domain, $override := $.LocalDnsProfile.KubeDnsOverrides}}
{{- $isRootDomain := eq $domain "." -}}
{{- $fwdToClusterCoreDNS := or (hasSuffix $domain "cluster.local") (eq $override.ForwardDestination "ClusterCoreDNS")}}
{{- $forwardPolicy := "" }}
{{- $forwardPolicy := "sequential" -}}
{{- if eq $override.ForwardPolicy "RoundRobin" -}}
    {{- $forwardPolicy = "round_robin" -}}
{{- else if eq $override.ForwardPolicy "Random" -}}
    {{- $forwardPolicy = "random" -}}
{{- end }}
{{$domain}}:53 {
	{{- if eq $override.QueryLogging "Error" }}
    errors
    {{- else if eq $override.QueryLogging "Log" }}
    log
    {{- end }}
    bind {{getLocalDnsClusterListenerIp}}
    {{- if $fwdToClusterCoreDNS}}
    forward . {{getCoreDnsServiceIp $}} {
    {{- else}}
    forward . {{getAzureDnsIp}} {
    {{- end}}
        {{- if eq $override.Protocol "ForceTCP"}}
        force_tcp
        {{- end}}
        policy {{$forwardPolicy}}
        max_concurrent {{$override.MaxConcurrent}}
    }
    ready {{getLocalDnsClusterListenerIp}}:8181
    cache {{$override.CacheDurationInSeconds}} {
        success 9984
        denial 9984
        {{- if ne $override.ServeStale "Disable"}}
        {{- if eq $override.ServeStale "Verify"}}
        serve_stale {{$override.ServeStaleDurationInSeconds}}s verify
        {{- else if eq $override.ServeStale "Immediate"}}
        serve_stale {{$override.ServeStaleDurationInSeconds}}s immediate
        {{- end }}
        {{- end }}
        servfail 0
    }
    loop
    nsid localdns-pod
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