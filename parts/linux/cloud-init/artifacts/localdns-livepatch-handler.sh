#!/usr/bin/env bash
#
# localDNS live-patching handler, shared by the Ubuntu and Azure Linux reconcilers.
#
# Unlike securityPatch, nothing here is OS-specific: the work is delegated to
# aks-node-controller, which owns the payload schema and the Corefile render. This file
# therefore lives once and is sourced by both reconcilers rather than copied into each,
# so the outcome semantics below -- in particular that noCorefileData is a success and
# notFound is not -- cannot drift between the two.
#
# Both reconcilers run under `set -e`, so they source this file only after confirming it
# exists. A VHD baked before this file shipped simply has no localDNS handler registered,
# and the reconciler's "ignore unknown components" rule skips the component and keeps
# converging everything else.

# Path localdns.sh reads as the live-patched Corefile source. Defined once here because the
# handler both writes it and, on a rejected restart, has to remove it.
LOCALDNS_LIVEPATCHED_COREFILE="${LOCALDNS_LIVEPATCHED_COREFILE:-/opt/azure/containers/localdns/livepatched.localdns.corefile}"

localDNSIsCurrent() {
    local desired_payload="$1"
    local current_payload="$2"

    [ "${desired_payload}" = "${current_payload}" ]
}

updateLocalDNS() {
    local component_payload="${1:-}"
    local outcome

    if [ ! -x /opt/azure/containers/aks-node-controller ]; then
        echo "aks-node-controller binary is required for localDNS live patching"
        return 1
    fi

    if ! outcome="$(printf '%s' "${component_payload}" | /opt/azure/containers/aks-node-controller apply-localdns-config \
        --config-file - \
        --output "${LOCALDNS_LIVEPATCHED_COREFILE}")"; then
        echo "localDNS config apply failed"
        return 1
    fi
    printf '%s\n' "${outcome}"

    case "$(printf '%s\n' "${outcome}" | tail -n 1)" in
        applied)
            if ! systemctl restart localdns.service; then
                # CoreDNS refused this Corefile, so it is known-bad, not last-known-good.
                # Leaving it on disk would outlive the failed patch: localdns_source_corefile
                # prefers any non-empty live-patched file, and the boot-time LPS refresh keeps
                # the existing one when LPS is unreachable (discarding on a transient error
                # would flap a *working* node). So one rejected patch plus one unlucky boot
                # brings the node up on a Corefile already proven unusable. Drop it and let the
                # node come back on the baked Corefile.
                #
                # Recovering the running unit is deliberately left to its own tuned
                # Restart=/StartLimitBurst/OnFailure machinery; this only stops the failure
                # from persisting past the reboot.
                rm -f "${LOCALDNS_LIVEPATCHED_COREFILE}" "${LOCALDNS_LIVEPATCHED_COREFILE}.version"
                echo "failed to restart localdns.service; discarded the rejected corefile"
                return 1
            fi
            echo "localDNS update completed successfully"
            ;;
        alreadyCurrent)
            echo "localDNS is already current"
            ;;
        notFound)
            # In the runtime apply path the payload is supplied inline, so notFound is not expected;
            # treat it as a failure so the reconciler does not checkpoint an unapplied config as succeeded.
            echo "localDNS LPS config is not available"
            return 1
            ;;
        noCorefileData)
            # Nothing in this payload applies to this node: either no pool in the cluster has
            # localDNS enabled, or this node's pool does not. The component config is delivered
            # cluster-wide, so nodes outside the enabled pools see this on every tick.
            #
            # That is an authoritative answer from the control plane, not a failed apply, and the
            # bootstrap path already reads it that way. Reporting failure here would peg those
            # nodes at localDNS=Failed forever -- and because the reconcile loop only converges
            # once every component succeeds, it would also re-dispatch every component on every
            # tick for the life of the node.
            echo "localDNS has no config applicable to this node; nothing to apply"
            ;;
        *)
            echo "unexpected localDNS apply outcome: ${outcome}"
            return 1
            ;;
    esac
}
