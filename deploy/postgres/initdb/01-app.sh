#!/bin/sh
# Chạy MỘT lần, khi volume pgdata còn trống. Muốn chạy lại: docker compose down -v
set -eu

APP_PW="$(tr -d '\r\n' < /run/secrets/pg_app_pw)"

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" \
  -v app_user="$APP_DB_USER" -v app_db="$APP_DB_NAME" -v app_pw="$APP_PW" <<'SQL'
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

CREATE ROLE :"app_user" LOGIN PASSWORD :'app_pw';

-- DB riêng cho app, app_user là owner => có toàn quyền trên schema public của DB
CREATE DATABASE :"app_db" OWNER :"app_user";
REVOKE ALL ON DATABASE :"app_db" FROM PUBLIC;

\connect :"app_db"
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
SQL
