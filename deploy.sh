#!/bin/bash
set -e

sudo cp Caddyfile /etc/caddy/sites/cookd-api.caddy
sudo systemctl reload caddy

docker compose up -d --build
docker image prune -f
