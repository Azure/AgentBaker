"""Import only the existing AKS CSE boot datapoints, without retaining CSE secrets."""

import datetime
import re


NAMES = {
    "KernelStartTime": "Kernel start (CSE-reported)",
    "GuestAgentStartTime": "Azure Guest Agent start",
    "CSEStartTime": "AKS CSE start",
    "KubeletStartTime": "Kubelet start",
}


def parse_timestamp(value, timezone_offset=None):
    if not isinstance(value, str):
        raise ValueError("A CSE boot timestamp must be a string")
    try:
        parsed = datetime.datetime.fromisoformat(value.replace("Z", "+00:00"))
        if parsed.tzinfo is not None:
            return parsed.isoformat()
    except ValueError:
        pass
    for pattern in ("%a %Y-%m-%d %H:%M:%S %z", "%a %b %d %H:%M:%S %z %Y"):
        normalized = re.sub(r"\b(?:UTC|GMT)\b", "+0000", value)
        if timezone_offset is not None:
            if not re.fullmatch(r"[+-]\d{2}:\d{2}", timezone_offset):
                raise ValueError("Timezone offset must be +/-HH:MM")
            normalized = re.sub(r"\b[A-Z]{2,5}\b", timezone_offset, normalized)
        try:
            return datetime.datetime.strptime(normalized, pattern).isoformat()
        except ValueError:
            continue
    raise ValueError(
        f"Unsupported or ambiguous CSE timestamp {value!r}. Export ISO-8601 with "
        "an offset, or supply --timezone-offset for an explicit source timezone."
    )


def import_cse(data, timezone_offset=None):
    if not isinstance(data, dict):
        raise ValueError("Export CSE boot datapoints as a JSON object")
    datapoints = data.get("BootDatapoints", data.get("bootDatapoints", data))
    if not isinstance(datapoints, dict):
        raise ValueError("BootDatapoints must be a JSON object")
    result = []
    for key, name in NAMES.items():
        value = datapoints.get(key, datapoints.get(key[0].lower() + key[1:]))
        if value:
            result.append({
                "name": name, "timestamp": parse_timestamp(value, timezone_offset),
                "source": "AKS CSEStatus.BootDatapoints." + key,
            })
    if not result:
        raise ValueError("No recognized CSE boot datapoints in the exported object")
    return result
