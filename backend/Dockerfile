# --- build ---
FROM golang:alpine AS build
WORKDIR /src
# build-base traz gcc + musl-dev para o cgo do H3 (uber/h3-go)
RUN apk add --no-cache git build-base
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGET=api
# cgo ligado + tag "h3" = grade de cobertura com H3 real (res 9, hexágonos).
RUN CGO_ENABLED=1 go build -trimpath -tags h3 -ldflags="-s -w" -o /out/app ./cmd/${TARGET}

# --- runtime ---
# alpine (musl) em vez de distroless: o binário cgo linka musl dinamicamente.
# Sem dockerCommand no render.yaml — o ENTRYPOINT exec-form roda direto.
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 app
COPY --from=build /out/app /app
USER app
EXPOSE 8080
ENTRYPOINT ["/app"]
