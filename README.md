DNS Security Proxy (Go)

Production-ready DNS proxy with:

🔐 DNS inspection

⚡ Caching (Valkey)

📊 Prometheus metrics

🧠 External Cloud API enrichment

🐳 Docker-based deployment

🔧 YAML + ENV configuration

🛡 Upstream failover support

Architecture
Client
   ↓
DNS Proxy (Go)
   ↓
Valkey (cache)
   ↓
Upstream DNS (8.8.8.8 / 1.1.1.1 / etc)
   ↓
Cloud API (optional enrichment)


Additional components:

Prometheus metrics endpoint

Optional Grafana integration

Docker Compose orchestration

Requirements

Rocky Linux 10

Docker

Docker Compose (plugin)

Installation on Rocky Linux 10
1️⃣ Install Docker
sudo dnf install -y dnf-plugins-core
sudo dnf config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo
sudo dnf install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
sudo systemctl enable --now docker


Verify:

docker --version
docker compose version

2️⃣ Clone project
git clone https://github.com/your-org/dns-security-proxy.git
cd dns-security-proxy

3️⃣ Configure .env

Example:

# Required
CLOUD_API_KEY=YOUR_KEY

# Optional
CLOUD_API_URL=https://your-cloud/api/
VALKEY_PASSWORD=SecurePass123!
LOG_LEVEL=info
RATE_LIMIT_RPS=5

# DNS upstreams (comma-separated)
DNS_UPSTREAMS=8.8.8.8:53,1.1.1.1:53

4️⃣ (Optional) config.yaml

If present, YAML values are loaded first.
ENV variables override YAML.

Example:

dns:
  listen: ":53"
  upstreams:
    - "8.8.8.8:53"
    - "1.1.1.1:53"

http:
  listen: ":8080"

valkey:
  address: "valkey:6379"
  password: ""
  db: 0

cloud:
  api_url: ""
  api_key: ""

rate_limit_rps: 5
log_level: "info"

5️⃣ Start system
docker compose up -d --build


Check status:

docker compose ps

Testing the System
DNS test
dig @127.0.0.1 google.com


Expected: valid A records.

Test TLS DNS (if enabled)
dig @127.0.0.1 -p 853 google.com +tls

Health check
curl http://localhost:8080/health


Expected:

ok

Statistics
curl http://localhost:8080/stats | jq

Prometheus metrics
curl http://localhost:8080/metrics

Logs & Debugging
All containers
docker compose logs -f

Only DNS proxy
docker compose logs -f dns-proxy

Valkey
docker compose logs -f valkey

Inspect cache manually
docker exec -it valkey valkey-cli


Example:

keys *

Configuration Management
Priority order

Environment variables

config.yaml

Internal defaults

Where to change what
DNS Upstreams

.env

DNS_UPSTREAMS=8.8.8.8:53,1.1.1.1:53


or config.yaml:

dns:
  upstreams:
    - "8.8.8.8:53"

Rate limit

.env

RATE_LIMIT_RPS=10

Log level
LOG_LEVEL=debug

Valkey settings
VALKEY_PASSWORD=...


or in YAML.

How to Extend the Project
Add new ENV variable

Add field to Config struct

Load it in LoadConfig()

Add fallback default

No changes required in env.go unless new type is introduced.

Add new metric

Define Prometheus metric

Register it

Update in request flow

Add new DNS inspection rule

Modify:

dns_handler.go


Hook logic before forwarding to upstream.

Add new API integration

Extend:

cloud_client.go
