#!/bin/sh
# entrypoint.sh — substitute env vars in config, then start server.
# Migrations run automatically in boot.RunMigrations before servers start.
set -e

# Substitute env vars into Docker config
if [ -f /app/manifest/config/config.docker.yaml ]; then
  echo "[entrypoint] substituting env vars in config..."
  envsubst < /app/manifest/config/config.docker.yaml > /app/manifest/config/config.yaml
fi

echo "[entrypoint] starting server..."
exec /app/server "$@"
