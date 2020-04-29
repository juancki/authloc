FROM golang:1.14


WORKDIR /go/src/app
COPY . .

RUN go build main.go


ENTRYPOINT ["./main.go"]
