#!/usr/bin/env bash
set -euo pipefail

IMAGE=itemcosttracker
OUTFILE="${IMAGE}.tar.gz"

# The deployment server is linux/amd64. Build for it explicitly so the image
# isn't mislabelled as the build host's arch (e.g. arm64 on Apple Silicon),
# which the server can't run. PLATFORM can be overridden for other targets.
PLATFORM="${PLATFORM:-linux/amd64}"

echo "Building Docker image: ${IMAGE} (platform: ${PLATFORM})"
# --provenance=false / --sbom=false keep buildkit from wrapping the image in an
# attestation manifest list, which docker save/load handles poorly and can
# resurface the host/target platform mismatch warning on the server.
docker build --platform "${PLATFORM}" --provenance=false --sbom=false -t "${IMAGE}" .

echo "Exporting to ${OUTFILE}"
docker save "${IMAGE}" | gzip > "${OUTFILE}"

echo "Done: ${OUTFILE} ($(du -sh "${OUTFILE}" | cut -f1))"
