package dbutils

import (
	"database/sql"
	"encoding/base64"
//	"encoding/binary"
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
	"github.com/golang/protobuf/ptypes"
//        timestamppb "google.golang.org/protobuf/types/known/timestamppb"
	authpb "github.com/juancki/authloc/pb"
	cPool "github.com/juancki/wsholder/connectionPool"
	wspb "github.com/juancki/wsholder/pb"
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
    CHAT = "c:"
    CHATSTORAGE = "cs:"
    TOKEN_AUTH = "ta:"
    USER_CONN = "uc:"
    USER_TOKEN = "ut:"
    DEFAULT_Q_RADIUS = 10
    DEFAULT_Q_UNIT = "km"
)



func (rclient *Redis) StoreChatMessage(chatid string, uni *wspb.UniMsg) error{
    time, err := ptypes.Timestamp(uni.Meta.Arrived)
    daymonth := time.Day() // TODO use time.YearDay() and hour to have slots of 15 min.
    bts, err := proto.Marshal(uni)
    if err != nil{
        return err
    }
    key := fmt.Sprintf("%s%s:%d", CHATSTORAGE,chatid,daymonth)
    rclient.Lock()
    err = rclient.Redis.LPush(key,bts).Err()
    rclient.Unlock()
    if err != nil{
        return err
    }
    return nil
}

func (rclient *Redis) RetreiveChatMessages(chatid string, start *time.Time, end *time.Time) ([]byte, error) {
    dayinit := -1
    dayfin := -1
    if start != nil{
        dayinit = start.Day()
    }
    if  end != nil{
        dayinit = end.Day()
    }

    return rclient.queryChatMessages(chatid, dayinit, dayfin)
}

func (rclient *Redis)  queryChatMessages(chatid string, init int, fin int) ([]byte, error){
    // optimally this should not be stored in mem but sent to a channel 
    keypattern := fmt.Sprintf("%s%s:*",CHATSTORAGE,chatid)// TODO Find a query pattern
    rclient.Lock()
    req := rclient.Redis.Keys(keypattern)
    rclient.Unlock()
    err := req.Err()
    if err == redis.Nil{
        return make([]byte,0),nil
    } else if err != nil {
        return nil, err
    }
    result := make([]byte,0)
    for _, key := range req.Val() {
        r := rclient.Redis.LRange(key,0,-1)
        if r.Err() != nil { // There should be any redis.Nil bc of the previous keys lookup
            return result, err
        }
        for _, element := range r.Val(){
            result = append(result,([]byte(element))...)
        }
    }
    return result,nil
}


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


func (rclient *Redis) SetChat(chatid string, chat *authpb.Chat) error{
    bts, err := proto.Marshal(chat)
    if err != nil{
        return err
    }
    rclient.Lock()
    err = rclient.Redis.Set(CHAT+chatid,bts,time.Hour*2).Err()
    rclient.Unlock()
    if err != nil{
        return err
    }
    return nil
}

func (rclient *Redis) SetUserToken(userid string, token string) error{
    return rclient.set(USER_TOKEN+userid,[]byte(token))
}

func (rclient *Redis) GetTokenFromUser(userid string) (string, error) {
    return rclient.Get(USER_TOKEN+userid)
}

func (rclient *Redis) RemoveChat(chatid string) error{
    rclient.Lock()
    err := rclient.Redis.Del(CHAT+chatid).Err()
    rclient.Unlock()
    if err != nil{
        return err
    }
    return nil
}

func (rclient *Redis) set(key string, bts []byte) error{
    rclient.Lock()
    err := rclient.Redis.Set(key, bts,time.Hour*2).Err()
    rclient.Unlock()
    if err != nil{
        return err
    }
    return nil
}

func (rclient *Redis) GetChat(chatid string) (*authpb.Chat, error) {
    var chat authpb.Chat
    rclient.Lock()
    lookup := rclient.Redis.Get(CHAT+chatid)
    rclient.Unlock()
    err := lookup.Err()
    if err != nil{
        return nil, err
    }
    bts, _ := lookup.Bytes()

    err = proto.Unmarshal(bts, &chat)
    if err != nil{
        return nil, err
    }
    return &chat, nil
}

func (rgeoclient *Redis) GeoAddChat(location string, chatid string) error{
    long, lat := coorFromString(location)
    return rgeoclient.GeoAddChatCoor(long, lat, chatid)
}

func (rgeoclient *Redis) GeoUpdateChat(location string, chatid string) error{
    return rgeoclient.GeoAddChat(location,chatid)
}


func (rgeoclient *Redis) GeoAddChatCoor(long float64, lat float64, chatid string) error{
    loc := &redis.GeoLocation{}
    loc.Latitude = lat
    loc.Longitude = long
    loc.Name = chatid

    rgeoclient.Lock()
    err := rgeoclient.Redis.GeoAdd(GEOCHAT,loc).Err()
    rgeoclient.Unlock()
    if err != nil{
        return err
    }
    return nil
}

