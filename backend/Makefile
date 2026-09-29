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

# APP_DB_NAME, APP_DB_USER, PGADMIN_PORT... lấy từ .env nếu có
-include .env

APP_DB_NAME  ?= storeit
APP_DB_USER  ?= app_user
PGADMIN_PORT ?= 5050

PG_SUPERUSER := $(strip $(file < deploy/postgres/secrets/pg_user.txt))

# Env cho Go chạy trên host (run-api, test...): chỉ export biến do .env.local thêm vào
VARS_BEFORE_LOCAL := $(.VARIABLES)
-include .env.local
LOCAL_ENV_KEYS := $(filter-out $(VARS_BEFORE_LOCAL) VARS_BEFORE_LOCAL,$(.VARIABLES))
export $(LOCAL_ENV_KEYS)

COMPOSE := docker compose
PSQL    := $(COMPOSE) exec postgres psql

# Version của tool chạy bằng Docker, CI cũng dùng
SQLC_VERSION     := 1.31.1
GOLANGCI_VERSION := v2.14.0

.DEFAULT_GOAL := help
.PHONY: help init dev \
        up down restart build logs logs-app logs-db ps shell \
        psql psql-admin db-reset pgadmin \
        migrate migrate-new \
        sqlc sqlc-vet sqlc-compile generate generate-check \
        run-api run-worker bin test fmt fmt-check vet lint tidy check

help:
	@echo Prerequisite: init
	@echo Dev         : dev = up + logs
	@echo Docker      : up down restart build logs logs-app logs-db ps shell
	@echo DB          : psql psql-admin pgadmin db-reset CONFIRM=yes
	@echo Migrations  : migrate migrate-new name=...
	@echo Codegen     : generate generate-check sqlc sqlc-vet sqlc-compile
	@echo Go          : run-api run-worker bin test fmt fmt-check vet lint tidy check

# --- Prerequisite ---
# Chạy một lần sau khi clone: tạo secrets, cert và file env
init:
	sh deploy/scripts/init.sh
	@test -f .env       || cp .env.example .env
	@test -f .env.local || cp .env.local.example .env.local

# --- Dev ---
# Bật cả stack rồi xem log. Ctrl-C chỉ thoát xem log, stack vẫn chạy
dev: up logs

# --- Docker ---
up:
	$(COMPOSE) up -d

down:
	$(COMPOSE) down

restart:
	$(COMPOSE) restart app

# Build lại image dev khi đổi Dockerfile hoặc go.mod
build:
	$(COMPOSE) build app

logs:
	$(COMPOSE) logs -f

logs-app:
	$(COMPOSE) logs -f app

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

# XOÁ dữ liệu Postgres và pgAdmin (giữ cache Go), initdb chạy lại từ đầu
db-reset:
ifneq ($(CONFIRM),yes)
	$(error db-reset will erase all data. To run: make db-reset CONFIRM=yes)
endif
	$(COMPOSE) down
	docker volume rm -f storeit_pgdata storeit_pgadmin
	$(COMPOSE) up -d

# --- Migrations ---
# Chạy trên host với .env.local. Trong Docker thì `make up` tự chạy migrate
migrate:
	go run ./cmd/migrate

# Tạo file migration mới, đặt tên <module>_<thay đổi>
migrate-new:
ifndef name
	$(error usage: make migrate-new name=identity_accounts)
endif
	go tool goose -s -dir migrations create $(name) sql

# --- Codegen ---
# Sinh code từ openapi.yaml và query .sql. Code sinh ra được commit
generate: sqlc
	go generate ./...

# Cho CI: sinh lại code, fail nếu có gì khác bản đã commit
generate-check: generate
	@test -z "$$(git status --porcelain -- .)" || (echo "code sinh ra chưa cập nhật, chạy make generate rồi commit:"; git status --porcelain -- .; exit 1)

# --- sqlc (chạy bằng Docker) ---
# Chưa có query nào thì sqlc báo lỗi, nên bỏ qua
SQLC_QUERIES := $(wildcard internal/*/repository/queries/*.sql internal/platform/events/queries/*.sql)
SQLC := docker run --rm -v "$(CURDIR):/src" -w /src sqlc/sqlc:$(SQLC_VERSION)

sqlc sqlc-vet sqlc-compile:
ifeq ($(SQLC_QUERIES),)
	@echo "$@: chưa có query .sql nào, bỏ qua"
else
	$(SQLC) $(patsubst sqlc,generate,$(patsubst sqlc-%,%,$@))
endif

# --- Go (chạy trên host) ---
run-api:
	go run ./cmd/server

run-worker:
	go run ./cmd/worker

# Build server, worker, migrate vào bin/
bin:
	go build -o bin/ ./cmd/...

test:
	go test ./...

fmt:
	go fmt ./...

# Chỉ kiểm tra, không sửa file
fmt-check:
	@test -z "$$(gofmt -l .)" || (echo "chưa gofmt:"; gofmt -l .; exit 1)

vet:
	go vet ./...

# golangci-lint bằng Docker, config ở .golangci.yml. Volume giữ cache cho lần sau nhanh
lint:
	docker run --rm -v "$(CURDIR):/app" -w /app \
		-v storeit_gomod:/go/pkg/mod -v storeit_golangci:/root/.cache \
		golangci/golangci-lint:$(GOLANGCI_VERSION) golangci-lint run

tidy:
	go mod tidy

# Chạy trước khi commit
check: sqlc-compile fmt-check vet lint test
