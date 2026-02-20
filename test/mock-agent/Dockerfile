FROM golang:1.21-alpine

WORKDIR /app

# Mock agent that responds to A2A protocol
COPY mock-agent.go .

RUN go mod init mock-agent && \
    go build -o mock-agent mock-agent.go

EXPOSE 8080

CMD ["./mock-agent"]
