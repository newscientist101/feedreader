#!/usr/bin/env bash
# Provision a brand-new feedreader instance on Google Compute Engine running
# Container-Optimized OS (COS), pulling the image from Google Artifact Registry.
#
# Run this from your workstation (needs: gcloud, openssl, ssh). It is
# idempotent-ish: re-running reuses existing GCP resources where possible,
# but it will refuse to clobber an existing instance.
#
#   cd deploy/gce
#   cp config.env.example config.env && $EDITOR config.env
#   ./setup-vm.sh
#
# What it does:
#   1. Enables required APIs and creates a read-only Artifact Registry
#      service account for the VM (no key files — metadata-server tokens).
#   2. Reserves a static external IP and opens ports 80/443.
#   3. WAITS for you to point your DOMAIN's A record at that IP (Caddy
#      cannot obtain a TLS cert until DNS resolves).
#   4. Creates the COS VM with a cloud-init bootstrap (installs compose,
#      configures GAR auth, defines the systemd units).
#   5. Generates Authelia secrets + admin password hash, uploads the
#      rendered deploy config, and starts the stack.
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_SRC="$(cd "${HERE}/.." && pwd)"   # the repo's deploy/ directory

if [[ ! -f "${HERE}/config.env" ]]; then
  echo "ERROR: ${HERE}/config.env not found. Copy config.env.example and edit it." >&2
  exit 1
fi
# shellcheck disable=SC1091
source "${HERE}/config.env"

REMOTE_DIR=/var/lib/feedreader/deploy
GAR_HOST="${GAR_LOCATION}-docker.pkg.dev"
FEEDREADER_IMAGE="${GAR_HOST}/${PROJECT_ID}/${GAR_REPOSITORY}/${GAR_IMAGE}"
# Pin the compose plugin version installed on the VM.
COMPOSE_VERSION="${COMPOSE_VERSION:-v2.39.1}"

GC=(gcloud --project "${PROJECT_ID}")
SSH=("${GC[@]}" compute ssh "${INSTANCE_NAME}" --zone "${ZONE}" --tunnel-through-iap)
SCP=("${GC[@]}" compute scp --zone "${ZONE}" --tunnel-through-iap)

log() { printf '\n\033[1;34m==>\033[0m %s\n' "$*"; }
die() { printf '\033[1;31mERROR:\033[0m %s\n' "$*" >&2; exit 1; }

# ── Preflight ────────────────────────────────────────────────────
for bin in gcloud openssl ssh; do
  command -v "$bin" >/dev/null || die "required command not found: $bin"
done
[[ "${DOMAIN}" != "feeds.example.com" ]] || die "Set DOMAIN in config.env to your real domain."

if "${GC[@]}" compute instances describe "${INSTANCE_NAME}" --zone "${ZONE}" >/dev/null 2>&1; then
  die "Instance '${INSTANCE_NAME}' already exists in ${ZONE}. Use update-vm.sh, or delete it first."
fi

# ── 1. APIs + service account ──────────────────────────────────────
log "Enabling required APIs (compute, artifactregistry, iamcredentials)…"
"${GC[@]}" services enable \
  compute.googleapis.com \
  artifactregistry.googleapis.com \
  iamcredentials.googleapis.com

if ! "${GC[@]}" iam service-accounts describe "${SA_EMAIL}" >/dev/null 2>&1; then
  log "Creating service account ${SA_EMAIL}…"
  "${GC[@]}" iam service-accounts create "${SA_NAME}" \
    --display-name "feedreader VM (Artifact Registry reader)"
else
  log "Service account ${SA_EMAIL} already exists."
fi

log "Granting roles/artifactregistry.reader to the service account…"
"${GC[@]}" projects add-iam-policy-binding "${PROJECT_ID}" \
  --member "serviceAccount:${SA_EMAIL}" \
  --role roles/artifactregistry.reader \
  --condition=None >/dev/null

