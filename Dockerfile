# --- build ---
FROM golang:alpine AS build
WORKDIR /src
# build-base traz gcc + musl-dev para o cgo do H3 (uber/h3-go)
RUN apk add --no-cache git build-base
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGET=api
# cgo ligado + tag "h3" = grade de cobertura com H3 real.
# Link estático com musl → o binário roda no distroless/static.
RUN CGO_ENABLED=1 go build -trimpath -tags h3 \
    -ldflags="-s -w -linkmode external -extldflags '-static'" \
    -o /out/app ./cmd/${TARGET}

# --- runtime ---
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/app /app
USER nonroot
EXPOSE 8080
ENTRYPOINT ["/app"]
