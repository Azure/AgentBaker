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
        --output /opt/azure/containers/localdns/livepatched.localdns.corefile)"; then
        echo "localDNS config apply failed"
        return 1
    fi
    printf '%s\n' "${outcome}"

    case "$(printf '%s\n' "${outcome}" | tail -n 1)" in
        applied)
            if ! systemctl restart localdns.service; then
                echo "failed to restart localdns.service"
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
