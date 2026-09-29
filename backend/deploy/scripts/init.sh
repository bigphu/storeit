#!/bin/sh
# Tạo secrets + TLS cert cho dev, file đã có thì bỏ qua
set -eu
cd "$(dirname "$0")/../.."          # về backend/
export MSYS_NO_PATHCONV=1           # Git Bash: không đổi "/CN=..." thành đường dẫn Windows

secret() {                          # secret <file> [giá trị cố định]
  [ -s "$1" ] && return 0
  mkdir -p "$(dirname "$1")"
  if [ $# -ge 2 ]; then printf '%s' "$2" > "$1"
  else openssl rand -hex 32 | tr -d '\r\n' > "$1"; fi   # openssl trên Windows in ra CRLF
  echo "created $1"
}

secret deploy/postgres/secrets/pg_user.txt postgres
secret deploy/postgres/secrets/pg_pw.txt
secret deploy/postgres/secrets/pg_app_pw.txt
secret deploy/pgadmin/secrets/pgadmin_pw.txt
secret deploy/app/secrets/jwt_keys.txt "v1:$(openssl rand -base64 32)"   # keyring kid:base64, 32 byte

C=deploy/postgres/certs
if [ ! -s "$C/server.crt" ]; then
  mkdir -p "$C"
  openssl req -x509 -newkey rsa:2048 -nodes -days 3650 -subj "/CN=storeit-dev-ca" \
    -addext "basicConstraints=critical,CA:TRUE" -addext "keyUsage=critical,keyCertSign,cRLSign" \
    -keyout "$C/rootCA.key" -out "$C/rootCA.pem"
  openssl req -newkey rsa:2048 -nodes -subj "/CN=postgres" \
    -keyout "$C/server.key" -out "$C/server.csr"
  printf 'subjectAltName=DNS:localhost,DNS:postgres,IP:127.0.0.1\nextendedKeyUsage=serverAuth\n' > "$C/san.ext"
  openssl x509 -req -sha256 -days 825 -in "$C/server.csr" \
    -CA "$C/rootCA.pem" -CAkey "$C/rootCA.key" -CAcreateserial \
    -extfile "$C/san.ext" -out "$C/server.crt"
  rm -f "$C/server.csr" "$C/san.ext" "$C/rootCA.srl"
  echo "created TLS certs in $C"
fi
