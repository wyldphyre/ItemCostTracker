#!/usr/bin/env bash
set -euo pipefail

IMAGE=itemcosttracker
OUTFILE="${IMAGE}.tar.gz"

echo "Building Docker image: ${IMAGE}"
docker build -t "${IMAGE}" .

echo "Exporting to ${OUTFILE}"
docker save "${IMAGE}" | gzip > "${OUTFILE}"

echo "Done: ${OUTFILE} ($(du -sh "${OUTFILE}" | cut -f1))"
