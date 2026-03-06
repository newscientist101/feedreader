package srv

// setupPageHTML is the self-contained HTML for the setup UI.
// It does not use base.html or any server templates since the app
// is not yet configured for normal operation.
const setupPageHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>FeedReader Setup</title>
<style>
*,*::before,*::after{box-sizing:border-box}
body{font-family:system-ui,-apple-system,sans-serif;margin:0;padding:0;
  background:#0f0f1a;color:#e0e0e0;min-height:100vh}
.container{max-width:640px;margin:0 auto;padding:2rem 1.5rem}
h1{font-size:1.75rem;margin:0 0 .25rem;color:#fff}
.subtitle{color:#888;margin:0 0 2rem;font-size:.95rem}
.warning{background:#3d2e00;border:1px solid #b38600;color:#ffd54f;
  padding:.75rem 1rem;border-radius:8px;margin-bottom:1.5rem;font-size:.9rem}
.section{background:#1a1a2e;border-radius:12px;padding:1.5rem;
  margin-bottom:1.5rem;border:1px solid #2a2a40}
.section h2{font-size:1.1rem;margin:0 0 1rem;color:#ccc}
label{display:block;font-size:.85rem;color:#999;margin-bottom:.25rem}
input,select{width:100%;padding:.6rem .75rem;background:#12121f;
  border:1px solid #333;border-radius:6px;color:#e0e0e0;font-size:.9rem;
  margin-bottom:1rem;outline:none;transition:border-color .15s}
input:focus,select:focus{border-color:#5c6bc0}
select{appearance:none;cursor:pointer;
  background-image:url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='12' height='8'%3E%3Cpath d='M1 1l5 5 5-5' stroke='%23888' stroke-width='1.5' fill='none'/%3E%3C/svg%3E");
  background-repeat:no-repeat;background-position:right .75rem center;
  padding-right:2.5rem}
.provider-config{display:none}
.provider-config.active{display:block}
.help{font-size:.8rem;color:#666;margin-top:-.75rem;margin-bottom:1rem}
btn,button,.btn{display:inline-block;padding:.7rem 1.5rem;
  background:#5c6bc0;color:#fff;border:none;border-radius:8px;
  font-size:.95rem;cursor:pointer;transition:background .15s;
  text-align:center;font-weight:500}
.btn:hover{background:#7986cb}
.btn:disabled{background:#333;color:#666;cursor:not-allowed}
.btn-row{display:flex;gap:1rem;align-items:center;margin-top:.5rem}
.status{font-size:.9rem;color:#888;margin-left:.5rem}
.status.error{color:#ef5350}
.status.success{color:#66bb6a}
.footer{text-align:center;color:#555;font-size:.8rem;margin-top:2rem}
.footer code{background:#1a1a2e;padding:.15rem .4rem;border-radius:4px}
</style>
</head>
<body>
<div class="container">
  <h1>📡 FeedReader Setup</h1>
  <p class="subtitle">Configure your feed reader before first use.</p>
  <!--NON_LOCAL_WARNING-->

  <form id="setup-form" autocomplete="off">
    <div class="section">
      <h2>Server</h2>
      <label for="listen">Listen address</label>
      <input type="text" id="listen" name="listen" value=":8000" placeholder=":8000">
      <p class="help">Address and port to bind to. Use :8000 for all interfaces or 127.0.0.1:8000 for localhost only.</p>

      <label for="db">Database path</label>
      <input type="text" id="db" name="db" value="db.sqlite3" placeholder="db.sqlite3">
    </div>

    <div class="section">
      <h2>Authentication</h2>
      <label for="provider">Auth provider</label>
      <select id="provider" name="provider">
        <option value="proxy">Reverse Proxy Headers (Caddy, nginx, Traefik, etc.)</option>
        <option value="tailscale">Tailscale Serve / Funnel</option>
        <option value="cloudflare">Cloudflare Tunnel + Access</option>
        <option value="authelia">Authelia</option>
        <option value="oauth2_proxy">OAuth2 Proxy</option>
        <option value="exedev">exe.dev (legacy)</option>
      </select>

      <div id="config-proxy" class="provider-config active">
        <label for="user_id_header">User ID header</label>
        <input type="text" id="user_id_header" name="user_id_header" placeholder="Remote-User">
        <p class="help">Leave blank for default (Remote-User). Your reverse proxy must set this header.</p>

        <label for="email_header">Email header</label>
        <input type="text" id="email_header" name="email_header" placeholder="Remote-Email">
        <p class="help">Leave blank for default (Remote-Email).</p>
      </div>

      <div id="config-cloudflare" class="provider-config">
        <label for="team_domain">Team domain</label>
        <input type="text" id="team_domain" name="team_domain" placeholder="myteam">
        <p class="help">Your Cloudflare Access team name (e.g. "myteam" for myteam.cloudflareaccess.com).</p>

        <label for="audience">Application Audience (AUD)</label>
        <input type="text" id="audience" name="audience" placeholder="">
        <p class="help">The Application Audience tag from your Cloudflare Access application settings.</p>
      </div>

      <div id="config-tailscale" class="provider-config">
        <p class="help" style="margin-top:0">No additional configuration needed. Tailscale injects user identity headers automatically when using <code>tailscale serve</code>.</p>
      </div>

      <div id="config-authelia" class="provider-config">
        <p class="help" style="margin-top:0">No additional configuration needed. Authelia sets Remote-User and Remote-Email headers through your reverse proxy.</p>
      </div>

      <div id="config-oauth2_proxy" class="provider-config">
        <p class="help" style="margin-top:0">No additional configuration needed. OAuth2 Proxy sets X-Forwarded-User and X-Forwarded-Email headers.</p>
      </div>

      <div id="config-exedev" class="provider-config">
        <p class="help" style="margin-top:0">Legacy exe.dev platform auth. Uses X-Exedev-Userid and X-Exedev-Email headers.</p>
      </div>
    </div>

    <div class="btn-row">
      <button type="submit" class="btn" id="save-btn">Save &amp; Restart</button>
      <span class="status" id="status"></span>
    </div>
  </form>

  <p class="footer">You can also configure via CLI: <code>feedreader init</code></p>
</div>

<script>
(function() {
  var providerSelect = document.getElementById('provider');
  var form = document.getElementById('setup-form');
  var statusEl = document.getElementById('status');
  var saveBtn = document.getElementById('save-btn');

  function showProviderConfig() {
    var configs = document.querySelectorAll('.provider-config');
    for (var i = 0; i < configs.length; i++) {
      configs[i].classList.remove('active');
    }
    var target = document.getElementById('config-' + providerSelect.value);
    if (target) target.classList.add('active');
  }

  providerSelect.addEventListener('change', showProviderConfig);
  showProviderConfig();

  form.addEventListener('submit', function(e) {
    e.preventDefault();
    saveBtn.disabled = true;
    statusEl.textContent = 'Saving...';
    statusEl.className = 'status';

    var body = {
      provider: providerSelect.value,
      listen: document.getElementById('listen').value,
      db: document.getElementById('db').value
    };

    if (body.provider === 'proxy') {
      var uid = document.getElementById('user_id_header').value;
      var email = document.getElementById('email_header').value;
      if (uid) body.user_id_header = uid;
      if (email) body.email_header = email;
    } else if (body.provider === 'cloudflare') {
      var td = document.getElementById('team_domain').value;
      var aud = document.getElementById('audience').value;
      if (td) body.team_domain = td;
      if (aud) body.audience = aud;
    }

    fetch('/setup/save', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify(body)
    })
    .then(function(resp) {
      if (!resp.ok) {
        return resp.text().then(function(t) { throw new Error(t); });
      }
      return resp.json();
    })
    .then(function(data) {
      statusEl.textContent = data.message || 'Saved! Restarting...';
      statusEl.className = 'status success';
      setTimeout(function() {
        statusEl.textContent = 'Waiting for restart...';
        pollForRestart();
      }, 2000);
    })
    .catch(function(err) {
      statusEl.textContent = 'Error: ' + err.message;
      statusEl.className = 'status error';
      saveBtn.disabled = false;
    });
  });

  function pollForRestart() {
    var attempts = 0;
    var maxAttempts = 30;
    function check() {
      attempts++;
      if (attempts > maxAttempts) {
        statusEl.textContent = 'Server may have restarted. Please reload the page.';
        saveBtn.disabled = false;
        return;
      }
      fetch('/', {method: 'HEAD'})
        .then(function(resp) {
          if (resp.status !== 503) {
            window.location.reload();
          } else {
            setTimeout(check, 1000);
          }
        })
        .catch(function() {
          setTimeout(check, 1000);
        });
    }
    setTimeout(check, 1000);
  }
})();
</script>
</body>
</html>`
