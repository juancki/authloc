
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	// Third party libs
	"github.com/go-redis/redis/v7"
	"github.com/golang/protobuf/proto"
	pb "github.com/juancki/authloc/pb"
	mydb "github.com/juancki/authloc/dbutils"
	_ "github.com/lib/pq"
)


func send400error(w http.ResponseWriter,str string){
    w.WriteHeader(http.StatusBadRequest)
    w.Write([]byte(str))
}

type Redis struct{
    redis redis.Client
    sync.Mutex
}


type Postgre struct{
    pg sql.DB
    sync.Mutex
}


func NewRedis(connURL string) *Redis{
    // redis://password@netloc:port/dbnum
    // redis does not have a username
    key := "redis://"
    if strings.HasPrefix(connURL,key){
        connURL = connURL[len(key):]
    }
    dbnum,_ := strconv.Atoi(strings.Split(connURL,"/")[1])
    connURL = strings.Split(connURL,"/")[0]

    pass := strings.Split(connURL,"@")[0]
    log.Print("`",pass,"`\t",len(pass))
    addr := strings.Split(connURL,"@")[1]
    client := redis.NewClient(&redis.Options{
            Addr:     addr,
            Password: "", // no password set
            DB:       dbnum,  // use default DB
    })

    pong, err := client.Ping().Result()
    if err != nil{
        // Exponential backoff
        log.Print("Un able to connect to REDIS `", dbnum,"` at: `", addr, "` with password: `",pass,"`.")
        log.Print(err)
        return nil
    }
    log.Print("I said PING, Redis said: ",pong)
    return &Redis{redis: *client}

}

func NewPostgre(connURL string) *Postgre{
    // postgresql://user:password@netloc:port/dbname
    key := "postgresql://"
    if strings.HasPrefix(connURL,key){
        connURL = connURL[len(key):]
    }
    dbname := strings.Split(connURL,"/")[1]
    connURL = strings.Split(connURL,"/")[0]

    creds := strings.Split(connURL,"@")[0]
    user := strings.Split(creds,":")[0]
    pass := strings.Split(creds,":")[1]
    addr := strings.Split(connURL,"@")[1]
    host := strings.Split(addr,":")[0]
    port := strings.Split(addr,":")[1]
    psqlInfo := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
            host, port, user, pass, dbname)
    db, err := sql.Open("postgres", psqlInfo)
    if err != nil{
        log.Fatal("Un able to connect to POSTGRE with ", psqlInfo)
    }
    log.Print("Connected to Postgre!")// with, ",psqlInfo)
    return &Postgre{pg: *db}

}


var rclient *mydb.Redis
var rgeoclient *mydb.Redis
var pclient *mydb.Postgre

func main(){
    redis:= flag.String("redis", "@localhost:6379/0", "format password@IPAddr:port")
    redisGeo := flag.String("redisGeo", "@localhost:6379/1", "format password@IPAddr:port")
    flag.Parse()

    // Set up redis
    rclient = nil
    for rclient == nil{
        rclient = mydb.NewRedis(*redis)
        time.Sleep(time.Second*1)
    }
    rgeoclient = nil
    for rgeoclient == nil{
        rgeoclient  = mydb.NewRedis(*redisGeo)
        time.Sleep(time.Second*1)
    }
    event := &pb.Event{}
    event.Name = "Juan Carlos Gómez Pomar"
    event.Uuids = 323
    event.N = -1224

    fmt.Printf("Original: %+v\n", event)
    bts, err := proto.Marshal(event)
    if err != nil{
        fmt.Println(err)
        return
    }

    rclient.Redis.Set("key",bts,time.Hour*2)

    time.Sleep(time.Second)

    result := rclient.Redis.Get("key")
    if result.Err() != nil{
        fmt.Println(result.Err())
        return
    }

    bts2, err := result.Bytes()
    if err != nil{
        fmt.Println(err)
        return
    }
    event2 := new(pb.Event)
    err = proto.Unmarshal(bts2,event2)
    if err != nil{
        fmt.Println(err)
        return
    }
    fmt.Println(event2)



    rgeoclient.GeoAddChatCoor("50.50:30.30","werqadsf",event2)
    rgeoclient.GeoAddChatCoor("50.51:30.30","asdfasdf",event2)




    fmt.Println("Finished")


}
