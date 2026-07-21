# Build stage: pinned to the bookworm variant so the release binary links
# against the same glibc as the runtime image below (aarch64/amd64 both fine).
FROM rust:1-bookworm AS build
WORKDIR /src
COPY Cargo.toml Cargo.lock ./
COPY src ./src
RUN cargo build --release --locked

# Runtime stage: slim Debian with python3 (the only interpreter the sandbox
# executes) and a non-root user.
FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends python3 \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --system --uid 10001 --create-home --shell /usr/sbin/nologin sandbox
COPY --from=build /src/target/release/sandbox /sandbox
USER sandbox
EXPOSE 8070
ENTRYPOINT ["/sandbox"]
