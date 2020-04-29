FROM golang:1.13


WORKDIR /go/src/app
COPY . .

RUN go build main.go


ENTRYPOINT ["./main.go"]
