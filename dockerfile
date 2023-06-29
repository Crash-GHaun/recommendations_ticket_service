FROM golang:latest as builder
WORKDIR /ticketservice
COPY go.* ./
RUN go mod download
COPY .  ./

RUN go run compilePlugins.go
RUN GO111MODULE=on CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -mod=readonly -v -o ticketservice main.go ticketLogic.go
RUN chmod +x /ticketservice

FROM gcr.io/distroless/base

COPY --from=builder ticketservice/ticketservice /ticketservice
COPY --from=builder ticketservice/plugins/ /plugins


CMD ["./ticketservice"]