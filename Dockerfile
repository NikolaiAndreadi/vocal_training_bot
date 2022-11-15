# syntax=docker/dockerfile:1

# === BUILD STAGE
FROM golang:1.19-buster AS build

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY *.go ./
COPY BotExt/*.go ./BotExt/

RUN go build -o /botapp

# === DEPLOY STAGE
FROM gcr.io/distroless/base-debian10 AS run

WORKDIR /
COPY --from=build /botapp /botapp

# metrics service
EXPOSE 2112
# http/https
EXPOSE 80 88 443 8443

USER nonroot:nonroot

ENTRYPOINT [ "/botapp"]
