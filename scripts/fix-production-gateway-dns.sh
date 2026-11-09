#!/usr/bin/env bash
# One-time, reversible correction for Docker Gateway IP changes in Nginx 1.28.
# Run on EC2 only after reviewing the exact config and authorizing the change.
set -euo pipefail

mode="${1:-}"
config=/opt/osmi/nginx/nginx.conf
backup=/opt/osmi/deploy/nginx.conf.before-gateway-resolve-20261108

if [[ "$mode" != --check && "$mode" != --apply ]]; then
  echo 'Uso: sudo bash fix-production-gateway-dns.sh --check|--apply' >&2
  exit 2
fi
[[ $EUID -eq 0 ]] || { echo ROOT_REQUERIDO >&2; exit 1; }
[[ -f "$config" ]] || { echo CONFIG_FALTA >&2; exit 1; }

state="$(python3 - "$config" <<'PY'
from pathlib import Path
import sys
data = Path(sys.argv[1]).read_bytes()
old = b'        server gateway:8083'
new = b'        server gateway:8083 resolve'
zone = b'        zone osmi_gateway 64k;'
if b'resolver 127.0.0.11' not in data or data.count(b'    upstream osmi_gateway {') != 1:
    print('UNEXPECTED')
elif data.count(new) == 1 and data.count(zone) == 1:
    print('APPLIED')
elif data.count(old) == 1 and data.count(new) == 0 and data.count(zone) == 0:
    print('READY')
else:
    print('UNEXPECTED')
PY
)"
if [[ "$mode" == --check ]]; then
  echo "NGINX_GATEWAY_DNS=$state"
  [[ "$state" != UNEXPECTED ]]
  exit
fi
[[ "$state" == READY ]] || { echo "REFUSING_CONFIG_STATE=$state" >&2; exit 1; }
[[ ! -e "$backup" ]] || { echo BACKUP_ALREADY_EXISTS >&2; exit 1; }
docker exec osmi-nginx nginx -t >/dev/null 2>&1 || { echo NGINX_PRECHECK_FAILED >&2; exit 1; }

restore() {
  if [[ ! -f "$backup" ]]; then
    echo BACKUP_MISSING_DURING_ROLLBACK >&2
    return
  fi
  python3 - "$config" "$backup" <<'PY'
from pathlib import Path
import sys
source, backup = map(Path, sys.argv[1:])
with source.open('wb') as out:
    out.write(backup.read_bytes())
PY
  docker exec osmi-nginx nginx -t >/dev/null 2>&1 && docker exec osmi-nginx nginx -s reload >/dev/null 2>&1 || true
  echo CONFIG_RESTORED_AFTER_FAILURE >&2
}
trap restore EXIT

# Preserve the host file inode because Docker bind-mounts that file.
python3 - "$config" "$backup" <<'PY'
from pathlib import Path
import os, sys
source, backup = map(Path, sys.argv[1:])
data = source.read_bytes()
newline = b'\r\n' if b'\r\n' in data else b'\n'
upstream = b'    upstream osmi_gateway {' + newline
server = b'        server gateway:8083' + newline
if data.count(upstream) != 1 or data.count(server) != 1:
    raise SystemExit('UNEXPECTED_CONFIG_CONTENT')
os.umask(0o077)
with backup.open('xb') as out:
    out.write(data)
patched = data.replace(upstream, upstream + newline + b'        zone osmi_gateway 64k;' + newline, 1)
patched = patched.replace(server, b'        server gateway:8083 resolve' + newline, 1)
with source.open('wb') as out:
    out.write(patched)
PY
docker exec osmi-nginx nginx -t >/dev/null 2>&1 || { echo NGINX_CANDIDATE_INVALID >&2; exit 1; }
docker exec osmi-nginx nginx -s reload >/dev/null 2>&1 || { echo NGINX_RELOAD_FAILED >&2; exit 1; }
code="$(curl --noproxy '*' --silent --show-error --output /dev/null --write-out '%{http_code}' --resolve api.myosmi.com:443:127.0.0.1 https://api.myosmi.com/v1/events)" || { echo ORIGIN_CHECK_FAILED >&2; exit 1; }
[[ "$code" == 200 ]] || { echo "ORIGIN_HTTP=$code" >&2; exit 1; }
trap - EXIT
echo NGINX_GATEWAY_DNS_FIXED_AND_ORIGIN_200
