
package main
import (
	"database/sql"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
        "errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	// Third party libs
	"github.com/go-redis/redis/v7"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)


func HomeHandler(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w,"Hello, World Home page")
}


const (
    WORLD= "world"
)

type UUIDTYPE uint64


type Authloc struct{
    /* Attributes are req to be
    capitalized to decode on JSON*/
    Name string
    Pass string
    Loc string
    token string
}
func string2base64(s string) string {
    return base64.StdEncoding.EncodeToString([]byte(s))
}

func AuthlocHandler(w http.ResponseWriter, r *http.Request) {
    // check user password from request
    var body Authloc
    err := json.NewDecoder(r.Body).Decode(&body)
    if err != nil{
        w.WriteHeader(http.StatusBadRequest)
        w.Write(([]byte)("Expected json format with keys: Name, Pass, Loc. All strings."))
        return
    }
    raw := fmt.Sprintf("%s:%s:%s",body.Name,body.Pass,body.Loc)
    hashed, _ :=bcrypt.GenerateFromPassword([]byte(raw),bcrypt.MinCost)
    body.token = string(hashed) // TODO reduce token size to decrease impact in mem
    value := fmt.Sprintf("%s:%s:%s:",string2base64(body.Loc),string2base64(body.Name),"timeFormated") // TODO fix time formatting
    rclient.Lock()
    result := rclient.redis.Set(body.token,value,time.Hour*2) // TODO: make it come from config.
    rclient.Unlock()
    if result.Err() != nil{
        w.WriteHeader(http.StatusInternalServerError)
        w.Write([]byte ("Please retry in a few seconds"))
    }
    fmt.Fprintf(w,"{\"Token\":\"%s\"}",body.token) // Json formated
}


func base64_2_uuid(str string) (UUIDTYPE,error){
    bts,err := base64.StdEncoding.DecodeString(str)
    if err != nil{
        return 0, err
    }
    result, l :=  binary.Uvarint(bts)

    if l == 0{
        return 0, errors.New("Unable to cast fronm base64 to uuid in base64_2_uuid")
    }
    return UUIDTYPE(result),nil
}


func QueryNearNeighbourhs (cuuid string )([]UUIDTYPE,error){
    query := redis.GeoRadiusQuery{}
    query.Radius =  10
    query.Unit = "km"
    rgeoclient.Lock()
    query_out := rgeoclient.redis.GeoRadiusByMember(WORLD,cuuid,&query)
    rgeoclient.Unlock()
    log.Println("query_out: ",query_out)
    if query_out.Err() != nil{
        return nil,query_out.Err()
    }

    geoquery,_ := query_out.Result()
    cuuids :=  make([]UUIDTYPE,len(geoquery))
    for index, geoloc := range geoquery{
        cid, err := base64_2_uuid(geoloc.Name)
        cuuids[index] = cid
        if err != nil{
            return nil,err
        }
    }
    return cuuids, nil
}


func verifyAuth(Token string, cuuid string){
    // TODO
}

type WriteMsg struct {
    Token string
    Message string
    // CUUID string
}

func WriteMsgHandler(w http.ResponseWriter, r *http.Request) {
    var incoming WriteMsg
    decoder := json.NewDecoder(r.Body)
    decoder.DisallowUnknownFields()
    err := decoder.Decode(&incoming)
    log.Print(incoming)
    if err != nil{
        w.WriteHeader(http.StatusBadRequest)
        w.Write(([]byte)("Expected json format with a keys: Token, Message. All strings."))
        return
    }
    rclient.Lock()
    tokenlookup := rclient.redis.Get(incoming.Token)
    rclient.Unlock()
    if tokenlookup.Err() != nil {
        // Token not found in our token DB.
        log.Print(tokenlookup.Err())
        w.WriteHeader(http.StatusForbidden)
        w.Write([]byte ("Invalid token."))
        return
    }
    res, err := tokenlookup.Result()
    cuuid := strings.Split(res,":")[3]
    // TODO Handle this error: Query broken: Bad connection or sth.
    ids, err := QueryNearNeighbourhs(cuuid)
    log.Printf("cuuid: %s\n\tcuuids: %+v\n\rerr: %s",cuuid, ids, err)
    if err != nil{
        log.Print(err)
        w.WriteHeader(http.StatusInternalServerError)
        w.Write([]byte ("Please retry in a few seconds"))
        return
    }

    log.Print(cuuid)
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
    key := "postgresql://"
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

var rclient *Redis
var rgeoclient *Redis
var pclient *Postgre

func main(){
    port := flag.String("port", "localhost:8000", "port to connect (server)")
    redis:= flag.String("redis", "@localhost:6379/0", "format password@IPAddr:port")
    redisGeo := flag.String("redisGeo", "@localhost:6379/1", "format password@IPAddr:port")
    postgre := flag.String("postgre", "authloc:Authloc2846@localhost:5432/postgis_db", "user:password@IPAddr:port/dbname")
    flag.Parse()
    fport := *port
    if strings.Index(*port,":") == -1 {
        fport = "localhost:" + *port
    }
    // Set up Postgre
    pclient = nil
    for pclient == nil{
        pclient = NewPostgre(*postgre)
    }
    defer pclient.pg.Close()
    // Set up redis
    rclient = nil
    for rclient == nil{
        rclient = NewRedis(*redis)
        time.Sleep(time.Second*1)
    }
    rgeoclient = nil
    for rgeoclient == nil{
        rgeoclient  = NewRedis(*redisGeo)
        time.Sleep(time.Second*1)
    }
    fmt.Println(rgeoclient.redis.Conn())
    fmt.Println(rclient.redis.Conn())


    // Starting server
    fmt.Println("Starting authlo c...") // ,*frontport,"for front, ",*backport," for back")
    fmt.Println("--------------------------------------------------------------- ")


    router := mux.NewRouter()


    router.HandleFunc("/", HomeHandler)
    router.HandleFunc("/auth", AuthlocHandler)
    router.HandleFunc("/writemsg", WriteMsgHandler)
    loggedRouter := handlers.LoggingHandler(os.Stdout, router)


    log.Print("Starting Authloc server: ",fport)
    srv := &http.Server{
        Handler: loggedRouter,
        Addr:    fport, // "127.0.0.1:8000",
        // Good practice: enforce timeouts for servers you create!
        WriteTimeout: 15 * time.Second,
        ReadTimeout:  15 * time.Second,
    }
    log.Fatal(srv.ListenAndServe())

}
