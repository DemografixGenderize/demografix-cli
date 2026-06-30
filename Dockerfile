# Minimal image for CI use. GoReleaser places the prebuilt binary in the build
# context, so this is just a copy onto a static, nonroot base.
FROM gcr.io/distroless/static:nonroot
COPY demografix /usr/local/bin/demografix
ENTRYPOINT ["/usr/local/bin/demografix"]
