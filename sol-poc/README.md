# SOL-POC-NEW — AMT Serial-over-LAN Server

Go HTTP/WebSocket server that connects to an AMT device via MPS and exposes an interactive SOL terminal over WebSocket.

## Prerequisites

- Go 1.25+ (via asdf or system)
- Access to MPS endpoint and a valid Keycloak JWT token
- AMT device password (from Vault)

## Build

```bash
cd /home/seu/edge-manageability-framework/SOL-POC-NEW
go build -o sol_server sol_server.go
```

## Start Server

```bash
./sol_server -listen :9090
```

Server starts on port 9090. Verify with:

```bash
curl -s http://localhost:9090/api/status
```

## Stop Server

```bash
pkill -f './sol_server'
```

## Connect to AMT SOL Session

### Step 1 — Set Environment Variables

```bash
export JWT_TOKEN="<your-fresh-keycloak-jwt-token>"
export AMT_PASSWORD=$(kubectl exec -n orch-platform vault-0 -- vault kv get -field=password secret/amt-password)
```

### Step 2 — Start SOL Session

```bash
curl -s -X POST http://localhost:9090/api/connect \
  -H 'Content-Type: application/json' \
  -d "{\"jwtToken\":\"$JWT_TOKEN\", \"amtPass\":\"$AMT_PASSWORD\"}"
```

Optional fields in the JSON body (defaults shown):

| Field        | Default                                              |
|--------------|------------------------------------------------------|
| `mpsHost`    | `mps-wss.orch-10-139-218-35.pid.infra-host.com`     |
| `deviceGuid` | `89174ecf-31c3-22e3-5f8d-48210b509c73`              |
| `amtUser`    | `admin`                                              |
| `port`       | `16994`                                              |
| `mode`       | `sol`                                                |

### Step 3 — Connect Interactive Terminal via wssh3

```bash
wssh3 ws://localhost:9090/ws/terminal
```

Or using websocat:

```bash
websocat ws://localhost:9090/ws/terminal
```

Type commands and press Enter. Use `Ctrl+C` to disconnect the terminal client.

### Step 4 — Check Status

```bash
curl -s http://localhost:9090/api/status
```

### Step 5 — Disconnect SOL Session

```bash
curl -s -X POST http://localhost:9090/api/disconnect
```

---

## Install wssh3

```bash
# Install system dependencies
sudo apt install -y libevent-dev python3 python3-pip

# Upgrade pip
sudo /usr/bin/pip3 install --upgrade pip

# Install gevent
sudo /usr/bin/pip3 install gevent

# Clone wssh3
cd ~
git clone https://github.com/Tectract/wssh3.git

# Install ws4py_modified
cd ~/wssh3/ws4py_modified
sudo /usr/bin/pip3 install .

# Install wssh3
cd ~/wssh3
sudo /usr/bin/pip3 install .

# Fix zope namespace issue (Ubuntu 22.04)
sudo cp -r /usr/local/lib/python3.10/dist-packages/zope/event /usr/lib/python3/dist-packages/zope/event

# Verify installation
wssh3 -h
```

## Install websocat (alternative)

```bash
curl -L -o /tmp/websocat https://github.com/vi/websocat/releases/download/v1.13.0/websocat.x86_64-unknown-linux-musl
chmod +x /tmp/websocat
sudo mv /tmp/websocat /usr/local/bin/websocat
websocat --version
```

## API Reference

| Endpoint            | Method | Description                              |
|---------------------|--------|------------------------------------------|
| `/api/connect`      | POST   | Start MPS→AMT SOL session (JSON body)    |
| `/api/disconnect`   | POST   | Tear down active SOL session             |
| `/api/status`       | GET    | Session state + recent log messages      |
| `/ws/terminal`      | WS     | Interactive terminal WebSocket endpoint   |
