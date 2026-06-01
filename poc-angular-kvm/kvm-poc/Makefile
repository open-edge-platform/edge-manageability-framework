ANGULAR_DIR = kvm-angular-app
SERVER_DIR  = server
BINARY      = $(SERVER_DIR)/kvm_server

.PHONY: build run clean

## build: compile Angular into server/static/ then embed into the Go binary
## The resulting binary is self-contained — no Node.js or npm needed to run it.
build:
	@echo "→ Building Angular app..."
	cd $(ANGULAR_DIR) && npm run build
	@echo "→ Building Go binary with embedded Angular UI..."
	cd $(SERVER_DIR) && go build -o kvm_server .
	@echo ""
	@echo "✓ Done: $(BINARY)"
	@echo "  Run:  ./$(BINARY)"
	@echo "  The browser will open automatically."

## run: build then launch — browser opens automatically
run: build
	./$(BINARY)

## clean: remove the compiled binary
clean:
	rm -f $(BINARY)
