# syntax=docker/dockerfile:1@sha256:4edf897a3ffa55b89f906fc8cc78afdb3f1834cc9c7083565e611a8a7d5fe99e

# Digest for golang:1.27.4-bookworm (linux/amd64), matching mise.toml's pinned
# Go patch exactly; resolved against Docker Hub's API.
FROM golang:1.27.4-bookworm@sha256:fb7cceb528b47141eccd9b60bf3cbd27d5ef5f8cf5b7d338bec866da495b50dc AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ cmd/
COPY internal/ internal/

RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/cvd ./cmd/cvd

# distroless/static-debian12 carries only a minimal root filesystem,
# ca-certificates and tzdata, no shell, no package manager. The ":nonroot" tag
# runs as uid/gid 65532, not root.
#
# Digest for gcr.io/distroless/static-debian12:nonroot (linux/amd64), resolved
# against gcr.io's registry API; re-resolve with "docker buildx imagetools
# inspect gcr.io/distroless/static-debian12:nonroot" if this needs refreshing.
FROM gcr.io/distroless/static-debian12:nonroot@sha256:52dcfbabb7457ea47c82f6e13af8c8a4a1d9f7b0145142b3ecab20f2b888411d

COPY --from=build /out/cvd /usr/local/bin/cvd

# Matches ADDR's and ADMIN_ADDR's defaults, the public listener and
# govue's own admin listener (health, readiness, metrics).
EXPOSE 8080
EXPOSE 8081

# Numeric, not "nonroot:nonroot": the same uid/gid the base image's own
# nonroot user already resolves to, redundant but kept explicit.
USER 65532:65532

ENTRYPOINT ["/usr/local/bin/cvd"]
