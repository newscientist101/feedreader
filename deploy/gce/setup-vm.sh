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
GAR_REPO_PATH="${GAR_HOST}/${PROJECT_ID}/${GAR_REPOSITORY}"
FEEDREADER_IMAGE="${GAR_REPO_PATH}/${GAR_IMAGE}"
IPV6_ONLY="${IPV6_ONLY:-0}"
# In IPv6-only mode the caddy/authelia images are mirrored into GAR so the
# IPv4-less VM can pull them via Private Google Access. Empty otherwise
# (compose falls back to the public Docker Hub defaults).
CADDY_IMAGE=""
AUTHELIA_IMAGE=""
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

# ── 2. Network: external IP (IPv4) or IPv6-only (free) + firewall ───
DNS_RECORD_TYPE="A"
WANT_IP=""
if [[ "${IPV6_ONLY}" == "1" ]]; then
  log "IPV6_ONLY=1 → provisioning a dual-stack VM with NO external IPv4 (free public IPv6)."
  DNS_RECORD_TYPE="AAAA"

  # Custom-mode VPC (auto-mode networks can't have IPv6 subnet ranges).
  if ! "${GC[@]}" compute networks describe "${IPV6_NETWORK}" >/dev/null 2>&1; then
    log "Creating custom-mode VPC ${IPV6_NETWORK}…"
    "${GC[@]}" compute networks create "${IPV6_NETWORK}" --subnet-mode custom
  fi
  # Dual-stack EXTERNAL subnet: internal IPv4 (for COS + IAP SSH) + public IPv6.
  if ! "${GC[@]}" compute networks subnets describe "${IPV6_SUBNET}" --region "${REGION}" >/dev/null 2>&1; then
    log "Creating dual-stack subnet ${IPV6_SUBNET} (${IPV6_SUBNET_RANGE} + external IPv6)…"
    "${GC[@]}" compute networks subnets create "${IPV6_SUBNET}" \
      --network "${IPV6_NETWORK}" --region "${REGION}" \
      --range "${IPV6_SUBNET_RANGE}" \
      --stack-type IPV4_IPV6 --ipv6-access-type EXTERNAL \
      --enable-private-ip-google-access
  fi

  # Firewall: allow web traffic over IPv6, and IAP SSH over the internal IPv4.
  if ! "${GC[@]}" compute firewall-rules describe "allow-${NETWORK_TAG}-v6" >/dev/null 2>&1; then
    log "Creating IPv6 web firewall rule (tcp:80,443 udp:443 from ::/0)…"
    "${GC[@]}" compute firewall-rules create "allow-${NETWORK_TAG}-v6" \
      --network "${IPV6_NETWORK}" --direction INGRESS --action ALLOW \
      --rules tcp:80,tcp:443,udp:443 \
      --target-tags "${NETWORK_TAG}" --source-ranges "::/0"
  fi
  if ! "${GC[@]}" compute firewall-rules describe "allow-${NETWORK_TAG}-iap" >/dev/null 2>&1; then
    log "Creating IAP SSH firewall rule (tcp:22 from 35.235.240.0/20)…"
    "${GC[@]}" compute firewall-rules create "allow-${NETWORK_TAG}-iap" \
      --network "${IPV6_NETWORK}" --direction INGRESS --action ALLOW \
      --rules tcp:22 --target-tags "${NETWORK_TAG}" \
      --source-ranges "35.235.240.0/20"
  fi
else
  ADDR_NAME="${INSTANCE_NAME}-ip"
  if ! "${GC[@]}" compute addresses describe "${ADDR_NAME}" --region "${REGION}" >/dev/null 2>&1; then
    log "Reserving static external IP ${ADDR_NAME}…"
    "${GC[@]}" compute addresses create "${ADDR_NAME}" --region "${REGION}"
  fi
  WANT_IP="$("${GC[@]}" compute addresses describe "${ADDR_NAME}" --region "${REGION}" --format='value(address)')"
  log "Static IP: ${WANT_IP}"

  FW_NAME="allow-${NETWORK_TAG}"
  if ! "${GC[@]}" compute firewall-rules describe "${FW_NAME}" >/dev/null 2>&1; then
    log "Creating firewall rule ${FW_NAME} (tcp:80,443 udp:443)…"
    "${GC[@]}" compute firewall-rules create "${FW_NAME}" \
      --direction INGRESS --action ALLOW \
      --rules tcp:80,tcp:443,udp:443 \
      --target-tags "${NETWORK_TAG}" \
      --source-ranges 0.0.0.0/0
  fi
