# Deploying feedreader on Google Compute Engine (Container-Optimized OS)

These scripts provision and update a single-VM feedreader deployment on a
[Container-Optimized OS](https://cloud.google.com/container-optimized-os)
(COS) Compute Engine instance, pulling the application image from **Google
Artifact Registry** (published by `.github/workflows/google.yml`).

The stack is the same Caddy + Authelia + feedreader Docker Compose project
in `deploy/` — see [`../../DEPLOY.md`](../../DEPLOY.md) for how it works.

## Why this is COS-specific

COS has no package manager and won't run non-containerized apps, so we
never `apt install` or `git clone` on the host. Instead:

- **GAR auth uses no keys.** The VM runs as a service account with
  `roles/artifactregistry.reader`; `docker-credential-gcr` (pre-installed
  on COS) exchanges the metadata-server token for a registry token on
  every pull.
- **Provisioning uses cloud-init**, the COS-native mechanism. It installs
  the Docker Compose plugin into `/var/lib/docker/cli-plugins` (the only
  persistent *and* executable path) and defines two systemd units.
- **Config + data live under `/var/lib/feedreader`** (persistent). The
  systemd units are recreated in `/etc` (a stateless tmpfs) on every boot
  by cloud-init, so the stack survives reboots and COS auto-updates.

## Prerequisites (on your workstation)

- `gcloud` CLI, authenticated (`gcloud auth login`) with rights to create
  instances, service accounts, IAM bindings, addresses, and firewall rules
  in the project.
- `openssl` and `ssh` (the scripts run from your machine, not the VM).
- A domain you control (to point an A record at the VM).

IAP is used for SSH (`--tunnel-through-iap`), so you don't need to open
port 22. Ensure the IAP TCP forwarding firewall allows your account, or
add `--no-tunnel-through-iap` by editing the `SSH`/`SCP` arrays if you
prefer public SSH.

## Usage

```bash
cd deploy/gce
cp config.env.example config.env
$EDITOR config.env          # set DOMAIN, ADMIN_EMAIL, project/zone, etc.

./setup-vm.sh               # provision a new VM (one-time)
```

`setup-vm.sh` walks through:

1. Enable APIs; create the `feedreader-vm` service account and grant it
   `roles/artifactregistry.reader`.
2. Reserve a static IP and open ports 80/443.
3. **Pause** and print the A record you must create
   (`DOMAIN → <static IP>`), then wait until DNS resolves — Caddy can't
   get a Let's Encrypt cert until it does.
4. Create the COS VM with the cloud-init bootstrap.
5. Generate the three Authelia secrets, prompt for an initial admin
   password, generate its argon2 hash, upload the rendered config, and
   start the stack.

Then open `https://<DOMAIN>/`, log in once with the admin password, and
register a passkey at `https://<DOMAIN>/authelia/`.

### Updating

```bash
cd deploy/gce
./update-vm.sh              # pull latest :main, recreate, prune
```

Pin a different image for one run (persists in the VM's `.env`):

```bash
IMAGE_TAG=main-<sha> ./update-vm.sh        # roll back / forward to a SHA
IMAGE_TAG=decouple-from-exedev ./update-vm.sh
```

## Files

| File | Purpose |
|---|---|
| `config.env.example` | Copy to `config.env`; all tunables live here |
| `cloud-init.yaml.tmpl` | COS bootstrap (compose install, GAR auth, systemd units) |
| `setup-vm.sh` | Provision a new VM end-to-end |
| `update-vm.sh` | Pull the latest image onto an existing VM |

## GCP-side IAM note

The scripts grant the VM's service account only **read** access to
Artifact Registry. Image *publishing* is handled separately by GitHub
Actions via Workload Identity Federation (see
`.github/workflows/google.yml`) — the VM never pushes.

## Troubleshooting

- **Cert not issuing:** confirm `DOMAIN` resolves to the static IP and
  ports 80/443 are open; check Caddy logs (command printed by setup).
- **`pull` denied:** verify the SA has `roles/artifactregistry.reader`
  and the instance was created with `--scopes cloud-platform`.
- **Stack didn't start on first boot:** that's expected before config is
  uploaded; `setup-vm.sh` starts `feedreader.service` after the scp step.
  Inspect with `sudo journalctl -u feedreader -u feedreader-bootstrap`.
