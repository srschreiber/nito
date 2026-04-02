#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/.."

CONFIGS_DIR="docker-configs"
ENV_FILE="$CONFIGS_DIR/.env"
BROKER_CONFIG="$CONFIGS_DIR/broker-config.yml"

mkdir -p "$CONFIGS_DIR"

# Generate .env once (contains the source-of-truth password)
if [ ! -f "$ENV_FILE" ]; then
    echo "Generating $ENV_FILE..."
    DB_PASSWORD=$(openssl rand -hex 32)
    cat > "$ENV_FILE" <<EOF
POSTGRES_USER=nito
POSTGRES_PASSWORD=$DB_PASSWORD
POSTGRES_DB=nito
EOF
fi

# Always regenerate broker configs from .env so they stay in sync
DB_PASSWORD=$(grep POSTGRES_PASSWORD "$ENV_FILE" | cut -d= -f2)

# Production config: broker uses host networking, so postgres is at localhost
cat > "$BROKER_CONFIG" <<EOF
broker:
  addr: "0.0.0.0:7070"
db:
  user: nito
  password: $DB_PASSWORD
  host: localhost
  port: "5432"
  name: nito
EOF

# Dev config: broker uses bridge networking, so postgres is at the service name
cat > "$CONFIGS_DIR/broker-config-dev.yml" <<EOF
broker:
  addr: "0.0.0.0:7070"
db:
  user: nito
  password: $DB_PASSWORD
  host: db
  port: "5432"
  name: nito
EOF

# Build broker image
echo "Building broker image..."
docker build -f broker/Dockerfile -t nito-broker:latest .

# Bring down any running services (preserves DB volume)
echo "Stopping existing services..."
docker compose down

# Start services
echo "Starting services..."
docker compose up -d

echo "Broker is running at :7070"