fi

# ── 4. Render cloud-init + create the COS instance ───────────────────────
WORK="$(mktemp -d)"
trap 'rm -rf "${WORK}"' EXIT
CLOUD_INIT="${WORK}/cloud-init.yaml"
sed -e "s|@GAR_HOST@|${GAR_HOST}|g" \
    -e "s|@COMPOSE_VERSION@|${COMPOSE_VERSION}|g" \
    "${HERE}/cloud-init.yaml.tmpl" > "${CLOUD_INIT}"

log "Creating COS instance ${INSTANCE_NAME} in ${ZONE}…"
# Networking flags differ by mode.
if [[ "${IPV6_ONLY}" == "1" ]]; then
  NET_ARGS=(--network-interface="network=${IPV6_NETWORK},subnet=${IPV6_SUBNET},stack-type=IPV4_IPV6,no-address")
else
  NET_ARGS=(--address "${WANT_IP}")
fi

"${GC[@]}" compute instances create "${INSTANCE_NAME}" \
  --zone "${ZONE}" \
  --machine-type "${MACHINE_TYPE}" \
  --image-family "${IMAGE_FAMILY}" \
  --image-project "${IMAGE_PROJECT}" \
  --boot-disk-size "${BOOT_DISK_SIZE}" \
  --boot-disk-type "${BOOT_DISK_TYPE:-pd-standard}" \
  "${NET_ARGS[@]}" \
  --tags "${NETWORK_TAG}" \
  --service-account "${SA_EMAIL}" \
  --scopes "https://www.googleapis.com/auth/cloud-platform" \
  --metadata-from-file "user-data=${CLOUD_INIT}"

# Determine the VM's public IP for the DNS record.
if [[ "${IPV6_ONLY}" == "1" ]]; then
  WANT_IP="$("${GC[@]}" compute instances describe "${INSTANCE_NAME}" --zone "${ZONE}" \
    --format='value(networkInterfaces[0].ipv6AccessConfigs[0].externalIpv6)')"
  [[ -n "${WANT_IP}" ]] || die "VM has no external IPv6 address; check the subnet's ipv6-access-type=EXTERNAL."
  log "Public IPv6: ${WANT_IP}"
fi

log "Waiting for SSH to come up…"
for _ in $(seq 1 30); do
  if "${SSH[@]}" --command 'true' >/dev/null 2>&1; then break; fi
  sleep 10
done
"${SSH[@]}" --command 'true' >/dev/null 2>&1 || die "Could not SSH to the instance."

# ── 4a. IPv6-only: route Docker Hub images through a GAR remote repo ─────
# The VM has no external IPv4 and no Cloud NAT, so it cannot reach Docker
# Hub directly for the caddy/authelia images. Instead, create an Artifact
# Registry *remote repository* (a Docker Hub pull-through cache): Artifact
# Registry fetches from Docker Hub on its side, and the VM pulls from AR
# over Private Google Access (free, IPv4-internal to Google). We then point
# compose at the cached image paths.
if [[ "${IPV6_ONLY}" == "1" ]]; then
  DH_REPO="${DOCKERHUB_REMOTE_REPO:-docker-hub-remote}"
  DH_HOST="${GAR_LOCATION}-docker.pkg.dev/${PROJECT_ID}/${DH_REPO}"
  if ! "${GC[@]}" artifacts repositories describe "${DH_REPO}" --location "${GAR_LOCATION}" >/dev/null 2>&1; then
    log "Creating Artifact Registry Docker Hub remote repo '${DH_REPO}'…"
    "${GC[@]}" artifacts repositories create "${DH_REPO}" \
      --repository-format docker --location "${GAR_LOCATION}" \
      --mode remote-repository --remote-docker-repo docker-hub \
      --description "Docker Hub pull-through cache for the IPv6-only feedreader VM"
  fi
  # Docker Hub library images are under library/<name>; user images keep
  # their org. The remote repo mirrors the upstream path verbatim.
  CADDY_IMAGE="${DH_HOST}/library/caddy:2-alpine"
  AUTHELIA_IMAGE="${DH_HOST}/authelia/authelia:latest"
  log "caddy   -> ${CADDY_IMAGE}"
  log "authelia-> ${AUTHELIA_IMAGE}"
fi

