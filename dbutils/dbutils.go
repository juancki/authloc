package dbutils

import (
	"database/sql"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	// Third party libs
	redis "github.com/go-redis/redis/v7"
	"github.com/golang/protobuf/proto"
	authpb "github.com/juancki/authloc/pb"
	cPool "github.com/juancki/wsholder/connectionPool"
	_ "github.com/lib/pq"
)

func string2base64(s string) string {
    return base64.StdEncoding.EncodeToString([]byte(s))
}

const(
    WORLD = "world"
    GEOCHAT = "chat"
    CHATMEMBER = "cm:"
    EVENT = "e:"
    TOKEN_AUTH = "ta:"
    USUER_CONN = "uc:"
    DEFAULT_Q_RADIUS = 10
    DEFAULT_Q_UNIT = "km"
)

func coorFromString(str string) (float64,float64){
    // Coordinates are encoded in a string such that
    // "long:lat" with both long and lat with decimals.
    ind := strings.Split(str,":")
    if len(ind) < 2{
        return -1,-1
    }
    long,_ := strconv.ParseFloat(ind[0],64)
    lat,_ := strconv.ParseFloat(ind[1],64)
    return long,lat
}


func (rgeoclient *Redis) GeoAddChatCoor(location string, chatid string, event *authpb.Event) error{
    long, lat := coorFromString(location)
    return rgeoclient.GeoAddChat(long,lat,chatid,event)
}


func (rgeoclient *Redis) GeoAddChat(long float64, lat float64, chatid string, event *authpb.Event) error{
    bts, err := proto.Marshal(event)
    if err != nil{
        return err
    }
    loc := &redis.GeoLocation{}
    loc.Latitude = lat
    loc.Longitude = long
    loc.Name = chatid

    rgeoclient.Lock()
    rgeoclient.Redis.GeoAdd(GEOCHAT,loc)
    err = rgeoclient.Redis.Set(chatid,bts,time.Hour*2).Err()
    rgeoclient.Unlock()
    if err != nil{
        return err
    }
    return nil
}


func (rclient *Redis) SetUseridCuuid(userid string, cuuid cPool.Uuid) error{
    // For now only one connection (cuuid) for each user.
    bts := make([]byte, binary.MaxVarintLen64)
    binary.BigEndian.PutUint64(bts,cuuid)
    rclient.Lock()
    err := rclient.Redis.Set(USUER_CONN+userid, bts, 2*time.Hour).Err()
    rclient.Unlock()
    if err != nil {
        return err
    }
    return nil
}


func (rclient *Redis) GetCuuidFromUserid(userids []string) ([]cPool.Uuid, error){
    // For now only one connection (cuuid) for each user.
    var result []cPool.Uuid
    for _, uid := range userids {
        rclient.Lock()
        lookup := rclient.Redis.Get(USUER_CONN+uid)
        rclient.Unlock()
        if lookup.Err() == nil {
            r, _ := lookup.Bytes()
            result = append(result,binary.BigEndian.Uint64(r))
        }
    }
    return result, nil
}

func (rclient *Redis) SetChatMember(chatid string, userid string) error {
    memb := &redis.Z{}
    memb.Member = userid
    chatmemberkey := CHATMEMBER + chatid
    rclient.Lock()
    err := rclient.Redis.ZAdd(chatmemberkey, memb).Err()
    rclient.Unlock()
    if err != nil {
        return err
    }
    return nil
}


func (rclient *Redis) RemoveChatMember(chatid string, userid string) error {
    memb := &redis.Z{}
    memb.Member = userid
    chatmemberkey := CHATMEMBER + chatid
    rclient.Lock()
    err := rclient.Redis.ZRem(chatmemberkey, memb).Err()
    rclient.Unlock()
    if err != nil {
        return err
    }
    return nil
}

func (rclient *Redis) RemoveAllChatMembers(chatid string) error {
    chatmemberkey := CHATMEMBER + chatid
    rclient.Lock()
    err := rclient.Redis.Del(chatmemberkey).Err()
    rclient.Unlock()
    if err != nil {
        return err
    }
    return nil
}

func (rclient *Redis) GetChatMembers (chatid string) ([]string, error) {
    chatmemberkey := CHATMEMBER + chatid
    rclient.Lock()
    res := rclient.Redis.ZRange(chatmemberkey,0,-1)
    rclient.Unlock()
    err := res.Err()
    if err != nil {
        return nil, err
    }
    zmem, err := res.Result()
    if err != nil {
        return nil, err
    }
    return zmem, nil
}

func (rclient *Redis) SetEvent(eventid string, event *authpb.Event) error {
    bts, err := proto.Marshal(event)
    if err != nil{
        return err
    }
    rclient.Lock()
    res := rclient.Redis.Set(eventid, bts, time.Hour*2)
    rclient.Unlock()
    if res.Err() != nil {
        return res.Err()
    }
    return nil
}

func (rclient *Redis) GetEvent(eventid string) (*authpb.Event,error) {
    rclient.Lock()
    result := rclient.Redis.Get(eventid)
    rclient.Unlock()
    if result.Err() != nil{
        return nil, result.Err()
    }
    bts, err := result.Bytes()
    if err != nil{
        return nil, err
    }
    event := new(authpb.Event)
    err = proto.Unmarshal(bts, event)
    if err != nil{
        return nil, err
    }
    return event, nil
}

