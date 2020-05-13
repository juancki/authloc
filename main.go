
package main
import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"errors"
	"strings"
	"sync"
	"time"

	// Third party libs
	"github.com/golang/protobuf/ptypes"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	pb "github.com/juancki/wsholder/pb"
	authpb "github.com/juancki/authloc/pb"
	mydb "github.com/juancki/authloc/dbutils"
	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

// Config
// TODO add config for the grpc server

// TODO How to develop on both projects at the same time (removing go mod ¿?)

// FEATURES
// TODO gorutine for the gcrp connection to the client
// TODO clean the logging

func HomeHandler(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w,"Hello, World Home page")
}


const (
    WORLD= "world"
)

func sendMessageToChatWithMembers(rep *pb.ReplicationMsg, members []string) error{
    cuuids, err := rclient.GetCuuidFromUserid(members...)
    if err != nil {
        return err
    }
    if cuuids == nil  || len(cuuids) == 0{
        return nil
    }
    rep.CUuids = cuuids
    grpcmsgs  <- rep
    return nil
}


func sendMessageToChat(chatid string, rep *pb.ReplicationMsg) error {
    members, err := rclient.GetChatMembers(chatid)
    if err != nil{
        return err
    }
    return sendMessageToChatWithMembers(rep, members)
}


func CreateChatHandler(w http.ResponseWriter, r *http.Request) {
//     TODO
//     user, err := authenticateRequest(r)
//     if err != nil{
//         send401Unauthorized(w,"Expected bearer token.")
//         return
//     }
    loc := "13.13:20.20"
    user:= "pep"
    var chat authpb.Chat
    err := json.NewDecoder(r.Body).Decode(&chat)
    if err != nil{
        send400error(w,"Expected json format with keys: Name, IsOpen, IsPhysical, among others. All strings.")
        return
    }
    // filling up some fields; always overwrite this fields.
    chat.Creator = user
    chat.Creation =  ptypes.TimestampNow()
    chatid := createMsgId(loc+user, chat.Creation.Nanos, chat.Creation.Seconds)
    members := chat.Members
    chat.Members = nil
    err = rclient.SetChat(chatid, &chat)
    if err != nil{
        log.Println(err, &chat)
        send500error(w)
        return
    }
    if chat.IsOpen {
        err = rgeoclient.GeoAddChat(loc, chatid)
        if err != nil{
            rgeoclient.RemoveChat(chatid)
            send500error(w)
            return
        }
    }
    if members != nil && len(members) != 0{
        err := rclient.SetChatMember(chatid, members...)
        if err != nil{
            rgeoclient.RemoveChat(chatid)
            send500error(w)
        }
        var rep pb.ReplicationMsg
        rep.Meta = new(pb.Metadata)
        rep.Meta.MsgMime = make(map[string]string)
        rep.Meta.MsgMime["Content-Type"] = "text/plain"
        rep.Meta.Resource = "/chat/"+chatid
        rep.Msg = []byte("")
        sendMessageToChatWithMembers(&rep, members)
    }
    // Notify member -> this can be done in a queue to return faster this object
    // Save this message in each member notification list. push and pop notifications.
    // GetCuuid from Users -> send message to wsholder.
    w.Header().Add("Content-Type", "application/json")
    out:= fmt.Sprintf("{ \"chatid\": \"%s\"}",chatid)
    w.Write([]byte(out))
}



type Authloc struct{
    /* Attributes are req to be
    capitalized to decode on JSON*/
    Name string
    Pass string
    Loc string
}

func string2base64(s string) string {
    return base64.StdEncoding.EncodeToString([]byte(s))
}

func AuthlocHandler(w http.ResponseWriter, r *http.Request) {
    // check user password from request
    var body Authloc
    err := json.NewDecoder(r.Body).Decode(&body)
    if err != nil{
        send400error(w,"Expected json format with keys: Name, Pass, Loc. All strings.")
        return
    }
    raw := fmt.Sprintf("%s:%s:%s", body.Name, body.Pass, body.Loc)
    hashed, _ := bcrypt.GenerateFromPassword([]byte(raw),bcrypt.MinCost)
    token := string(hashed) // TODO reduce token size to decrease impact in mem
    err = rclient.SetToken(token, body.Name, body.Loc)
    err = rclient.SetUserToken(body.Name, token)
    if err != nil{
        log.Println("Error 101: ", err)
        send500error(w)
        return
    }
    fmt.Fprintf(w,"{\"Token\":\"%s\"}",token) // Json formated
}


