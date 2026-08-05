# Multi-stage: compile a static Go binary, ship it on a tiny distroless base.
# The SQL parser (pg_query_go) embeds C, so this needs cgo — but we link it
# statically against musl so the result still runs on distroless/static.
FROM golang:1.25-alpine AS build
RUN apk add --no-cache gcc musl-dev
WORKDIR /src

# Cache module downloads separately from the source.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -trimpath \
    -ldflags='-linkmode external -extldflags "-static"' \
    -o /out/current ./cmd/current

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/current /current
EXPOSE 8080
ENTRYPOINT ["/current"]
