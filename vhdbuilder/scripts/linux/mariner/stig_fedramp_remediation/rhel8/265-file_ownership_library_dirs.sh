#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 265/364: 'file_ownership_library_dirs'")
for LIBDIR in /usr/lib /usr/lib64 /lib /lib64
do
  if [ -d $LIBDIR ]
  then
    find -L $LIBDIR \! -user root -exec chown root {} \; 
  fi
done
