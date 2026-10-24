#!/usr/bin/env bash
# sms-platform dev container, with or without the compose plugin.
# Plain `docker` equivalent of compose.yaml; run it INSIDE the WSL distro that
# owns the Docker daemon. From Windows use scripts/dev-container.ps1.
#
#   scripts/dev-container.sh build   # build sms-platform-dev:latest (needs docker buildx)
#   scripts/dev-container.sh up      # create+start container (builds if image missing)
#   scripts/dev-container.sh start | stop | down | status | shell | test | logs
#   scripts/dev-container.sh env     # print the SMS_* values for compose (… env >> .env)
#   scripts/dev-container.sh compose <args>   # docker compose with those values
#
# Mount: the git repo root that contains sms-platform/ -> /workspace/<root dir
# name>, so .git is inside the mount and work orders can be committed from the
# container. In the public repo that is sms-platform/ itself; when sms-platform
# sits deeper (the course production repo), the root is found with git.
# shell/test/logs run in the sms-platform/ directory.
#
# Env overrides (also read from SMS_* lines in .env next to compose.yaml; the
# environment wins): SMS_REPO_ROOT (mount source), SMS_REPO_NAME (container
# folder name), SMS_CT_SKILLS (control-tower/ + ct-*/ to bake in; default none),
# SMS_IMAGE, SMS_CONTAINER, SMS_PORT, SMS_PLATFORM_DIR (build context).
# SMS_SUBDIR is derived from the mount; .env may carry it for compose.
# (prefixed on purpose: WSL exports NAME=<hostname> into every shell.)
set -euo pipefail

HERE=${SMS_PLATFORM_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}

# SMS_* from .env, without overriding the environment. Values are taken
# literally; surrounding quotes are dropped.
if [ -f "$HERE/.env" ]; then
  while IFS= read -r line || [ -n "$line" ]; do
    line=${line%$'\r'}
    case "$line" in
      SMS_*=*)
        key=${line%%=*}; val=${line#*=}
        val=${val#\"}; val=${val%\"}; val=${val#\'}; val=${val%\'}
        [ -n "${!key+x}" ] || export "$key=$val"
        ;;
    esac
  done < "$HERE/.env"
fi

# Relative paths are relative to sms-platform/ (where compose.yaml is), as in compose.
abs() { case "$1" in /*) (cd "$1" && pwd) ;; *) (cd "$HERE/$1" && pwd) ;; esac; }

if [ -n "${SMS_REPO_ROOT:-}" ]; then
  REPO_ROOT=$(abs "$SMS_REPO_ROOT")
else
  REPO_ROOT=$(git -C "$HERE" rev-parse --show-toplevel 2>/dev/null) || REPO_ROOT=$HERE
fi
IMAGE=${SMS_IMAGE:-sms-platform-dev:latest}
NAME=${SMS_CONTAINER:-bat-server-sms-platform}
PORT=${SMS_PORT:-9880}
CT_SKILLS=""
[ -z "${SMS_CT_SKILLS:-}" ] || CT_SKILLS=$(abs "$SMS_CT_SKILLS") \
  || { echo "SMS_CT_SKILLS not found: $SMS_CT_SKILLS" >&2; exit 1; }
DATA_VOLUME="${NAME}-data"
# Container paths: /workspace/<repo root dir name>/<sms-platform relative to root>
REPO_NAME=${SMS_REPO_NAME:-$(basename "$REPO_ROOT")}
REL=$(realpath --relative-to="$REPO_ROOT" "$HERE")
SUBDIR=""
[ "$REL" = . ] || SUBDIR="/$REL"
MOUNT_DIR="/workspace/$REPO_NAME"
WORK_DIR="$MOUNT_DIR$SUBDIR"

build() {
  local ctx=()
  if [ -n "$CT_SKILLS" ]; then
    ctx=(--build-context "ct-skills=$CT_SKILLS")
  else
    echo "SMS_CT_SKILLS not set: building without Control Tower skills" >&2
  fi
  DOCKER_BUILDKIT=1 docker build \
    "${ctx[@]}" \
    --build-arg BAT_IMAGE=bat-server:latest \
    --build-arg "SMS_WORKDIR=$WORK_DIR" \
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

# compose.yaml's variables for this layout; paths relative to sms-platform/.
env_lines() {
  echo "SMS_REPO_ROOT=$(realpath --relative-to="$HERE" "$REPO_ROOT")"
  echo "SMS_REPO_NAME=$REPO_NAME"
  echo "SMS_SUBDIR=$SUBDIR"
  [ -z "$CT_SKILLS" ] || echo "SMS_CT_SKILLS=$(realpath --relative-to="$HERE" "$CT_SKILLS")"
  [ -z "${SMS_IMAGE:-}" ] || echo "SMS_IMAGE=$SMS_IMAGE"
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
  env)    env_lines ;;
  compose)
    shift
    # The values above go to compose through the environment (it wins over .env).
    while IFS= read -r line; do export "${line%%=*}=${line#*=}"; done < <(env_lines)
    [ -z "$CT_SKILLS" ] || echo "ct-skills: $CT_SKILLS" >&2
    docker compose --project-directory "$HERE" -f "$HERE/compose.yaml" "$@"
    ;;
  *) echo "usage: $0 {build|up|start|stop|down|status|shell|logs|test|env|compose <args>}" >&2; exit 2 ;;
esac
