# Optional convenience only -- the binary itself is the real deliverable
# and never requires Docker to use.
FROM golang:1.25-alpine AS build
RUN apk add --no-cache ca-certificates
WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 go build -o /out/stentor ./cmd/stentor

FROM scratch
# scratch has no CA bundle -- every platform call here is HTTPS, so copy one in.
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/stentor /stentor
ENTRYPOINT ["/stentor"]