# ── 2. Static IP + firewall ────────────────────────────────────────
ADDR_NAME="${INSTANCE_NAME}-ip"
if ! "${GC[@]}" compute addresses describe "${ADDR_NAME}" --region "${REGION}" >/dev/null 2>&1; then
  log "Reserving static external IP ${ADDR_NAME}…"
  "${GC[@]}" compute addresses create "${ADDR_NAME}" --region "${REGION}"
fi
STATIC_IP="$("${GC[@]}" compute addresses describe "${ADDR_NAME}" --region "${REGION}" --format='value(address)')"
log "Static IP: ${STATIC_IP}"

FW_NAME="allow-${NETWORK_TAG}"
if ! "${GC[@]}" compute firewall-rules describe "${FW_NAME}" >/dev/null 2>&1; then
  log "Creating firewall rule ${FW_NAME} (tcp:80,443 udp:443)…"
  "${GC[@]}" compute firewall-rules create "${FW_NAME}" \
    --direction INGRESS --action ALLOW \
    --rules tcp:80,tcp:443,udp:443 \
    --target-tags "${NETWORK_TAG}" \
    --source-ranges 0.0.0.0/0
fi

# ── 3. DNS gate ───────────────────────────────────────────────────
cat <<EOF

──────────────────────────────────────────────────────────────
ACTION REQUIRED: point DNS at the VM before continuing.

  Create an A record:   ${DOMAIN}  →  ${STATIC_IP}

Caddy obtains a Let's Encrypt certificate on first start, which requires
${DOMAIN} to already resolve to this IP. The script will wait for it.
──────────────────────────────────────────────────────────────
EOF

resolve_ip() {
  if command -v dig >/dev/null;       then dig +short A "${DOMAIN}" | tail -n1
  elif command -v host >/dev/null;     then host -t A "${DOMAIN}" 2>/dev/null | awk '/has address/{print $NF; exit}'
  elif command -v getent >/dev/null;   then getent ahostsv4 "${DOMAIN}" | awk '{print $1; exit}'
  else python3 -c "import socket;print(socket.gethostbyname('${DOMAIN}'))" 2>/dev/null
  fi
}
log "Waiting for ${DOMAIN} to resolve to ${STATIC_IP} (Ctrl-C to abort)…"
while :; do
  GOT="$(resolve_ip || true)"
  if [[ "${GOT}" == "${STATIC_IP}" ]]; then
    log "DNS OK: ${DOMAIN} → ${GOT}"; break
  fi
  printf '  current: %s (want %s) — retrying in 15s\n' "${GOT:-<none>}" "${STATIC_IP}"
  sleep 15
done

# ── 4. Render cloud-init + create the COS instance ───────────────────────
WORK="$(mktemp -d)"
trap 'rm -rf "${WORK}"' EXIT
CLOUD_INIT="${WORK}/cloud-init.yaml"
sed -e "s|@GAR_HOST@|${GAR_HOST}|g" \
    -e "s|@COMPOSE_VERSION@|${COMPOSE_VERSION}|g" \
    "${HERE}/cloud-init.yaml.tmpl" > "${CLOUD_INIT}"

log "Creating COS instance ${INSTANCE_NAME} in ${ZONE}…"
"${GC[@]}" compute instances create "${INSTANCE_NAME}" \
  --zone "${ZONE}" \
  --machine-type "${MACHINE_TYPE}" \
  --image-family "${IMAGE_FAMILY}" \
  --image-project "${IMAGE_PROJECT}" \
  --boot-disk-size "${BOOT_DISK_SIZE}" \
  --address "${STATIC_IP}" \
  --tags "${NETWORK_TAG}" \
  --service-account "${SA_EMAIL}" \
  --scopes "https://www.googleapis.com/auth/cloud-platform" \
  --metadata-from-file "user-data=${CLOUD_INIT}"

log "Waiting for SSH to come up…"
for _ in $(seq 1 30); do
  if "${SSH[@]}" --command 'true' >/dev/null 2>&1; then break; fi
  sleep 10
done
"${SSH[@]}" --command 'true' >/dev/null 2>&1 || die "Could not SSH to the instance."

