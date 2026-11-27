# syntax=docker/dockerfile:1@sha256:572cebbc2b15e6f84c077742952674889cb63cd5e9b80abdffcbe587b6f19183

# Digest for golang:1.27.4-bookworm is the multi-platform image index
# (linux/amd64, linux/arm64, linux/386, linux/arm/v7, linux/ppc64le),
# matching mise.toml's pinned Go patch exactly; resolved against Docker
# Hub's API. Re-resolve with
# "docker buildx imagetools inspect golang:1.27.4-bookworm" if this patch
# version ever moves; that command reports the index digest below.
FROM golang:1.27.4-bookworm@sha256:ce59b41415e2524ae99926fe1f37ce48ba8c194dcbd2a56a65436cdcefa2bd34 AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ cmd/
COPY internal/ internal/

RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/cvd ./cmd/cvd

# The PDFs in rendered/, when the build context has them. CI runs
# `task render:pdf` before its image build, the publish job downloads the
# release's PDFs into rendered/ instead. A local build without typst has
# none, and cvd then answers 503 on the PDF route only. typst never runs in
# the image or at container start.
RUN --mount=type=bind,source=.,target=/context \
    mkdir -p /out/pdf \
    && for f in /context/rendered/cv.de.pdf /context/rendered/cv.en.pdf; do \
      if [ -f "$f" ]; then cp "$f" /out/pdf/; fi; \
    done

# distroless/static-debian12 carries only a minimal root filesystem,
# ca-certificates and tzdata, no shell, no package manager. The ":nonroot" tag
# runs as uid/gid 65532, not root.
#
# Digest for gcr.io/distroless/static-debian12:nonroot is the multi-platform
# image index (linux/amd64, linux/arm64, linux/arm/v7, linux/s390x,
# linux/ppc64le), resolved against gcr.io's registry API; re-resolve with
# "docker buildx imagetools inspect gcr.io/distroless/static-debian12:nonroot"
# if this needs refreshing; that command reports the index digest below.
FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab

COPY --from=build /out/cvd /usr/local/bin/cvd

# The data cvd serves ships in the image: it is the repository's own data/,
# so one tag is one binary plus the document it serves. A deployment that
# wants other data mounts it over DATA_DIR; see charts/cv.
COPY data/ /data/
ENV DATA_DIR=/data

# The pre-rendered PDFs, possibly none; see the build stage. cvd reads them
# once at start from PDF_DIR.
COPY --from=build /out/pdf/ /pdf/
ENV PDF_DIR=/pdf

# Matches ADDR's and ADMIN_ADDR's defaults, the public listener and
# govue's own admin listener (health, readiness, metrics).
EXPOSE 8080
EXPOSE 8081

# Numeric, not "nonroot:nonroot": the same uid/gid the base image's own
# nonroot user already resolves to, redundant but kept explicit.
USER 65532:65532

ENTRYPOINT ["/usr/local/bin/cvd"]