// Removes uuid from the redis geo database (thread safe).
func (rgeo *Redis) GeoRemoveCuuid(uuid cPool.Uuid) error {
    rgeo.Lock()
    res := rgeo.Redis.ZRem(WORLD,cPool.Uuid2base64(uuid))
    rgeo.Unlock()
    return res.Err()
}

func (rclient *Redis) SetUseridCuuid(userid string, cuuid cPool.Uuid) error{
    // For now only one connection (cuuid) for each user.
    bts := cPool.Uuid2Bytes(cuuid)
    rclient.Lock()
    err := rclient.Redis.Set(USER_CONN+userid, bts, 2*time.Hour).Err()
    rclient.Unlock()
    if err != nil {
        return err
    }
    return nil
}

func castMGetUseridToCuuid(mgetval []interface{}, userids []string) ([]cPool.Uuid, error){
    result := make([]cPool.Uuid,0)
    for ind ,v := range mgetval {
        if v != nil {
            // cPool.Bytes2Cuuid 
            bts, ok := v.(string)
            if !ok {
                // Really an error
                log.Print("Error unpacking user Cuuid in ",userids[ind], mgetval)
                continue
            }
            r := cPool.Bytes2Uuid([]byte(bts))
            result = append(result,r)
        }
    }
    return result, nil
}


func (rclient *Redis) GetCuuidFromUserid(userids ...string) ([]cPool.Uuid, error){
    // For now only one connection (cuuid) for each user.
    uidswitprefix := make([]string,len(userids))
    for ind, uid:=  range userids{
        uidswitprefix[ind] = USER_CONN+uid
    }
    rclient.Lock()
    lookup := rclient.Redis.MGet(uidswitprefix...)
    rclient.Unlock()
    if lookup.Err() != nil {
        return nil, lookup.Err()
    }
    result,_ := lookup.Result()
    return castMGetUseridToCuuid(result ,userids)
}

func (rclient *Redis) SetChatMember(chatid string, userid ...string) error {
    mem := make([]*redis.Z,len(userid))
    for index, value := range userid{
        mem[index] = &redis.Z{}
        mem[index].Member = value
    }
    chatmemberkey := CHATMEMBER + chatid
    rclient.Lock()
    err := rclient.Redis.ZAdd(chatmemberkey, mem...).Err()
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
    if err == redis.Nil{
        return nil
    }else if err != nil {
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

func (rclient *Redis) CheckMemberInChat(chatid string, member string) (bool, error) {
    chatmemberkey := CHATMEMBER + chatid
    rclient.Lock()
    res := rclient.Redis.ZScore(chatmemberkey, member)
    rclient.Unlock()
    err := res.Err()
    if err == redis.Nil { // redis.Nil -> redis key does not exist
        return false, nil
    }else if err != nil{
        return false, err
    }
    return true, nil
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

func (rclient *Redis) Get(key string) (string,error){
    rclient.Lock()
    val, err := rclient.Redis.Get(key).Result()
    rclient.Unlock()
    if err != nil {
        log.Println("Not succesful: ", key, " ", err)
        return "",err
    }
    return val, nil
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

func (rclient *Redis) AppendCuuidIfExists(token string, cuuid cPool.Uuid) (string, error){
    // autloc   calls SetToken on auth step.
    // wsholder calls AppendCuuidIfExists on ws connection.
    // authloc  calls GetCuuidFromToken to retreive Cuuid. On write messages functions
    // UPDATE REDIS
    rclient.Lock()
    r := rclient.Redis.Get(TOKEN_AUTH+token)
    rclient.Unlock()
    err := r.Err()
    if err != nil {
        return "", err
    }
    rclient.Lock()
    err = rclient.Redis.Append(TOKEN_AUTH+token,cPool.Uuid2base64(cuuid)).Err()
    rclient.Unlock()
    if err != nil {
        return "", err
    }
    value, _ := r.Result()
    return value, nil
}

func (rclient *Redis) GetUserFromToken(token string) (string,error){
    // autloc   calls SetToken on auth step.
    // authloc  calls GetUserFromToken to retreive userid. On write messages functions
    rclient.Lock()
    tokenlookup := rclient.Redis.Get(TOKEN_AUTH+token)
    rclient.Unlock()
    if tokenlookup.Err() != nil {
        return "", tokenlookup.Err()
    }
    res, _:= tokenlookup.Result()
    splits := strings.Split(res,":")
    userid := splits[1]
    return userid, nil
}

func (rclient *Redis) GetCuuidFromToken(token string) (string,error){
    // autloc   calls SetToken on auth step.
    // wsholder calls AppendCuuidIfExists on ws connection.
    // authloc  calls GetCuuidFromToken to retreive Cuuid. On write messages functions
    rclient.Lock()
    tokenlookup := rclient.Redis.Get(TOKEN_AUTH+token)
    rclient.Unlock()
    if tokenlookup.Err() != nil {
        return "", tokenlookup.Err()
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

