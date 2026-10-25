FROM golang:1.26-bookworm AS go
FROM node:22-bookworm-slim
ARG OPENCODE_VERSION=1.19.4
RUN apt-get update && apt-get install -y --no-install-recommends openssh-server socat tmux git curl ca-certificates make gcc libc6-dev procps postgresql-client ripgrep && rm -rf /var/lib/apt/lists/*
COPY --from=go /usr/local/go /usr/local/go
ENV PATH="/usr/local/go/bin:/home/dev/go/bin:${PATH}"
RUN npm install -g --registry=https://registry.npmjs.org opencode-ai@${OPENCODE_VERSION} && mkdir -p /run/sshd && groupadd -g 1100 dev && useradd -u 1100 -g dev -m -s /bin/bash dev && passwd -d dev
COPY entrypoint.sh /usr/local/bin/workspace-entrypoint
COPY pow-dev.sh /usr/local/bin/pow-dev
RUN chmod +x /usr/local/bin/workspace-entrypoint /usr/local/bin/pow-dev
WORKDIR /home/dev/repo
EXPOSE 2222
ENTRYPOINT ["/usr/local/bin/workspace-entrypoint"]
