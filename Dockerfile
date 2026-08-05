# Multi-stage: compile a static Go binary, ship it on a tiny distroless base.
FROM golang:1.25-alpine AS build
WORKDIR /src

# Cache module downloads separately from the source.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Pure-Go (pgx), so CGO off → a static binary that runs on distroless/static.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/current ./cmd/current

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/current /current
EXPOSE 8080
ENTRYPOINT ["/current"]
