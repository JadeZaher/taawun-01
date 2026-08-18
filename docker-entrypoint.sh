#!/bin/sh
set -eu

# Railway volumes replace the image directory and initially arrive root-owned.
install -d -o taawun -g taawun /data /data/artifacts
exec su-exec taawun /app/taawun
