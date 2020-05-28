
package main
import (
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	// Third party libs
	"github.com/go-redis/redis"
	"github.com/golang/protobuf/ptypes"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	mydb "github.com/juancki/authloc/dbutils"
	authpb "github.com/juancki/authloc/pb"
	pb "github.com/juancki/wsholder/pb"
	_ "github.com/lib/pq"
	geohash "github.com/mmcloughlin/geohash"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

// Config

// FEATURES
// TODO Create user profiles

// TODO /singup, /login, /logout ,... OAuth

// TODO distributed systems.
//      On new wsholders auto connect.
//      Batch messages to the corresponsing wsholders based on CUUID

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
    if len(members) == 0{
        return nil
    }
    if err != nil{
        return err
    }
    return sendMessageToChatWithMembers(rep, members)
}

func stringSet(in []string) []string {
    // returns list without duplicates
    set := make(map[string]bool)
    for _, member := range in{
        set[member] = true
    }
    out := make([]string,len(set))
    ind := 0
    for key := range set{
        out[ind] = key
        ind += 1
    }
    return out
}


func StoreAndForwardChat(chatid string, chat *authpb.Chat) error {
    members := chat.Members
    chat.Members = nil // TODO this changes the pointer of chat.Memebers
    err := rclient.SetChat(chatid, chat)
    if chat.IsOpen {
        loc := chat.More["Created-At-Location"]
        err = rgeoclient.GeoAddChat(loc, chatid)
        if err != nil{
            rgeoclient.RemoveChat(chatid)
            return err
        }
        // Broadcast
    }
    if members != nil && len(members) != 0{
        members = stringSet(members) // This avoids repeated members 
        err := rclient.SetChatMember(chatid, members...)
        if err != nil{
            rgeoclient.RemoveChat(chatid)
            rgeoclient.GeoRemoveChat(chatid)
            return err
        }
        var rep pb.ReplicationMsg
        rep.Meta = new(pb.Metadata)
        rep.Meta.MsgMime = make(map[string]string)
        rep.Meta.MsgMime["Content-Type"] = "text/plain"
        rep.Meta.Resource = "/chat/"+chatid
        rep.Msg = []byte(chat.Name)
        sendMessageToChat(chatid,&rep)
    }
    return nil
}

func CreateChatHandler(w http.ResponseWriter, r *http.Request) {
    row, err := authenticateRequestPlusRow(r)
    if err != nil{
        send401Unauthorized(w,"Expected valid bearer token.")
        return
    }
    loc := row["loc"]
    user:= row["userid"]
    var chat authpb.Chat
    err = json.NewDecoder(r.Body).Decode(&chat)
    if err != nil{
        send400error(w,"Expected json format with keys: Name, IsOpen, among others. All strings.")
        return
    }
    // filling up some fields; always overwrite this fields.
    chat.Creator = user
    chat.Creation =  ptypes.TimestampNow()
    chatid := createMsgId(loc+user, chat.Creation.Nanos, chat.Creation.Seconds)
    chat.Members = append(chat.Members,user)
    chat.Resource = "/chat/"+chatid
    if chat.More == nil{
        chat.More = make(map[string]string)
    }
    if _, ok := chat.More["Created-At-Location"]; !ok{
        chat.More["Created-At-Location"] = loc
    }

    err =  StoreAndForwardChat(chatid, &chat)
    if err != nil{
        log.Println(err, &chat)
        send500error(w)
        return
    }
    w.Header().Add("Content-Type", "application/json")
    out:= fmt.Sprintf("{ \"chatid\": \"%s\"}",chatid)
    w.Write([]byte(out))
}