# ── 5. Generate secrets + admin hash, render config, upload, start ─────────
log "Generating Authelia secrets…"
SESSION_SECRET="$(openssl rand -hex 32)"
STORAGE_KEY="$(openssl rand -hex 32)"
JWT_SECRET="$(openssl rand -hex 32)"

printf 'Set the initial Authelia password for user "%s" (used once, before passkey): ' "${ADMIN_USERNAME}"
read -rs ADMIN_PASSWORD; echo
[[ -n "${ADMIN_PASSWORD}" ]] || die "Empty password."

# Use the same authelia image the stack will run (the GAR-cached one in
# IPv6-only mode, since the VM can't reach Docker Hub directly).
HASH_IMAGE="${AUTHELIA_IMAGE:-authelia/authelia:latest}"
log "Generating argon2 password hash on the VM (${HASH_IMAGE})…"
PASS_HASH="$("${SSH[@]}" --command \
  "export DOCKER_CONFIG=/var/lib/feedreader/.docker; sudo -E docker run --rm '${HASH_IMAGE}' authelia crypto hash generate argon2 --password '${ADMIN_PASSWORD}' 2>/dev/null | sed -n 's/^Digest: //p'")"
[[ "${PASS_HASH}" == \$argon2* ]] || die "Failed to generate password hash (got: ${PASS_HASH})"

log "Rendering deployment config…"
STAGE="${WORK}/deploy"
mkdir -p "${STAGE}/authelia"
cp "${DEPLOY_SRC}/docker-compose.yml" "${STAGE}/"
cp "${DEPLOY_SRC}/Caddyfile"        "${STAGE}/"
cp "${DEPLOY_SRC}/authelia/configuration.yml"   "${STAGE}/authelia/"
cp "${DEPLOY_SRC}/authelia/users_database.yml"  "${STAGE}/authelia/"

{
  echo "DOMAIN=${DOMAIN}"
  echo "FEEDREADER_IMAGE=${FEEDREADER_IMAGE}"
  echo "IMAGE_TAG=${IMAGE_TAG}"
  # In IPv6-only mode, point caddy/authelia at the GAR Docker Hub cache.
  [[ -n "${CADDY_IMAGE}" ]]    && echo "CADDY_IMAGE=${CADDY_IMAGE}"
  [[ -n "${AUTHELIA_IMAGE}" ]] && echo "AUTHELIA_IMAGE=${AUTHELIA_IMAGE}"
} > "${STAGE}/.env"

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

# ── DNS gate ───────────────────────────────────────────────────
# Caddy obtains its Let's Encrypt cert on first start, which requires the
# domain to already resolve to the VM. Wait for the right record
# (A for IPv4 mode, AAAA for IPv6-only mode) before starting the stack.
cat <<EOF

──────────────────────────────────────────────────────────────
ACTION REQUIRED: point DNS at the VM before continuing.

  Create a ${DNS_RECORD_TYPE} record:   ${DOMAIN}  →  ${WANT_IP}

The script will wait until ${DOMAIN} resolves to this address.
EOF
if [[ "${DNS_RECORD_TYPE}" == "AAAA" ]]; then
  cat <<'EOF'
Note: this is an IPv6-only deployment — publish ONLY the AAAA record.
Clients on IPv4-only networks will not be able to reach the site.
EOF
fi
echo "──────────────────────────────────────────────────────────────"

resolve_ip() {
  local rec="$1"
  if command -v dig >/dev/null;     then dig +short "${rec}" "${DOMAIN}" | tail -n1
  elif command -v host >/dev/null;   then host -t "${rec}" "${DOMAIN}" 2>/dev/null | awk '/address/{print $NF; exit}'
  else python3 -c "import socket; fam = socket.AF_INET6 if '${rec}'=='AAAA' else socket.AF_INET; print(socket.getaddrinfo('${DOMAIN}', None, fam)[0][4][0])" 2>/dev/null
  fi
}
log "Waiting for ${DOMAIN} (${DNS_RECORD_TYPE}) to resolve to ${WANT_IP} (Ctrl-C to abort)…"
while :; do
  GOT="$(resolve_ip "${DNS_RECORD_TYPE}" || true)"
  if [[ "${GOT}" == "${WANT_IP}" ]]; then
    log "DNS OK: ${DOMAIN} → ${GOT}"; break
  fi
  printf '  current: %s (want %s) — retrying in 15s\n' "${GOT:-<none>}" "${WANT_IP}"
  sleep 15
done

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