func (rclient *Redis) SetToken(token string, name string, loc string) error{
    // autloc   calls SetToken on auth step.
    // wsholder calls AppendCuuidIfExists on ws connection.
    // authloc  calls GetCuuidFromToken to retreive Cuuid. On write messages functions
    value := fmt.Sprintf("%s:%s:%s:",string2base64(loc),string2base64(name),"timeFormated") // TODO fix time formatting
    rclient.Lock()
    err := rclient.Redis.Set(TOKEN_AUTH+token,value,time.Hour*2).Err() // TODO: make it come from config.
    rclient.Unlock()
    if err != nil {
        return err
    }
    return nil
}

func (rclient *Redis) AppendCuuidIfExists(token string, cuuid cPool.Uuid) error{
    // autloc   calls SetToken on auth step.
    // wsholder calls AppendCuuidIfExists on ws connection.
    // authloc  calls GetCuuidFromToken to retreive Cuuid. On write messages functions
    // UPDATE REDIS
    rclient.Lock()
    err := rclient.Redis.Get(token).Err()
    rclient.Unlock()
    if err != nil {
        return err
    }
    rclient.Lock()
    err = rclient.Redis.Append(token,cPool.Uuid2base64(cuuid)).Err()
    rclient.Unlock()
    if err != nil {
        return err
    }
    return nil
}


func (rclient *Redis) GetCuuidFromToken(token string) (string,error){
    // autloc   calls SetToken on auth step.
    // wsholder calls AppendCuuidIfExists on ws connection.
    // authloc  calls GetCuuidFromToken to retreive Cuuid. On write messages functions
    rclient.Lock()
    tokenlookup := rclient.Redis.Get(TOKEN_AUTH+token)
    rclient.Unlock()
    if tokenlookup.Err() != nil {
        log.Print(tokenlookup.Err())
        return "", errors.New("Invalid token")
    }
    res, _:= tokenlookup.Result()
    splits := strings.Split(res,":")
    if len(splits) <4 {
        // The cuuid is the last position of the 
        // redis entry starting from the 4th position element [3].
        // Basically, client needs to connect first to wsholder to 
        // to send a message.
        return "", errors.New("To send a message is req to be connected to the feed.")
    }
    cuuid := splits[len(splits)-1]
    return cuuid,nil
}

func (rgeoclient *Redis) QueryNearChats (cuuid string) ([]string, error){
    // Those chats are supposedly open and can be shared with any
    query := redis.GeoRadiusQuery{}
    query.Radius =  DEFAULT_Q_RADIUS // This radius should be similar to QueryNearNeigh
    query.Unit = DEFAULT_Q_UNIT
    rgeoclient.Lock() //////////////////////////////////////// LOCK
    query_pos := rgeoclient.Redis.GeoPos(GEOCHAT,cuuid)
    rgeoclient.Unlock()///////////////////////////////////// UNLOCK
    geoloc, _ := query_pos.Result()
    if query_pos.Err() != nil || len(geoloc) != 1{
        return nil,query_pos.Err()
    }
    loc := geoloc[0]
    rgeoclient.Lock() //////////////////////////////////////// LOCK
    query_out := rgeoclient.Redis.GeoRadius(GEOCHAT,loc.Longitude,loc.Latitude,&query)
    rgeoclient.Unlock()///////////////////////////////////// UNLOCK
    if query_out.Err() != nil{
        log.Println("query_out: ",query_out)
        return nil,query_out.Err()
    }

    geoquery,_ := query_out.Result()
    chatids :=  make([]string,len(geoquery))
    for index, geoloc := range geoquery{
        chatids[index] = geoloc.Name
    }
    return chatids, nil
}

func (rgeoclient *Redis)QueryNearNeighbourhs (cuuid string) ([]cPool.Uuid, error){
    query := redis.GeoRadiusQuery{}
    query.Radius =  DEFAULT_Q_RADIUS // This radius should be similar to QueryNearNeigh
    query.Unit = DEFAULT_Q_UNIT
    rgeoclient.Lock() //////////////////////////////////////// LOCK
    query_out := rgeoclient.Redis.GeoRadiusByMember(WORLD,cuuid,&query)
    rgeoclient.Unlock()///////////////////////////////////// UNLOCK
    if query_out.Err() != nil{
        log.Println("query_out: ",query_out)
        return nil,query_out.Err()
    }

    geoquery,_ := query_out.Result()
    cuuids :=  make([]cPool.Uuid,len(geoquery))
    for index, geoloc := range geoquery{
        cid, err := cPool.Base64_2_uuid(geoloc.Name)
        if err != nil{
            return nil,err
        }
        cuuids[index] = cid
    }
    return cuuids, nil
}


func verifyAuth(Token string, cuuid string){
    // TODO
}

type Redis struct{
    Redis redis.Client
    sync.Mutex
}


type Postgre struct{
    Pg sql.DB
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
    return &Redis{Redis: *client}

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
    return &Postgre{Pg: *db}

}

