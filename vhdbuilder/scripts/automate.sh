#!/bin/bash
set -x

echo "Starting script"
git branch
git status
git log --pretty=oneline