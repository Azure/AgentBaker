#!/bin/bash

set -euxo pipefail

source e2e-helper.sh

echo "scenario is $SCENARIO_NAME"
jq -s '.[0] * .[1]' nodebootstrapping_config.json scenarios/$SCENARIO_NAME/property-$SCENARIO_NAME.json > scenarios/$SCENARIO_NAME/nbc-$SCENARIO_NAME.json
