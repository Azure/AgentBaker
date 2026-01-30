#!/bin/sh

case "${PATH}" in
    /opt/bin:*) : ;;
    *) PATH=/opt/bin:${PATH} ;;
esac