func verifyAuth(Token string, cuuid string){
    // TODO
}

type WriteMsg struct {
    Token string
    Message string
    // CUUID string
    Mime map[string]string
}


func createMsgId(strseed string,nanos int32,seconds int64) string{
    // set message id
    hasher := sha1.New()
    hasher.Write([]byte(strseed))
    buf := make([]byte,10)
    binary.BigEndian.PutUint32(buf, uint32(nanos))
    hasher.Write(buf)
    msgid := base64.URLEncoding.EncodeToString(hasher.Sum(nil))[0:7]
    return msgid
}


func replicationChatMessage(chatid string, r *http.Request) (*pb.ReplicationMsg, error){
    // Creates a Replication Msg using the headers from the
    // the r request and the chatid information.
    timearrived := ptypes.TimestampNow()
    body, err := ioutil.ReadAll(r.Body)
    if err != nil{
        return nil, err
    }
    repMsg := new(pb.ReplicationMsg)
    repMsg.Msg = body
    msgid := createMsgId(chatid,timearrived.Nanos,timearrived.Seconds)
    resource := fmt.Sprintf("/msg/%s/%s",chatid, msgid)
    repMsg.Meta = new(pb.Metadata)
    repMsg.Meta.Resource = resource
    repMsg.Meta.Arrived = timearrived
    repMsg.Meta.MsgMime = make(map[string]string)
    repMsg.Meta.MsgMime["Content-Type"] = r.Header.Get("Content-Type")
    metamime := r.Header.Get("WS-META")
    if metamime != "" {
        repMsg.Meta.MsgMime["WS-META"] = metamime
        b64, err := base64.StdEncoding.DecodeString(metamime)
        if err == nil{
            var headers map[string]string
            json.Unmarshal(b64, &headers)
            for key, value := range headers{
                repMsg.Meta.MsgMime[key] = value
            }
        }
    }
    return repMsg, nil
}

func authenticateRequest(r *http.Request) (string, error){
    // Most basic OAuth token bearer implementation 
    // https://tools.ietf.org/html/rfc6750 2.1 Authorization Request Header Field
    splits := strings.Split(r.Header.Get("Authentication"), " ")
    if splits==nil  ||  len(splits) == 0{
        return "", errors.New("Authentication token not provided")
    }
    token := splits[len(splits)-1]
    userid, err := rclient.GetUserFromToken(token)
    if err != nil{
        return "", err
    }
    return userid, nil
}

func WriteMsgChatHandler(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    chatid, ok := vars["chatid"]
    if !ok {
        send400error(w,"Bad requests, this URL is /write/chat/[chat-id]")
        return
    }
    user, err := authenticateRequest(r)
    if err != nil{
        log.Println("Could not authenticate", err)
        send401Unauthorized(w,"Could not authenticate")
        return
    }
    rep, err :=  replicationChatMessage(chatid,r)
    if err != nil{
        log.Println(err)
        send400error(w, "Message malformed.")
        return
    }
    rep.Meta.Poster = user
    err = sendMessageToChat(chatid,rep)
    if err != nil{
        send500error(w)
    }
    // TODO store message persistently ¿?
    //    var uni pb.UniMsg; uni.Meta = rep.Meta; uni.Msg = rep.Msg;
}

func WriteMsgHandler(w http.ResponseWriter, r *http.Request) {
    var incoming WriteMsg
    decoder := json.NewDecoder(r.Body)
    decoder.DisallowUnknownFields()
    err := decoder.Decode(&incoming)
    if err != nil{
        log.Print(incoming)
        send400error(w,"Expected json format with a keys: Token, Message. All strings.")
        return
    }
    cuuid,err :=  rclient.GetCuuidFromToken(incoming.Token)
    if err != nil{
        send400error(w,err.Error())
        return
    }
    // TODO Handle this error: Query broken: Bad connection or sth.
    ids, err := rgeoclient.QueryNearNeighbourhs(cuuid)
    if err != nil{
        log.Printf("Error 111\tcuuid: %s\n\t\t\t|cuuids: %+v\n\t\t\t+err: %s",cuuid, ids, err)
        send500error(w)
        return
    }
    repMsg := &pb.ReplicationMsg{CUuids:ids,Msg: []byte(incoming.Message)}
    // Filling META
    repMsg.Meta = &pb.Metadata{Poster:cuuid, Resource: "/msg/[chat-id]/[m-id]", Arrived: ptypes.TimestampNow()}
    repMsg.Meta.MsgMime = make(map[string]string) // TODO add headers such as content type ...
    if tp := r.Header.Get("Content-Type"); tp != ""{
        repMsg.Meta.MsgMime["Content-Type"] = tp
    }
    grpcmsgs <- repMsg
}

