#!/bin/sh
set -eu
cd "$(dirname "$0")"
if [ ! -x "./Port8080" ]; then
  echo "Port8080 is missing. Re-download or re-extract the package."
  read -r -p "Press Return to close this window…" _
  exit 1
fi
exec "./Port8080"
