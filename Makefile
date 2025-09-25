PROTOC ?= protoc
PROTO_DIR := proto
GEN_DIR := protoBuilds/membership

.PHONY: proto tidy

proto:
	@mkdir -p $(GEN_DIR)
	$(PROTOC) \
		--go_out=$(GEN_DIR) --go_opt=paths=source_relative \
		--go-grpc_out=$(GEN_DIR) --go-grpc_opt=paths=source_relative \
		-I $(PROTO_DIR) \
		$(PROTO_DIR)/*.proto

tidy:
	go mod tidy

build:
	go build -o daemon ./cmd/daemon


