FROM golang:1.20-bullseye as builder
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
COPY --from=builder ticketTitleTpl.txt ticketTitleTpl.txt
COPY --from=builder updateTicketTpl.txt updateTicketTpl.txt


CMD ["./ticketservice"]