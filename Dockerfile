# Build stage
FROM golang:1.22-bookworm AS builder

RUN apt-get update && apt-get install -y --no-install-recommends nodejs npm && rm -rf /var/lib/apt/lists/*

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY package.json package-lock.json tailwind.config.js ./
RUN npm ci

COPY . .
RUN go run github.com/a-h/templ/cmd/templ@v0.2.778 generate
RUN npm run build:css
RUN CGO_ENABLED=0 go build -o /out/goshpanel ./cmd/goshpanel

# Runtime stage
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /out/goshpanel /usr/local/bin/goshpanel
COPY configs/config.example.yaml /etc/goshpanel/config.example.yaml

EXPOSE 4674
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/goshpanel"]
