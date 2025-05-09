proto:
	protoc -I=./internal/proto --go_out=./internal/proto --go-grpc_out=./internal/proto internal/proto/subpub.proto

install-tools:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
