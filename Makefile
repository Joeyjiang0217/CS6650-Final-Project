PROTOC      ?= protoc
PROTO_ROOT  := api/proto
PROTO_OUT   := api/gen

PROTO_FILES := \
	chatroom/v1/common.proto \
	chatroom/v1/chat.proto \
	chatroom/v1/message.proto \
	chatroom/v1/transmit.proto

.PHONY: proto-tools proto proto-clean proto-check

proto-tools:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@echo "Make sure $$GOBIN or $$(go env GOPATH)/bin is in your PATH"

proto-check:
	$(PROTOC) --version
	@command -v protoc-gen-go >/dev/null 2>&1 || (echo "protoc-gen-go not found in PATH" && exit 1)
	@command -v protoc-gen-go-grpc >/dev/null 2>&1 || (echo "protoc-gen-go-grpc not found in PATH" && exit 1)

proto:
	mkdir -p $(PROTO_OUT)
	$(PROTOC) -I $(PROTO_ROOT) \
		--go_out=$(PROTO_OUT) \
		--go_opt=paths=source_relative \
		--go-grpc_out=$(PROTO_OUT) \
		--go-grpc_opt=paths=source_relative \
		$(PROTO_FILES)

proto-clean:
	rm -rf $(PROTO_OUT)/chatroom