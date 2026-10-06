# syntax=docker/dockerfile:1

# Cross-compiles ccm for every platform in the Makefile's PLATFORMS list.
# Go cross-compiles natively (CGO_ENABLED=0), so the builder always runs on
# the host's platform; no emulation needed.
#
#   docker buildx build --target dist --output type=local,dest=dist .
#
# or just `make docker-dist`.

ARG GO_VERSION=1.22

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS build
RUN apk add --no-cache make
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
# .git is not in the build context, so the version comes in as a build arg.
ARG VERSION=dev
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    make dist VERSION=${VERSION}

FROM scratch AS dist
COPY --from=build /src/dist/ /
