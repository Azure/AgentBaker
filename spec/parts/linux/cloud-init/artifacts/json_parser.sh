#!/bin/bash

# Extract image URL name from a JSON file
get_image_url_using_name() {
    local artifact_path="$1"
    local image_name="$2"
    jq -r --arg name "$image_name" '.images[] | select(.name == $name) | .value' "$artifact_path"
}