func StoreAndForwardEventAndChat(eventid string, event *authpb.Event, chat *authpb.Chat) error {
    members := event.Members
    event.Members = nil // TODO this changes the pointer of event.Memebers
    err := rclient.SetEvent(eventid, event)
    if event.IsOpen {
        loc := event.More["Created-At-Location"]
        err = rgeoclient.GeoAddEvent(loc, eventid)
        if err != nil{
            rgeoclient.RemoveEvent(eventid)
            return err
        }
        // TODO Broadcast
    }
    if members != nil && len(members) != 0{
        members = stringSet(members) // This avoids repeated members
        err := rclient.SetEventMember(eventid, members...)
        if err != nil{
            rgeoclient.RemoveEvent(eventid)
            rgeoclient.GeoRemoveEvent(eventid)
            return err
        }
        var rep pb.ReplicationMsg
        rep.Meta = new(pb.Metadata)
        rep.Meta.MsgMime = make(map[string]string)
        rep.Meta.MsgMime["Content-Type"] = "text/plain"
        rep.Meta.Resource = "/event/"+eventid
        rep.Msg = []byte(event.Name)
        sendMessageToChat(eventid,&rep)
    }
    chat.Members = nil // TODO this changes the pointer of event.Memebers
    StoreAndForwardChat("event:"+eventid, chat)
    return nil
}

func CreateEventHandler(w http.ResponseWriter, r *http.Request) {
    row, err := authenticateRequestPlusRow(r)
    if err != nil{
        send401Unauthorized(w,"Expected valid bearer token.")
        return
    }
    loc := row["loc"]
    user:= row["userid"]
    var event authpb.Event
    var chat authpb.Chat
    bts, err := ioutil.ReadAll(r.Body)
    reader := bytes.NewReader(bts)
    err = json.NewDecoder(reader).Decode(&event)

    if err != nil{
        log.Print("err 1", err)
        send400error(w,"Expected json format with keys: Name, IsOpen, among others. All strings.")
        return
    }
    reader = bytes.NewReader(bts) // Rewind
    err = json.NewDecoder(reader).Decode(&chat)
    if err != nil{
        log.Print("err 2", err)
        send400error(w,"We tried though ")
        return
    }

    // filling up some fields; always overwrite this fields.
    event.Creation = ptypes.TimestampNow()
    chat.Creation = event.Creation
    event.Creator = user
    chat.Creator = user

    eventid := createMsgId(loc+user, event.Creation.Nanos, event.Creation.Seconds)
    chatid := "event:"+eventid
    event.Resource = "/event/"+eventid
    chat.Resource = "/chat/"+chatid
    chat.FromEvent = event.Resource
    event.Chat = chat.Resource

    if event.More == nil{
        event.More = make(map[string]string)
    }
    if _, ok := event.More["Created-At-Location"]; !ok{
        event.More["Created-At-Location"] = loc
    }

    if event.Members != nil {
        event.Members = append(event.Members,user)
        chat.Members = event.Members
    }
    err =  StoreAndForwardEventAndChat(eventid, &event, &chat)
    w.Header().Add("Content-Type", "application/json")
    out:= fmt.Sprintf("{ \"eventid\": \"%s\", \"chatid\": \"%s\"}",eventid,chatid)
    w.Write([]byte(out))
}

func rawWrite(w http.ResponseWriter,msg []byte){
    bts := make([]byte,binary.MaxVarintLen64)
    binary.PutUvarint(bts,uint64(len(msg)))
    w.Write(bts)
    w.Write(msg)
}

type RetrieveGeoRequest struct {
    Location string
    TimeInit string
    TimeFin string
}

func RetrieveGeoHandler(w http.ResponseWriter, r *http.Request) {
    row, err := authenticateRequestPlusRow(r)
    if err != nil{
        send401Unauthorized(w,"Expected valid bearer token.")
        return
    }
    var retrv RetrieveGeoRequest
    dec := json.NewDecoder(r.Body)
    dec.DisallowUnknownFields()
    err = dec.Decode(&retrv)
    if err != nil{
        fmt.Println("json error")
        send400error(w,"Expected json format with keys: chatid, timeInit, timeFin. All strings.")
        return
    }
    loc := retrv.Location
    if retrv.Location == ""{
        loc = row["loc"]
    }
    tinit, err1 := time.Parse(time.RFC3339,retrv.TimeInit)
    tfin, err2 := time.Parse(time.RFC3339,retrv.TimeFin)
    if err1 != nil || err2 != nil {
        fmt.Println("time format error")
        send400error(w,"Expected json time format be in RFC 3339. All strings.")
        return
    }
    c, err := rstore.RetrieveGeoMessages(context.TODO(), loc, tinit, tfin)
    for msg:= range c{
        rawWrite(w,msg)
    }
}

