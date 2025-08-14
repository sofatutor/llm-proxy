# syntax=docker/dockerfile:1.7
FROM ubuntu:25.04

ENV DEBIAN_FRONTEND=noninteractive

# Create a non-root user and install development tools
ARG USER=ubuntu
ARG UID=1000
ARG GID=1000

RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
	--mount=type=cache,target=/var/lib/apt,sharing=locked \
	apt-get -o Acquire::Check-Valid-Until=false -o Acquire::Check-Date=false update \
	&& apt-get install -y --no-install-recommends \
		bash \
		sudo \
		ca-certificates \
		curl \
		wget \
		gnupg \
		lsb-release \
		git \
		make \
		pkg-config \
		build-essential \
		sqlite3 \
		libsqlite3-dev \
		redis-server \
		xvfb \
		xauth \
	&& printf 'Acquire::Check-Valid-Until "false";\nAcquire::Check-Date "false";\n' > /etc/apt/apt.conf.d/99no-check-valid-until

# Install Docker Engine inside the container (for dind-like workflows)
RUN mkdir -p /etc/apt/keyrings \
	&& curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg \
	&& echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" > /etc/apt/sources.list.d/docker.list \
	&& apt-get -o Acquire::Check-Valid-Until=false -o Acquire::Check-Date=false update \
	&& apt-get install -y --no-install-recommends \
		docker-ce \
		docker-ce-cli \
		containerd.io \
		docker-buildx-plugin \
		docker-compose-plugin \
	&& groupadd -f docker

# Create user and grant sudo + docker group access
RUN groupadd -f -g ${GID} ${USER} || true \
	&& if ! id -u ${USER} >/dev/null 2>&1; then useradd -m -u ${UID} -g ${GID} -s /bin/bash ${USER}; fi \
	&& usermod -aG docker,sudo ${USER} \
	&& grep -q "^${USER} ALL=(ALL) NOPASSWD: ALL$" /etc/sudoers || echo "${USER} ALL=(ALL) NOPASSWD: ALL" >> /etc/sudoers

# Install Go (1.23.x) and set up PATH for all users
RUN curl -fsSL https://go.dev/dl/go1.23.0.linux-amd64.tar.gz -o /tmp/go.tgz \
	&& rm -rf /usr/local/go \
	&& tar -C /usr/local -xzf /tmp/go.tgz \
	&& ln -sf /usr/local/go/bin/go /usr/local/bin/go \
	&& go version

# Enable CGO for go-sqlite3 and set GOPATH for the non-root user
ENV CGO_ENABLED=1
ENV GO111MODULE=on
ENV GOPATH=/home/${USER}/go
ENV PATH=/usr/local/go/bin:${GOPATH}/bin:${PATH}
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

VOLUME /var/lib/docker
