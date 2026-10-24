# syntax=docker/dockerfile:1.7
# sms-platform dev container (course unit 4-09).
#
# Base image: the BAT server image you build yourself from the BAT source
#   (cd better-agent-terminal && npm ci && npm run build:docker-image
#    → bat-server:<version> and bat-server:latest; nothing is published).
# It already carries /opt/bat-server (server, bundled node, bundled claude),
# EXPOSE 9876, the TLS HEALTHCHECK and the ENTRYPOINT
#   tini -- /opt/bat-server/bin/bat-server --bind-interface all
# We inherit all of that unchanged so BAT's Docker wizard can attach with
# "Use existing" (docker-deployment.md § Dev Container Integration).
#
# Build needs BuildKit (docker buildx). The Control Tower skills are optional
# and come in as the named context `ct-skills`, which replaces the empty stage
# of the same name below:
#   docker compose build dev                      (SMS_CT_SKILLS in compose.yaml)
#   docker build --build-context ct-skills=<dir> -t sms-platform-dev .
#   docker build -t sms-platform-dev .            (no CT skills baked in)
ARG BAT_IMAGE=bat-server:latest

# Default for the named context: nothing. --build-context ct-skills=<dir>
# overrides this stage.
FROM scratch AS ct-skills

FROM ${BAT_IMAGE}

# Go toolchain: same version as the recording host (go.mod `go 1.27.3`).
ARG GO_VERSION=1.27.3
ARG GO_SHA256=65efbc80fdc1fa2cd2631cad5b481777a68e8725f34e89f50fc6ac714e379b40

# git/make: work orders commit (the mount includes .git) and run `make test`
# inside the container.
# gcc + libc6-dev: `go test -race` needs cgo.
RUN apt-get update \
 && apt-get install -y --no-install-recommends git make gcc libc6-dev ca-certificates \
 && rm -rf /var/lib/apt/lists/*

RUN curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" -o /tmp/go.tgz \
 && echo "${GO_SHA256}  /tmp/go.tgz" | sha256sum -c - \
 && tar -C /usr/local -xzf /tmp/go.tgz \
 && rm /tmp/go.tgz

# GOTOOLCHAIN=local: the container is pinned; a go.mod asking for another
# version must fail loudly instead of downloading a toolchain silently.
ENV PATH=/usr/local/go/bin:/root/go/bin:${PATH} \
    GOTOOLCHAIN=local \
    GOFLAGS=-buildvcs=false

# Login shells (`sh -lc`, BAT PTYs) re-read PATH from /etc/profile and lose
# the ENV above, so also publish Go through profile.d and /usr/local/bin.
RUN printf 'export PATH=/usr/local/go/bin:/root/go/bin:$PATH\nexport GOTOOLCHAIN=local GOFLAGS=-buildvcs=false\n' > /etc/profile.d/go.sh \
 && ln -s /usr/local/go/bin/go /usr/local/bin/go \
 && ln -s /usr/local/go/bin/gofmt /usr/local/bin/gofmt

# The server runs as root while the bind-mounted repo belongs to the host
# user; without this, git (and go's VCS stamping) refuses the "dubious
# ownership" directory. (4-09 常見坑：root 擁有者)
# diff.ignoreSubmodules=all: a mounted repo that tracks a gitlink whose .git
# pointer holds a Windows absolute path (the course production repo does)
# makes every `git status` / `git commit` on Linux die with "fatal: not a git
# repository: ...D:/..." while inspecting it. Skipping submodule inspection
# keeps git usable inside the container; sms-platform itself has no submodules.
RUN git config --global --add safe.directory '*' \
 && git config --global core.autocrlf false \
 && git config --global diff.ignoreSubmodules all

# Control Tower skills for the claude that runs inside the container: every
# control-tower / ct-* directory with a SKILL.md in the `ct-skills` context
# (8 skills for CT v5.12.2), e.g. a CT release's skills/ folder or your own
# ~/.claude/skills. Without the context the image has none; install them
# later or rebuild with SMS_CT_SKILLS (compose.yaml).
RUN --mount=type=bind,from=ct-skills,target=/tmp/ct-skills \
    mkdir -p /root/.claude/skills \
 && for d in /tmp/ct-skills/control-tower /tmp/ct-skills/ct-*; do \
      [ -f "$d/SKILL.md" ] && cp -r "$d" /root/.claude/skills/; \
    done; \
    n=$(ls /root/.claude/skills | wc -l); \
    if [ "$n" = 0 ]; then echo "ct-skills: none baked in (set SMS_CT_SKILLS to add them)"; \
    else echo "ct-skills: $n baked in"; fi

# claude and node are bundled in /opt/bat-server (4-09 要點①); put them on
# PATH for interactive shells. BAT itself resolves them through the bundle.
# Link the platform binary, not bin/claude: that wrapper resolves its target
# relative to $0 and breaks behind a symlink.
RUN ln -s /opt/bat-server/node_modules/@anthropic-ai/claude-code-linux-x64/claude /usr/local/bin/claude \
 && ln -s /opt/bat-server/bin/node /usr/local/bin/node

# The bind mount is the repo root, so .git is inside the mount; shells start
# in the Go module. Default: sms-platform is the repo root, cloned as
# ct-bat-bmad-go-sms. compose.yaml passes the same path as working_dir and
# build arg; `docker run` users pass -w (scripts/dev-container.sh).
ARG SMS_WORKDIR=/workspace/ct-bat-bmad-go-sms
WORKDIR ${SMS_WORKDIR}
# ENTRYPOINT / CMD / HEALTHCHECK / EXPOSE: inherited from bat-server. Do not override.
