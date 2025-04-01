#!/usr/bin/env bash

set -x

vmssResourceIds=""
for vmssModel in e2e/scenario-logs/*/vmssId.txt; do
  resourceId=$(cat ${vmssModel})
  vmssResourceIds="${vmssResourceIds} ${resourceId}"
done

if [ -n "${vmssResourceIds// }" ]; then
  az resource delete --ids ${vmssResourceIds}
fi