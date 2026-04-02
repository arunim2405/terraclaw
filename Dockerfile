# ============================================================================
# Stage 1: Build terraclaw binary
# ============================================================================
FROM --platform=linux/amd64 golang:1.25-bookworm AS builder


WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /terraclaw .


# ============================================================================
# Stage 2: Runtime image with all dependencies
# ============================================================================
FROM --platform=linux/amd64 debian:bookworm-slim

ARG TERRAFORM_VERSION=1.12.1
ARG STEAMPIPE_VERSION=latest
ARG NODE_MAJOR=22

# Install base dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl unzip zip git jq ca-certificates gnupg wget \
    && rm -rf /var/lib/apt/lists/*

# ---------------------------------------------------------------------------
# Install Node.js (required for opencode-ai)
# ---------------------------------------------------------------------------
RUN curl -fsSL https://deb.nodesource.com/setup_${NODE_MAJOR}.x | bash - \
    && apt-get install -y --no-install-recommends nodejs \
    && rm -rf /var/lib/apt/lists/*

# ---------------------------------------------------------------------------
# Install Terraform
# ---------------------------------------------------------------------------
RUN curl -fsSL "https://releases.hashicorp.com/terraform/${TERRAFORM_VERSION}/terraform_${TERRAFORM_VERSION}_linux_amd64.zip" \
        -o /tmp/terraform.zip \
    && unzip -o /tmp/terraform.zip -d /usr/local/bin \
    && rm /tmp/terraform.zip \
    && chmod +x /usr/local/bin/terraform

# ---------------------------------------------------------------------------
# Install AWS CLI v2
# ---------------------------------------------------------------------------
RUN curl -fsSL "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o /tmp/awscli.zip \
    && unzip -q /tmp/awscli.zip -d /tmp \
    && /tmp/aws/install \
    && rm -rf /tmp/aws /tmp/awscli.zip

# ---------------------------------------------------------------------------
# Install Steampipe (as root so it lands in /usr/local/bin)
# ---------------------------------------------------------------------------
RUN wget https://github.com/turbot/steampipe/releases/download/v2.2.0/steampipe_linux_amd64.deb && \
    dpkg -i steampipe_linux_amd64.deb && \
    rm steampipe_linux_amd64.deb
# ---------------------------------------------------------------------------
# Install OpenCode (opencode-ai via npm, as root for global install)
# ---------------------------------------------------------------------------
RUN npm install -g opencode-ai

# ---------------------------------------------------------------------------
# Create non-root user
# ---------------------------------------------------------------------------
RUN useradd -m -s /bin/bash terraclaw
USER terraclaw
WORKDIR /home/terraclaw

# NOTE: Steampipe plugin install is done at container startup (entrypoint.sh)
# because the steampipe binary cannot execute under QEMU during cross-platform builds.

# ---------------------------------------------------------------------------
# Copy terraclaw binary and project assets
# ---------------------------------------------------------------------------
COPY --from=builder /terraclaw /usr/local/bin/terraclaw
COPY --chown=terraclaw:terraclaw .agents /home/terraclaw/app/.agents
COPY --chown=terraclaw:terraclaw opencode.json /home/terraclaw/app/opencode.json

# Create working directories
RUN mkdir -p /home/terraclaw/app/output \
             /home/terraclaw/.cache/terraclaw \
             /home/terraclaw/.local/share/opencode

WORKDIR /home/terraclaw/app

# ---------------------------------------------------------------------------
# Default environment
# ---------------------------------------------------------------------------
ENV STEAMPIPE_HOST=localhost \
    STEAMPIPE_PORT=9193 \
    STEAMPIPE_DB=steampipe \
    STEAMPIPE_USER=steampipe \
    STEAMPIPE_PASSWORD="" \
    OPENCODE_PORT=4096 \
    TERRAFORM_BIN=terraform \
    OUTPUT_DIR=/home/terraclaw/app/output \
    DEBUG=true \
    DEBUG_LOG_FILE=/home/terraclaw/app/terraclaw.log \
    NO_CACHE=false

# ---------------------------------------------------------------------------
# Entrypoint
# ---------------------------------------------------------------------------
COPY --chown=terraclaw:terraclaw docker/entrypoint.sh /home/terraclaw/entrypoint.sh
RUN chmod +x /home/terraclaw/entrypoint.sh

ENTRYPOINT ["/home/terraclaw/entrypoint.sh"]
