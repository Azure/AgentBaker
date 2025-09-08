# ***********************************************************************************
# WARNING: Changes to this file will be overwritten and not persisted.
# ***********************************************************************************
# whoami (used for health check of DNS)
health-check.localdns.local:53 {
    bind {{NodeListenerIP}} {{ClusterListenerIP}}
    whoami
}
# VnetDNS overrides apply to DNS traffic from pods with dnsPolicy:default or kubelet (referred to as VnetDNS traffic).
{{- range $domain, $override := $.LocalDNSProfile.VnetDNSOverrides -}}
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
    bind {{NodeListenerIP}}
    {{- if $isRootDomain}}
    forward . {{AzureDNSIP}} {
    {{- else}}
    {{- if $fwdToClusterCoreDNS}}
    forward . {{CoreDNSServiceIP $}} {
    {{- else}}
    forward . {{AzureDNSIP}} {
    {{- end}}
	{{- end}}
        {{- if eq $override.Protocol "ForceTCP"}}
        force_tcp
        {{- end}}
        policy {{$forwardPolicy}}
        max_concurrent {{$override.MaxConcurrent}}
    }
    ready {{NodeListenerIP}}:8181
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
{{- range $domain, $override := $.LocalDNSProfile.KubeDNSOverrides}}
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
    bind {{ClusterListenerIP}}
    {{- if $fwdToClusterCoreDNS}}
    forward . {{CoreDNSServiceIP $}} {
    {{- else}}
    forward . {{AzureDNSIP}} {
    {{- end}}
        {{- if eq $override.Protocol "ForceTCP"}}
        force_tcp
        {{- end}}
        policy {{$forwardPolicy}}
        max_concurrent {{$override.MaxConcurrent}}
    }
    ready {{ClusterListenerIP}}:8181
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