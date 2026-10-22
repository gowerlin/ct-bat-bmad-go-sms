#!/usr/bin/env bash
# sms-platform dev container without the compose plugin (T0193; repo-root
# mount since T0194 / D116 ①).
# Plain `docker` equivalent of compose.yaml; run it INSIDE the WSL distro that
# owns the Docker daemon (D113). From Windows use scripts/dev-container.ps1.
#
#   scripts/dev-container.sh build   # build sms-platform-dev:latest (needs docker buildx)
#   scripts/dev-container.sh up      # create+start container (builds if image missing)
#   scripts/dev-container.sh start | stop | down | status | shell | test | logs
#
# Mount: the COURSE REPO ROOT (sms-platform/../../..) -> /workspace/<root dir name>,
# so .git is inside the mount and work orders can be committed from the
# container. shell/test/logs run in the sms-platform/ sub-directory.
# Students mounting their own repo: SMS_REPO_ROOT=<your repo root>.
#
# Env overrides: SMS_IMAGE, SMS_CONTAINER, SMS_PORT, SMS_CT_SKILLS,
# SMS_PLATFORM_DIR (build context), SMS_REPO_ROOT (mount source).
# (prefixed on purpose: WSL exports NAME=<hostname> into every shell.)
set -euo pipefail

HERE=${SMS_PLATFORM_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}
REPO_ROOT=${SMS_REPO_ROOT:-$(cd "$HERE/../../.." && pwd)}
IMAGE=${SMS_IMAGE:-sms-platform-dev:latest}
NAME=${SMS_CONTAINER:-bat-server-sms-platform}
PORT=${SMS_PORT:-9880}
CT_SKILLS=${SMS_CT_SKILLS:-$HERE/../../../BMad-Control-Tower/BMad-Control-Tower-v5.12.2/skills}
DATA_VOLUME="${NAME}-data"
# Container paths: /workspace/<repo root dir name>/<sms-platform relative to root>
MOUNT_DIR="/workspace/$(basename "$REPO_ROOT")"
WORK_DIR="$MOUNT_DIR/$(realpath --relative-to="$REPO_ROOT" "$HERE")"

build() {
  [ -d "$CT_SKILLS" ] || { echo "CT skills dir not found: $CT_SKILLS" >&2; exit 1; }
  DOCKER_BUILDKIT=1 docker build \
    --build-context "ct-skills=$CT_SKILLS" \
    --build-arg BAT_IMAGE=bat-server:latest \
    -t "$IMAGE" "$HERE"
}

up() {
  docker image inspect "$IMAGE" >/dev/null 2>&1 || build
  if docker container inspect "$NAME" >/dev/null 2>&1; then
    docker start "$NAME" >/dev/null
  else
    # Same shape as docker-deployment.md § Dev Container Integration and as
    # compose.yaml: loopback publish, one bind mount (repo root), named data
    # volume, working dir in sms-platform/, entrypoint inherited from bat-server.
    docker run -d \
      --name "$NAME" \
      --restart unless-stopped \
      -p "127.0.0.1:${PORT}:9876" \
      -v "$REPO_ROOT:$MOUNT_DIR" \
      -v "${DATA_VOLUME}:/root/.local/share/bat-server" \
      -w "$WORK_DIR" \
      "$IMAGE" >/dev/null
  fi
  status
}

status() {
  echo "mount: $REPO_ROOT -> $MOUNT_DIR   workdir: $WORK_DIR"
  docker ps -a --filter "name=^/${NAME}$" --format 'table {{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}'
  docker port "$NAME" 2>/dev/null || true
  docker inspect --format 'health: {{if .State.Health}}{{.State.Health.Status}}{{else}}n/a{{end}}' "$NAME" 2>/dev/null || true
}

case "${1:-up}" in
  build)  build ;;
  up)     up ;;
  start)  docker start "$NAME" && status ;;
  stop)   docker stop "$NAME" ;;
  down)   docker rm -f "$NAME" ;;   # keeps the data volume (token + cert)
  status) status ;;
  shell)  docker exec -it -w "$WORK_DIR" "$NAME" bash ;;
  logs)   docker logs --tail 50 "$NAME" ;;
  test)   docker exec -w "$WORK_DIR" "$NAME" sh -lc 'pwd && go version && go test ./... -race' ;;
  *) echo "usage: $0 {build|up|start|stop|down|status|shell|logs|test}" >&2; exit 2 ;;
esac
