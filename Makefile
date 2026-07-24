LDFLAGS := -ldflags="-s -w"

release:
	mkdir -p releases
	GOOS=linux   GOARCH=amd64 go build $(LDFLAGS) -o releases/mephisto-linux-amd64   ./cmd/mephisto
	GOOS=linux   GOARCH=arm64 go build $(LDFLAGS) -o releases/mephisto-linux-arm64   ./cmd/mephisto
	GOOS=darwin  GOARCH=amd64 go build $(LDFLAGS) -o releases/mephisto-darwin-amd64  ./cmd/mephisto
	GOOS=darwin  GOARCH=arm64 go build $(LDFLAGS) -o releases/mephisto-darwin-arm64  ./cmd/mephisto
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o releases/mephisto-windows-amd64.exe ./cmd/mephisto
	@echo "✅ 所有平台构建完成"