FROM alpine:3.16

# Set by Docker automatically.
# If building with `docker build` directly, make sure to set GOOS/GOARCH explicitly when calling make:
# `make build GOOS=linux GOARCH=amd64`
# Otherwise, make will not add suffixes to the binary name and Docker will not be able to find it.
# Alternatively, `make image` can also take care of producing the binary with the correct name and then running
# `docker build` for you.
ARG TARGETOS
ARG TARGETARCH

# Some old-ish versions of Docker do not support adding and renaming in the same line, and will instead
# interpret the second argument as a new folder to create and place the original file inside.
# For this reason, we workaround with regular ADD and then run mv inside the container.
ADD --chmod=755 newrelic-k8s-metrics-adapter-${TARGETOS}-${TARGETARCH} ./
RUN mv newrelic-k8s-metrics-adapter-${TARGETOS}-${TARGETARCH} /usr/local/bin/newrelic-k8s-metrics-adapter

ENTRYPOINT ["/usr/local/bin/newrelic-k8s-metrics-adapter"]
