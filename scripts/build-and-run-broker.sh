#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/.."

ENV_FILE=".env"
BROKER_CONFIG="broker/config.yml"

# Generate credentials if .env doesn't exist
if [ ! -f "$ENV_FILE" ]; then
    echo "Generating $ENV_FILE..."
    DB_PASSWORD=$(openssl rand -hex 32)
    cat > "$ENV_FILE" <<EOF
POSTGRES_USER=nito
POSTGRES_PASSWORD=$DB_PASSWORD
POSTGRES_DB=nito
EOF
    echo "$ENV_FILE created."
fi

# Parse credentials from .env
DB_USER=$(grep POSTGRES_USER "$ENV_FILE" | cut -d= -f2)
DB_PASSWORD=$(grep POSTGRES_PASSWORD "$ENV_FILE" | cut -d= -f2)
DB_NAME=$(grep POSTGRES_DB "$ENV_FILE" | cut -d= -f2)

# Generate broker config if it doesn't exist
if [ ! -f "$BROKER_CONFIG" ]; then
    echo "Generating $BROKER_CONFIG..."
    cat > "$BROKER_CONFIG" <<EOF
broker:
  addr: "0.0.0.0:7070"
db:
  user: $DB_USER
  password: $DB_PASSWORD
  host: localhost
  port: "5432"
  name: $DB_NAME
EOF
    echo "$BROKER_CONFIG created."
fi

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