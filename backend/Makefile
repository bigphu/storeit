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

# Lệnh Go của backend. Stack Docker, database, init: Makefile ở thư mục gốc

# Env cho Go chạy trên host (run-api, test...): chỉ export biến do .env.local thêm vào
VARS_BEFORE_LOCAL := $(.VARIABLES)
-include .env.local
LOCAL_ENV_KEYS := $(filter-out $(VARS_BEFORE_LOCAL) VARS_BEFORE_LOCAL,$(.VARIABLES))
# Đường dẫn file trong .env.local tính từ backend/; go test chạy mỗi package trong thư
# mục của nó, nên đổi sang đường dẫn tuyệt đối trước khi export
PATH_KEYS := $(filter %_FILE PGSSLROOTCERT,$(LOCAL_ENV_KEYS))
$(foreach k,$(PATH_KEYS),$(if $($(k)),$(eval $(k) := $(abspath $($(k))))))
export $(LOCAL_ENV_KEYS)

# Version của tool chạy bằng Docker, CI cũng dùng
SQLC_VERSION     := 1.31.1
GOLANGCI_VERSION := v2.14.0

.DEFAULT_GOAL := help
.PHONY: help migrate migrate-new \
        sqlc sqlc-vet sqlc-compile generate generate-check \
        run-api run-worker bin test fmt fmt-check vet lint tidy check

help:
	@echo Migrations  : migrate migrate-new name=...
	@echo Codegen     : generate generate-check sqlc sqlc-vet sqlc-compile
	@echo Go          : run-api run-worker bin test fmt fmt-check vet lint tidy check
	@echo Docker, DB  : make -C .. help

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
