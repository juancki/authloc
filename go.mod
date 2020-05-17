module github.com/juancki/authloc

go 1.14

require (
	github.com/go-redis/redis/v7 v7.2.0
	github.com/golang/protobuf v1.4.1
	github.com/gorilla/handlers v1.4.2 // indirect
	github.com/gorilla/mux v1.7.4 // indirect
	github.com/juancki/wsholder v0.0.0-20200501153118-5323afe3473e
	github.com/lib/pq v1.4.0
	github.com/mmcloughlin/geohash v0.9.0 // indirect
	golang.org/x/crypto v0.0.0-20200427165652-729f1e841bcc // indirect
	golang.org/x/net v0.0.0-20200501053045-e0ff5e5a1de5 // indirect
	google.golang.org/grpc v1.29.1 // indirect
	google.golang.org/protobuf v1.22.0
)

replace github.com/juancki/wsholder => /home/gojuancarlos/go/src/github.com/juancki/wsholder

replace github.com/juancki/authloc => /home/gojuancarlos/go/src/github.com/juancki/authloc