# ── 5. Generate secrets + admin hash, render config, upload, start ─────────
log "Generating Authelia secrets…"
SESSION_SECRET="$(openssl rand -hex 32)"
STORAGE_KEY="$(openssl rand -hex 32)"
JWT_SECRET="$(openssl rand -hex 32)"

printf 'Set the initial Authelia password for user "%s" (used once, before passkey): ' "${ADMIN_USERNAME}"
read -rs ADMIN_PASSWORD; echo
[[ -n "${ADMIN_PASSWORD}" ]] || die "Empty password."

log "Generating argon2 password hash on the VM (public authelia image)…"
PASS_HASH="$("${SSH[@]}" --command \
  "docker run --rm authelia/authelia:latest authelia crypto hash generate argon2 --password '${ADMIN_PASSWORD}' 2>/dev/null | sed -n 's/^Digest: //p'")"
[[ "${PASS_HASH}" == \$argon2* ]] || die "Failed to generate password hash (got: ${PASS_HASH})"

log "Rendering deployment config…"
STAGE="${WORK}/deploy"
mkdir -p "${STAGE}/authelia"
cp "${DEPLOY_SRC}/docker-compose.yml" "${STAGE}/"
cp "${DEPLOY_SRC}/Caddyfile"        "${STAGE}/"
cp "${DEPLOY_SRC}/authelia/configuration.yml"   "${STAGE}/authelia/"
cp "${DEPLOY_SRC}/authelia/users_database.yml"  "${STAGE}/authelia/"

cat > "${STAGE}/.env" <<EOF
DOMAIN=${DOMAIN}
FEEDREADER_IMAGE=${FEEDREADER_IMAGE}
IMAGE_TAG=${IMAGE_TAG}
EOF

# Fill Authelia configuration.yml placeholders.
sed -i \
  -e "s|<CHANGE_ME_SESSION_SECRET>|${SESSION_SECRET}|" \
  -e "s|<CHANGE_ME_STORAGE_ENCRYPTION_KEY>|${STORAGE_KEY}|" \
  -e "s|<CHANGE_ME_JWT_SECRET>|${JWT_SECRET}|" \
  -e "s|<CHANGE_ME_DOMAIN>|${DOMAIN}|g" \
  "${STAGE}/authelia/configuration.yml"

# Render users_database.yml (replace the whole users: block).
cat > "${STAGE}/authelia/users_database.yml" <<EOF
# Generated by setup-vm.sh. The password is only used for the first login
# before a passkey is registered; after that, logins are passwordless.
users:
  ${ADMIN_USERNAME}:
    displayname: '${ADMIN_DISPLAYNAME}'
    password: '${PASS_HASH}'
    email: '${ADMIN_EMAIL}'
    groups: []
EOF

log "Uploading config to the VM and starting the stack…"
"${SCP[@]}" --recurse "${STAGE}" "${INSTANCE_NAME}:/tmp/feedreader-deploy"
"${SSH[@]}" --command "\
  sudo mkdir -p ${REMOTE_DIR} && \
  sudo cp -r /tmp/feedreader-deploy/. ${REMOTE_DIR}/ && \
  sudo rm -rf /tmp/feedreader-deploy && \
  sudo systemctl restart feedreader.service && \
  sudo systemctl status --no-pager feedreader.service | head -n 5"

cat <<EOF

✅ Done.

  App:        https://${DOMAIN}/   (Caddy is provisioning a TLS cert; give it a minute)
  Login user: ${ADMIN_USERNAME}
  Next:       log in once with your password, then register a passkey at
              https://${DOMAIN}/authelia/

Useful:
  Logs:       gcloud compute ssh ${INSTANCE_NAME} --zone ${ZONE} --command 'sudo journalctl -u feedreader -f'
  Caddy log:  gcloud compute ssh ${INSTANCE_NAME} --zone ${ZONE} --command \\
              'sudo /var/lib/docker/cli-plugins/docker-compose --project-directory ${REMOTE_DIR} logs caddy'
  Update:     ./update-vm.sh
EOF
