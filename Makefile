# Windows: chạy recipe bằng sh.exe của Git for Windows thay cho cmd.exe.
# Thêm Git/bin, Git/usr/bin vào PATH vì lệnh đơn giản được make gọi thẳng, không qua SHELL
ifeq ($(MAKE_HOST),Windows32)
empty :=
space := $(empty) $(empty)
GIT_ROOT := $(shell git --exec-path)/../../..
ifneq ($(wildcard $(subst $(space),\ ,$(GIT_ROOT)/bin/sh.exe)),)
SHELL := $(GIT_ROOT)/bin/sh.exe
export PATH := $(PATH);$(GIT_ROOT)/bin;$(GIT_ROOT)/usr/bin
# Không cho sh.exe đổi /src thành C:/Program Files/Git/src khi gọi docker
export MSYS_NO_PATHCONV := 1
else
$(warning Không tìm thấy sh.exe của Git for Windows, recipe sẽ chạy bằng cmd.exe => cài Git for Windows hoặc chạy make trong Git Bash)
endif
endif

# Makefile của monorepo: stack Docker (compose.yml ở đây), database, và gọi
# sang backend/Makefile (Go) và frontend (npm). Lệnh riêng của Go: make -C backend <target>

# APP_DB_NAME, APP_DB_USER, PGADMIN_PORT... lấy từ .env nếu có
-include .env

APP_DB_NAME  ?= storeit
APP_DB_USER  ?= app_user
PGADMIN_PORT ?= 5050
WEB_PORT     ?= 3000

PG_SUPERUSER := $(strip $(file < deploy/postgres/secrets/pg_user.txt))

COMPOSE := docker compose
PSQL    := $(COMPOSE) exec postgres psql

# Tag và registry cho image production (make images)
TAG      ?= dev
REGISTRY ?= storeit

.DEFAULT_GOAL := help
.PHONY: help init dev \
        up down restart build logs logs-app logs-web logs-db ps shell \
        psql psql-admin db-reset pgadmin \
        generate check images

help:
	@echo Prerequisite: init
	@echo Dev         : dev = up + logs   "(web http://localhost:$(WEB_PORT), API http://localhost:8080/api/docs)"
	@echo Docker      : up down restart build logs logs-app logs-web logs-db ps shell
	@echo DB          : psql psql-admin pgadmin db-reset CONFIRM=yes
	@echo Code        : generate check
	@echo Release     : images TAG=1.0.0 REGISTRY=registry.example.com
	@echo Go only     : make -C backend help

# --- Prerequisite ---
# Chạy một lần sau khi clone: tạo secrets, cert và file env
init:
	sh deploy/scripts/init.sh
	@test -f .env               || cp .env.example .env
	@test -f backend/.env.local || cp backend/.env.local.example backend/.env.local

# --- Dev ---
# Bật cả stack rồi xem log. Ctrl-C chỉ thoát xem log, stack vẫn chạy
dev: up logs

# --- Docker ---
up:
	$(COMPOSE) up -d

down:
	$(COMPOSE) down

restart:
	$(COMPOSE) restart app worker web

# Build lại image dev khi đổi Dockerfile hoặc go.mod
build:
	$(COMPOSE) build app web

logs:
	$(COMPOSE) logs -f

logs-app:
	$(COMPOSE) logs -f app

logs-web:
	$(COMPOSE) logs -f web

logs-db:
	$(COMPOSE) logs -f postgres

ps:
	$(COMPOSE) ps

shell:
	$(COMPOSE) exec app sh

# --- Database ---
# Vào qua unix socket trong container nên không cần mật khẩu
psql:
	$(PSQL) -U $(APP_DB_USER) -d $(APP_DB_NAME)

psql-admin:
	$(PSQL) -U $(PG_SUPERUSER) -d postgres

pgadmin:
	$(COMPOSE) up -d pgadmin
	@echo pgAdmin: http://127.0.0.1:$(PGADMIN_PORT)

# XOÁ dữ liệu Postgres và pgAdmin (giữ cache Go, node_modules), initdb chạy lại từ đầu
db-reset:
ifneq ($(CONFIRM),yes)
	$(error db-reset will erase all data. To run: make db-reset CONFIRM=yes)
endif
	$(COMPOSE) down
	docker volume rm -f storeit_pgdata storeit_pgadmin
	$(COMPOSE) up -d

# --- Code ---
# Sinh code backend (sqlc, oapi-codegen) rồi type của frontend từ OpenAPI
generate:
	$(MAKE) -C backend generate
	cd frontend && npm run gen:api

# Chạy trước khi commit
check:
	$(MAKE) -C backend check
	cd frontend && npm run check

# --- Release ---
# Image production: API/worker/migrate (backend) và web (nginx + SPA)
images:
	docker build --target prod -t $(REGISTRY)/storeit:$(TAG) backend
	docker build --target prod -t $(REGISTRY)/storeit-web:$(TAG) frontend
