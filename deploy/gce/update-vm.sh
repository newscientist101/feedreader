#!/usr/bin/env bash
# Update an existing feedreader COS instance to the latest published image.
#
# Pulls the current :main tag from Google Artifact Registry and recreates
# the stack. The VM's attached service account + docker-credential-gcr
# handle GAR auth transparently (a fresh metadata token per pull), so no
# re-authentication step is needed.
#
#   cd deploy/gce
#   ./update-vm.sh
#
# Optionally override the tag (e.g. pin a SHA image or test a branch):
#   IMAGE_TAG=main-<sha> ./update-vm.sh
#   IMAGE_TAG=decouple-from-exedev ./update-vm.sh
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [[ ! -f "${HERE}/config.env" ]]; then
  echo "ERROR: ${HERE}/config.env not found. Copy config.env.example and edit it." >&2
  exit 1
fi
# shellcheck disable=SC1091
source "${HERE}/config.env"

REMOTE_DIR=/var/lib/feedreader/deploy
COMPOSE=/var/lib/docker/cli-plugins/docker-compose
GC=(gcloud --project "${PROJECT_ID}")
SSH=("${GC[@]}" compute ssh "${INSTANCE_NAME}" --zone "${ZONE}" --tunnel-through-iap)

log() { printf '\n\033[1;34m==>\033[0m %s\n' "$*"; }
die() { printf '\033[1;31mERROR:\033[0m %s\n' "$*" >&2; exit 1; }

command -v gcloud >/dev/null || die "gcloud not found."
"${GC[@]}" compute instances describe "${INSTANCE_NAME}" --zone "${ZONE}" >/dev/null 2>&1 \
  || die "Instance '${INSTANCE_NAME}' not found in ${ZONE}. Run setup-vm.sh first."

# Allow overriding the tracked tag for this run only. If set, rewrite the
# IMAGE_TAG line in the VM's .env so the change persists across reboots.
OVERRIDE_TAG="${IMAGE_TAG:-}"
if [[ -n "${OVERRIDE_TAG}" ]]; then
  log "Pinning IMAGE_TAG=${OVERRIDE_TAG} in ${REMOTE_DIR}/.env…"
  "${SSH[@]}" --command "\
    sudo sed -i '/^IMAGE_TAG=/d' ${REMOTE_DIR}/.env && \
    echo 'IMAGE_TAG=${OVERRIDE_TAG}' | sudo tee -a ${REMOTE_DIR}/.env >/dev/null"
fi

log "Pulling the latest image and recreating the stack…"
"${SSH[@]}" --command "\
  set -e; \
  export DOCKER_CONFIG=/var/lib/feedreader/.docker; \
  echo '--- before ---'; \
  sudo DOCKER_CONFIG=\$DOCKER_CONFIG ${COMPOSE} --project-directory ${REMOTE_DIR} images feedreader; \
  sudo DOCKER_CONFIG=\$DOCKER_CONFIG ${COMPOSE} --project-directory ${REMOTE_DIR} pull; \
  sudo systemctl restart feedreader.service; \
  echo '--- after ---'; \
  sudo DOCKER_CONFIG=\$DOCKER_CONFIG ${COMPOSE} --project-directory ${REMOTE_DIR} ps; \
  echo '--- pruning old images ---'; \
  sudo docker image prune -f"

log "Update complete. App: https://${DOMAIN}/"
