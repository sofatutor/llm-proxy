# syntax=docker/dockerfile:1.7
FROM golang:1.23-bookworm

# Create a non-root user and install development tools
ARG USER=ubuntu
ARG UID=1000
ARG GID=1000

RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
	--mount=type=cache,target=/var/lib/apt,sharing=locked \
	apt-get update \
	&& DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
		bash \
		ca-certificates \
		curl \
		git \
		make \
		pkg-config \
		build-essential \
		sqlite3 \
		libsqlite3-dev \
		redis-server \
	&& groupadd -g ${GID} ${USER} \
	&& useradd -m -u ${UID} -g ${GID} -s /bin/bash ${USER}

# Enable CGO for go-sqlite3 and set GOPATH for the non-root user
ENV CGO_ENABLED=1
ENV GO111MODULE=on
ENV GOPATH=/home/${USER}/go
ENV PATH=${PATH}:/usr/local/go/bin:${GOPATH}/bin
ENV GOMODCACHE=/home/${USER}/go/pkg/mod
ENV GOCACHE=/home/${USER}/.cache/go-build

RUN mkdir -p ${GOPATH} ${GOMODCACHE} ${GOCACHE} \
	&& chown -R ${UID}:${GID} ${GOPATH} /home/${USER}/.cache

# Important for background agents: do NOT copy source code here.
# The agent will clone the repository into the workspace later.

COPY --chown=${UID}:${GID} go.mod go.sum /home/${USER}/modcache/
RUN --mount=type=cache,target=/home/${USER}/go/pkg/mod,sharing=locked \
	--mount=type=cache,target=/home/${USER}/.cache/go-build,sharing=locked \
	cd /home/${USER}/modcache && go mod download

USER ${USER}
WORKDIR /home/${USER}

# Default shell
SHELL ["/bin/bash", "-lc"]

## Start Redis server on container startup for dev terminals
## Note: background agents may override CMD; we keep Redis launched via a profile script
RUN echo 'if ! pgrep -x redis-server >/dev/null 2>&1; then nohup redis-server --daemonize yes >/dev/null 2>&1 || true; fi' >> ~/.bashrc


