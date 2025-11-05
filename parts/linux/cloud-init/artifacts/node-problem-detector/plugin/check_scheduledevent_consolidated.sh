#!/bin/bash

# This plugin queries the IMDS for all scheduled events and checks the response for presence of the event type passed
# into the plugin.
# If event type is not currently scheduled (not in IMDS response), it returns OK.
# If scheduled event of requested event type is found, it returns NOTOK and stdout message for nodeCondition.

readonly OK=0
readonly NOTOK=1
readonly UNKNOWN=2
readonly TIME_OUT_CODE=28

TIME_OUT=24 # default IMDS request timeout 

# parse event type (t) and sleep time (s)
while getopts ':s:m:' OPTION; do
  case "$OPTION" in
    s)
      sleep "$OPTARG"  #redeploy =1s, reboot = 2s, freeze = 3s
      ;;
    m)
      TIME_OUT=$OPTARG
      ;;
    ?)
      echo "Usage: check_scheduledevent.sh -s <seconds> (optional) -m <seconds> (mandatory)"
      exit $UNKNOWN
      ;;
  esac
done

#convert HOSTNAME to VM name
VM_NAME=$HOSTNAME
if [ "${HOSTNAME:${#HOSTNAME}-10:-6}" == "vmss" ]; then
  digits=${HOSTNAME:${#HOSTNAME}-6}
  new_ending="_"$((36#$digits))
  VM_NAME=${HOSTNAME::-6}$new_ending
fi

# filter for correct vm (in resources) and requested event type with nearest NotBefore time
content=$(curl -m "$TIME_OUT" -H Metadata:true --noproxy "*" http://169.254.169.254/metadata/scheduledevents?api-version=2020-07-01)
# verify query connected
ec=$?
if [ $ec -ne 0 ]; then
  echo "IMDS query failed, exit code: $ec"
  if [ $ec == $TIME_OUT_CODE ]; then
    echo "Connection timed out after $TIME_OUT seconds."
  fi
  exit $UNKNOWN
fi

eventWithCorrectType=$(echo "$content" | jq --arg vm "$VM_NAME" '[.Events[]? | select(.Resources[]==$vm)] | sort_by(.EventStatus) | reverse | sort_by(.NotBefore)')

# verify query connected
if [ $? -ne 0 ]; then
  echo "IMDS query failed"
  exit $UNKNOWN
fi

# if no scheduled events of requested type are found, return OK
length=$(echo "$eventWithCorrectType" | jq length)
if [ "$length" -eq 0 ]; then
  echo "No scheduled VM event"
  exit $OK
fi

# capture EventType,EventStatus,EventNotBefore,Resources
ev_id=$(echo "$eventWithCorrectType" | jq -r '.[0].EventId')
ev_type=$(echo "$eventWithCorrectType" | jq -r '.[0].EventType')
ev_status=$(echo "$eventWithCorrectType" | jq -r '.[0].EventStatus')
ev_notbefore=$(echo "$eventWithCorrectType" | jq -r '.[0].NotBefore')
ev_source=$(echo "$eventWithCorrectType" | jq -r '.[0].EventSource')
ev_description=$(echo "$eventWithCorrectType" | jq -r '[.[].Description][0]')
ev_duration=$(echo "$eventWithCorrectType" | jq -r '[.[].DurationInSeconds][0]')

message="$ev_type $ev_status"

# Output and exit when requested event type is scheduled.
# Consolidated node condition has event type as message
if [ "$ev_type" = "Freeze" ] || [ "$ev_type" = "Reboot" ] || [ "$ev_type" = "Redeploy" ] || [ "$ev_type" = "Terminate" ] || [ "$ev_type" = "Preempt" ]; then
  # NotBefore is empty for events that are currently in progress.
  if [ "$ev_status" = "Scheduled" ]; then
    message="$message: $ev_notbefore"
  fi
  # truncate description if it might push message length to be > 512 characters
  if [ "${#ev_description}" -gt 400 ]; then
    ev_description="${ev_description:0:400}"
  fi
  message="$message. DurationInSeconds: $ev_duration. $ev_description For more information, see https://aka.ms/aks/scheduledevents."
  echo "$message"
  if [ "$ev_source" == "User" ] && [ "$ev_status" == "Scheduled" ]; then
    curl -s -H Metadata:true -X POST -d "{\"StartRequests\": [{\"EventId\": \"${ev_id}\"}]}" \
      http://169.254.169.254/metadata/scheduledevents?api-version=2020-07-01
  fi
  exit $NOTOK
else
  exit $OK
fi