func gRPCworker(addr string){
    conn, err := grpc.Dial(addr,grpc.WithInsecure(),grpc.WithBlock())
    c := pb.NewWsBackClient(conn)
    ctx, cancel := context.WithTimeout(context.Background(), time.Hour)// TODO find a better number for context timeout
    defer cancel()
    defer conn.Close()
    repCall, err := c.Replicate(ctx)
    if err != nil{
        log.Print("Error 112")
        log.Print(err)
        return
    }
    // Responding to incomming grpcmessages to deliver
    for true {
        repMsg := <-grpcmsgs
        err = repCall.Send(repMsg)
        if err != nil{
            log.Print("Error 114")
            log.Print(err)
            return
        }
        log.Printf("Forwarded message about uuids: %+v",repMsg.CUuids)
    }
}

// This should be initialized with `go gRPCmaster()`
// This continuously spaws a gRPCworker to maintail a 
// pool of 1 worker.
func gRPCmaster(addr string) {
    var wg sync.WaitGroup
    for true {
        wg.Add(1)
        go func (){
            gRPCworker(addr)
            wg.Done()
        }()
        wg.Wait()
    }
}


func send500error(w http.ResponseWriter){
    w.WriteHeader(http.StatusInternalServerError)
    w.Write([]byte("Please retry in a few seoncds."))
}

func send400error(w http.ResponseWriter,str string){
    w.WriteHeader(http.StatusBadRequest)
    w.Write([]byte(str))
}

func send401Unauthorized(w http.ResponseWriter,str string){
    w.WriteHeader(http.StatusUnauthorized)
    w.Write([]byte(str))
}


var rclient *mydb.Redis
var rgeoclient *mydb.Redis
var pclient *mydb.Postgre
var grpcmsgs chan *pb.ReplicationMsg

func main(){
    port := flag.String("port", "localhost:8000", "port to connect (server)")
    redis:= flag.String("redis", "@localhost:6379/0", "format password@IPAddr:port")
    redisGeo := flag.String("redisGeo", "@localhost:6379/1", "format password@IPAddr:port")
    postgre := flag.String("postgre", "authloc:Authloc2846@localhost:5432/postgis_db", "user:password@IPAddr:port/dbname")
    grpcSrv := flag.String("grpc", "localhost:8090", "IPv4.addrs:port")
    flag.Parse()
    fport := *port
    if strings.Index(*port,":") == -1 {
        fport = "localhost:" + *port
    }

    // Set up Postgre
    pclient = nil
    for pclient == nil{
        pclient = mydb.NewPostgre(*postgre)
    }

    defer pclient.Pg.Close()
    // Set up redis
    rclient = nil
    rclient = mydb.NewRedis(*redis)
    for rclient == nil{
        rclient = mydb.NewRedis(*redis)
        time.Sleep(time.Second*1)
    }
    rgeoclient = nil
    rgeoclient  = mydb.NewRedis(*redisGeo)
    for rgeoclient == nil{
        rgeoclient  = mydb.NewRedis(*redisGeo)
        time.Sleep(time.Second*1)
    }


    // gRPC connection handler
    grpcmsgs = make(chan *pb.ReplicationMsg)
    go gRPCmaster(*grpcSrv)


    // Starting server
    fmt.Println("Starting authlo c...") // ,*frontport,"for front, ",*backport," for back")
    fmt.Println("--------------------------------------------------------------- ")

    router := mux.NewRouter()
    router.HandleFunc("/", HomeHandler)
    router.HandleFunc("/auth", AuthlocHandler)
//    router.Handle("/writemsg", TimeRequest(http.HandlerFunc(WriteMsgHandler)))
    router.HandleFunc("/writemsg", WriteMsgHandler)
    router.HandleFunc("/write/chat/{chatid}", WriteMsgChatHandler)
    router.HandleFunc("/create/chat", CreateChatHandler)
//    router.HandleFunc("/create/event", CreateEventHandler)

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
///// MIDDLEWARE
func TimeRequest(h http.Handler) http.Handler {
    return http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        h.ServeHTTP(w, r) // call handler
        log.Println("Elapsed: ", time.Since(start), " ms")
    })
}