type RetrieveChatRequest struct {
    Chatid string
    TimeInit string
    TimeFin string
}

func RetrieveChatHandler (w http.ResponseWriter, r *http.Request) {
    _, err := authenticateRequest(r)
    if err != nil{
        send401Unauthorized(w,"Expected valid bearer token.")
        return
    }
    var retrv RetrieveChatRequest
    dec := json.NewDecoder(r.Body)
    dec.DisallowUnknownFields()
    err = dec.Decode(&retrv)
    if err != nil{
        fmt.Println("json error")
        send400error(w,"Expected json format with keys: chatid, timeInit, timeFin. All strings.")
        return
    }
    tinit, err1 := time.Parse(time.RFC3339,retrv.TimeInit)
    tfin, err2 := time.Parse(time.RFC3339,retrv.TimeFin)
    if err1 != nil || err2 != nil {
        fmt.Println("time format error")
        send400error(w,"Expected json time format be in RFC 3339. All strings.")
        return
    }
    c, err := rstore.RetrieveChatMessages(context.TODO(), retrv.Chatid, tinit, tfin)
    for msg:= range c{
        rawWrite(w,msg)
    }
}


type AuthlocRequest struct{
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
    var body AuthlocRequest
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

func createGeoMsgId(coorstr string,nanos int32,seconds int64) string{
    long, lat := mydb.CoorFromString(coorstr)
    hash := geohash.Encode(lat,long)[0:6]
    // set message id
    hasher := sha1.New()
    hasher.Write([]byte(hash))
    buf := make([]byte,10)
    binary.BigEndian.PutUint32(buf, uint32(nanos))
    hasher.Write(buf)
    msgid := base64.URLEncoding.EncodeToString(hasher.Sum(nil))[0:7]
    return hash+"/"+msgid
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

func NewReplicationMsgFromRequest(r *http.Request) (*pb.ReplicationMsg, error){
    // Creates a Replication Msg using the headers from the
    // the r request and the chatid information.
    timearrived := ptypes.TimestampNow()
    body, err := ioutil.ReadAll(r.Body)
    if err != nil{
        return nil, err
    }
    repMsg := new(pb.ReplicationMsg)
    repMsg.Msg = body
    repMsg.Meta = new(pb.Metadata)
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

func replicationChatMessage(chatid string, r *http.Request) (*pb.ReplicationMsg, error){
    // Creates a Replication Msg using the headers from the
    // the r request and the chatid information.
    repMsg, err := NewReplicationMsgFromRequest(r)
    if err != nil{
        return nil, err
    }
    msgid := createMsgId(chatid,repMsg.Meta.Arrived.Nanos,repMsg.Meta.Arrived.Seconds)
    resource := fmt.Sprintf("/msg/%s/%s",chatid, msgid)
    repMsg.Meta.Resource = resource
    return repMsg, nil
}

func authenticateRequestPlusRow(r *http.Request) (map[string]string, error){
    // Most basic OAuth token bearer implementation 
    // https://tools.ietf.org/html/rfc6750 2.1 Authorization Request Header Field
    splits := strings.Split(r.Header.Get("Authentication"), " ")
    if splits==nil  ||  len(splits) == 0{
        return nil, errors.New("Authentication token not provided")
    }
    token := splits[len(splits)-1]
    row, err := rclient.GetAllFromToken(token)
    if err != nil{
        return nil, err
    }
    row["token"] = token
    return row, nil
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
    err = sendMessageToChat(chatid,rep) // cuuids are filled in this function
    if err != nil{
        log.Println(err, chatid)
        send500error(w)
    }
    var s pb.UniMsg
    s.Msg = rep.Msg
    s.Meta = rep.Meta
    rstore.StoreChatMessage(chatid,&s) // This is a best-effort basis. Otherwise a Q is req.
}

func WriteMsgHandler(w http.ResponseWriter, r *http.Request) {
    row, err := authenticateRequestPlusRow(r)
    if err != nil{
        log.Println("Could not authenticate", err)
        send401Unauthorized(w,"Could not authenticate")
        return
    }
    rep, err := NewReplicationMsgFromRequest(r)
    if err != nil{
        log.Println(err)
        send400error(w, "Message malformed.")
        return
    }
    rep.Meta.Poster = row["userid"]
    msgid := createGeoMsgId(row["loc"],rep.Meta.Arrived.Nanos,rep.Meta.Arrived.Seconds)
    rep.Meta.Resource = "/geochat/"+msgid
    ids, err := rgeoclient.QueryNearNeighbourhsByCoor(row["loc"])
    if err != nil{
        log.Print("Error 111",row, ids, err)
        send500error(w)
        return
    }
    rep.CUuids =ids
    grpcmsgs <- rep
    uni := &pb.UniMsg{}
    uni.Meta = rep.Meta
    uni.Msg = rep.Msg
    err = rstore.StoreGeoMessage(row["loc"],uni)
    if err != nil{
        log.Print("Error 112",row, ids, err)
        return
    }
}

func GetEventHandler(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    eventid, ok := vars["eventid"]
    if !ok{
        send400error(w,"`/event/{eventid}` expected.")
        return
    }
    event, err := rclient.GetEvent(eventid)
    if err == redis.Nil {
        send404error(w, "Event with id:"+eventid+" was not found")
        return
    }
    bts, err := json.Marshal(event)
    if err != nil {
        send500error(w)
        return
    }
    w.Header().Add("Content-Type","application/json")
    w.Write(bts)
}

func GetChatHandler(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    chatid, ok := vars["chatid"]
    if !ok{
        send400error(w,"`/chat/{chatid}` expected.")
        return
    }
    chat, err := rclient.GetChat(chatid)
    if err == redis.Nil {
        send404error(w, "Chat with id:"+chatid+" was not found")
        return
    }
    bts, err := json.Marshal(chat)
    if err != nil {
        send500error(w)
        return
    }
    w.Header().Add("Content-Type","application/json")
    w.Write(bts)
}

func gRPCworker(addr string, wg *sync.WaitGroup){
    // TODO Add keep alive
    conn, err := grpc.Dial(addr,grpc.WithInsecure(),grpc.WithBlock())
    c := pb.NewWsBackClient(conn)
    // TODO find a better number for context timeout
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
    defer cancel()
    defer conn.Close()
    repCall, err := c.Replicate(ctx)
    if err != nil{
        log.Print("Error 112")
        log.Print(err)
        wg.Done() // Free the lock
        return
    }
    // Responding to incomming grpcmessages to deliver
    for true {
        repMsg, ok := <-grpcmsgs
        if !ok {
            log.Print("Error 113: Trying to read from closed channel.")
            wg.Done() // Free the lock
            return
        }
        err = repCall.Send(repMsg)
        if err != nil{
            log.Print("Error 114: Putting message back in the chan")
            wg.Done() // Free the lock
            // Writing to a chan is blocking if there is not enough space,
            // for this reason the wg is freed before
            grpcmsgs <- repMsg
            return
        }
        // log.Printf("Forwarded message about uuids: %+v",repMsg.CUuids)
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
                log.Println("Launching gRPC worker to: ", addr)
                gRPCworker(addr,&wg) // wg.Done is called inside to avoid a deadlock
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

func send404error(w http.ResponseWriter,str string){
    w.WriteHeader(http.StatusNotFound)
    w.Write([]byte(str))
}

func send401Unauthorized(w http.ResponseWriter,str string){
    w.WriteHeader(http.StatusUnauthorized)
    w.Write([]byte(str))
}


func connectToDBs(pg, red, geo, store string){
    // Set up Postgre
    connected := sync.WaitGroup{}
    pclient = nil
    connected.Add(1)
    go func(postgre string){
        defer connected.Done()
        pclient = mydb.NewPostgre(postgre)
        for pclient == nil{
            time.Sleep(time.Second)
            pclient = mydb.NewPostgre(postgre)
        }
    }(pg)
    // Set up redis
    rclient = nil
    connected.Add(1)
    go func(redis string){
        defer connected.Done()
        rclient = mydb.NewRedis(redis)
        for rclient == nil{
            rclient = mydb.NewRedis(redis)
            time.Sleep(time.Second)
        }
    }(red)
    // Geo redis
    rgeoclient = nil
    connected.Add(1)
    go func(redisGeo string){
        defer connected.Done()
        rgeoclient  = mydb.NewRedis(redisGeo)
        for rgeoclient == nil{
            rgeoclient  = mydb.NewRedis(redisGeo)
            time.Sleep(time.Second)
        }
    }(geo)
    // Store
    rstore = nil
    connected.Add(1)
    go func(redisStorgage string){
        defer connected.Done()
        rstore = mydb.NewRedis(redisStorgage)
        for rgeoclient == nil{
            rgeoclient  = mydb.NewRedis(redisStorgage)
            time.Sleep(time.Second*1)
        }
    }(store)
    connected.Wait()
}

var rstore *mydb.Redis
var rclient *mydb.Redis
var rgeoclient *mydb.Redis
var pclient *mydb.Postgre
var grpcmsgs chan *pb.ReplicationMsg

func main(){
    port := flag.String("port", "localhost:8000", "port to connect (server)")
    redis:= flag.String("redis", "@localhost:6379/0", "format password@IPAddr:port")
    redisGeo := flag.String("redisGeo", "@localhost:6379/1", "format password@IPAddr:port")
    redisStorgage := flag.String("rediStore", "@localhost:6379/2", "format password@IPAddr:port")
    postgre := flag.String("postgre", "authloc:Authloc2846@localhost:5432/postgis_db", "user:password@IPAddr:port/dbname")
    grpcSrv := flag.String("grpc", "localhost:8090", "IPv4.addrs:port")
    flag.Parse()
    fport := *port
    if strings.Index(*port,":") == -1 {
        fport = "localhost:" + *port
    }

    connectToDBs(*postgre,*redis,*redisGeo,*redisStorgage) 

    defer pclient.Pg.Close()
    defer rclient.Redis.Close()
    defer rgeoclient.Redis.Close()
    defer rstore.Redis.Close()

    // gRPC connection handler
    grpcmsgs = make(chan *pb.ReplicationMsg)
    go gRPCmaster(*grpcSrv)


    // Starting server
    fmt.Println("Starting authloc ...") // ,*frontport,"for front, ",*backport," for back")
    fmt.Println("--------------------------------------------------------------- ")

    router := mux.NewRouter()
    router.HandleFunc("/", HomeHandler)
    router.HandleFunc("/auth", AuthlocHandler)
//    router.Handle("/writemsg", TimeRequest(http.HandlerFunc(WriteMsgHandler)))
    router.HandleFunc("/writemsg", WriteMsgHandler) // GeoWriteMsg
    router.HandleFunc("/write/chat/{chatid}", WriteMsgChatHandler)
    router.HandleFunc("/create/chat", CreateChatHandler)
    router.HandleFunc("/create/event", CreateEventHandler)
    router.HandleFunc("/event/{eventid}", GetEventHandler)
    router.HandleFunc("/chat/{chatid}", GetChatHandler)
    router.HandleFunc("/retrieve/chat", RetrieveChatHandler)
    router.HandleFunc("/retrieve/geochat", RetrieveGeoHandler)

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
