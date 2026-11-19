# Cluster image (Auth + Proxy in one process). Built on a machine that can
# reach docker.io; the saved tarball is fully offline-loadable.
#
# Ubuntu 24.04 (glibc 2.39+) is required: the cluster binaries are built on a
# newer host libc and need GLIBC_2.38. debian:bookworm-slim is 2.36.
FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && \
    apt-get install --no-install-recommends -y \
        ca-certificates \
        curl \
        dumb-init && \
    update-ca-certificates && \
    apt-get -y clean && \
    rm -rf /var/lib/apt/lists/*

COPY teleport tctl tsh /usr/local/bin/
COPY entrypoint.sh /usr/local/bin/teleport-entrypoint

RUN chmod 0755 \
        /usr/local/bin/teleport \
        /usr/local/bin/tctl \
        /usr/local/bin/tsh \
        /usr/local/bin/teleport-entrypoint && \
    mkdir -p \
        /var/lib/teleport \
        /etc/teleport/certs \
        /etc/teleport/web-overlay/app

ENV TELEPORT_WEBASSETS_OVERLAY=/etc/teleport/web-overlay

VOLUME ["/var/lib/teleport"]

# Graceful shutdown (Teleport drains connections on SIGQUIT).
STOPSIGNAL SIGQUIT

EXPOSE 27443

ENTRYPOINT ["/usr/bin/dumb-init", "--", "/usr/local/bin/teleport-entrypoint"]
