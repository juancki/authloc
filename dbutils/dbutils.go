package dbutils

import (
	"database/sql"
	"encoding/base64"
	"math"

	//	"encoding/binary"
	"errors"
	"context"
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

func base64_2_string(b64 string) string {
    bts, err := base64.StdEncoding.DecodeString(b64)
    if err != nil{
        return ""
    }
    return string(bts)
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
    MAX_DAYS = 64
    MAX_HOURS = 24
    MAX_HOUR_GOUGE = 24*64
)

func ZERO_TIME () time.Time{
    return Date(1975,1,1)
}

var b64chars = [64]rune {'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H',
                      'I', 'J', 'K', 'L', 'M', 'N', 'O', 'P',
                      'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X',
                      'Y', 'Z', 'a', 'b', 'c', 'd', 'e', 'f',
                      'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n',
                      'o', 'p', 'q', 'r', 's', 't', 'u', 'v',
                      'w', 'x', 'y', 'z', '0', '1', '2', '3',
                      '4', '5', '6', '7', '8', '9', '+', '/'};


var b64invs = [...]int { 62, -1, -1, -1, 63, 52, 53, 54, 55, 56, 57, 58,
                        59, 60, 61, -1, -1, -1, -1, -1, -1, -1, 0, 1, 2, 3, 4, 5,
                        6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20,
                        21, 22, 23, 24, 25, -1, -1, -1, -1, -1, -1, 26, 27, 28,
                        29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40, 41, 42,
                        43, 44, 45, 46, 47, 48, 49, 50, 51 };

func hourGougeToTimeWindow(hourGouge uint64) string{
    day := hourGouge/24
    b1 := day%64
    b2 := hourGouge%24
    r1 := b64chars[b1]
    r2 := b64chars[b2]
    return string([]rune{r1,r2})
}

func timeToHourGouge(t time.Time) uint64{
    t1 := ZERO_TIME()
    return uint64(math.Floor(t.Sub(t1).Hours()))

}

func timeWindowToHourGouge(str string) int64{
    if len(str) != 2{
        return -1
    }
    r1 := rune(str[0])
    r2 := rune(str[1])
    if int(r1) -43>= len(b64invs){
        return -1
    }
    if int(r2) -43>= len(b64invs){
        return -1
    }
    days := b64invs[r1-43]
    hours := b64invs[r2-43]
    if days == -1 {
        return -1
    }
    if hours == -1 || hours > 23 {
        return -1
    }
    return int64(hours+24*days)
}

func currentHourGauge() uint64{
    t1 := ZERO_TIME()
    h := uint64(math.Floor(time.Now().Sub(t1).Hours()))
    return h % MAX_HOUR_GOUGE
}

func timeWindowToTime(str string) (time.Time, time.Time){
    hg := timeWindowToHourGouge(str)
    if hg == -1 {
        panic(fmt.Sprintf("Invalid time window in timeWindowToTime: %s.",str))
    }

    elapsed := (int64(currentHourGauge())-hg)%MAX_HOUR_GOUGE // always positive
    elapsed_duration := time.Duration(elapsed)*time.Hour
    n := time.Now()
    currentH := time.Date(n.Year(),n.Month(),n.Day(),n.Hour(),0,0,0,time.UTC).Add(-elapsed_duration)
    return currentH, currentH.Add(time.Hour) // the interval is [init,fin )
}


func Date(year, month, day int) time.Time {
    return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}

func getTimeWindows(ctx context.Context, init time.Time, fin time.Time) <-chan string{
    // Returns two char(golang rune) string
    c := make(chan string)
    in := timeToHourGouge(init) // Floor
    out := timeToHourGouge(fin.Add(time.Hour))

    go func(i uint64, f uint64){
        defer close(c)
        for j:=i; j<f; j++ {
            select {
            case <-ctx.Done():
                return
            default:
            c  <- hourGougeToTimeWindow(j)
            }
        }
    }(in, out)
    return c
}

func getTimeWindow(t time.Time) string {
    t1 := ZERO_TIME()
    in := uint64(math.Floor(t.Sub(t1).Hours()))
    return hourGougeToTimeWindow(in)
}

func (rclient *Redis) StoreGeoMessage(location string, uni *wspb.UniMsg) error{
    t, err := ptypes.Timestamp(uni.Meta.Arrived)
    if err != nil{
        return err
    }
    long, lat := CoorFromString(location)
    key := getTimeWindow(t) // fmt.Sprintf("%s%s:%s", CHATSTORAGE, dayhour, chatid)
    bts, err := proto.Marshal(uni)
    if err != nil{
        return err
    }
    base64Text := make([]byte, base64.StdEncoding.EncodedLen(len(bts)))
    base64.StdEncoding.Encode(base64Text,bts)
    gloc := &redis.GeoLocation{}
    gloc.Longitude = long
    gloc.Latitude = lat
    gloc.Name = string(base64Text)
    rclient.Lock()
    err = rclient.Redis.GeoAdd(key,gloc).Err()
    rclient.Unlock()
    if err != nil{
        return err
    }
    return nil
}

func (rclient *Redis) StoreChatMessage(chatid string, uni *wspb.UniMsg) error{
    t, err := ptypes.Timestamp(uni.Meta.Arrived)
    dayhour := getTimeWindow(t)
    bts, err := proto.Marshal(uni)
    if err != nil{
        return err
    }
    key := fmt.Sprintf("%s%s:%s", CHATSTORAGE, dayhour, chatid)
    rclient.Lock()
    err = rclient.Redis.LPush(key,bts).Err()
    rclient.Unlock()
    if err != nil{
        return err
    }
    return nil
}

func (rclient *Redis) RetrieveGeoMessages(ctx context.Context, location string, start time.Time, end time.Time) (<-chan []byte, error) {
    long, lat := CoorFromString(location)
    if lat == -1  && long == -1 {
        return nil, errors.New("Bad location string")
    }
    timeWindows := getTimeWindows(ctx, start, end)
    msgChan := make(chan []byte)
    go rclient.queryGeoMessages(ctx, long, lat, timeWindows, msgChan)
    return msgChan, nil
}

func (rclient *Redis)  queryGeoMessages(ctx context.Context, longitude, latitude float64, tws <-chan string,out chan<-[]byte) {
    // optimally this should not be stored in mem but sent to a channel 
    defer close(out)
    errcount := 0
    for tw := range tws {
        key:= tw
        select {
        case <-ctx.Done():
            log.Println("Context canceled", ctx.Err())
            return
        default:
        }

        q := &redis.GeoRadiusQuery{}
        q.Radius = DEFAULT_Q_RADIUS
        q.Unit = DEFAULT_Q_UNIT
        rclient.Lock()
        r := rclient.Redis.GeoRadius(key,longitude, latitude, q)
        rclient.Unlock()
        err := r.Err()
        if err == redis.Nil {
            continue
        } else if err != nil{
            log.Println("Error while reading from redis:",err)
            errcount += 1
            continue
        }
        // log.Println("Accessing: ",key,"Received",len(r.Val()),"elements")
        for _, element := range r.Val(){
            bts := make([]byte, base64.StdEncoding.DecodedLen(len(element.Name)))
            l, err := base64.StdEncoding.Decode(bts, []byte(element.Name))
            if err != nil{
                log.Println("Error while reading from redis:",err)
                errcount += 1
                continue
            }
            out<- bts[:l]
        }
    }
}

func (rclient *Redis) RetrieveChatMessages(ctx context.Context, chatid string, start time.Time, end time.Time) (<-chan []byte, error) {
    timeWindows := getTimeWindows(ctx, start, end)
    msgChan := make(chan []byte)
    go rclient.queryChatMessages(ctx, chatid, timeWindows, msgChan)
    return msgChan, nil
}



func (rclient *Redis)  queryChatMessages(ctx context.Context, chatid string, tws <-chan string,out chan<-[]byte) {
    // optimally this should not be stored in mem but sent to a channel 
    defer close(out)
    errcount := 0
    for tw := range tws {
        key:= fmt.Sprintf("%s%s:%s",CHATSTORAGE,tw,chatid)// TODO Find a query pattern
        select {
        case <-ctx.Done():
            log.Println("Context canceled", ctx.Err())
            return
        default:
        }

        rclient.Lock()
        r := rclient.Redis.LRange(key,0,-1)
        rclient.Unlock()
        err := r.Err()
        if err == redis.Nil {
            continue
        } else if err != nil{
            log.Println("Error while reading from redis:",err)
            errcount += 1
            continue
        }
        // log.Println("Accessing: ",key,"Received",len(r.Val()),"elements")
        for _, element := range r.Val(){
            out<- []byte(element)
        }
    }
}


func CoorFromString(str string) (float64,float64){
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
    long, lat := CoorFromString(location)
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

func (rclient *Redis) GetAllFromToken(token string) (map[string]string,error){
    // keys: loc, userid, created, [cuuid]
    // autloc   calls SetToken on auth step.
    // wsholder calls AppendCuuidIfExists on ws connection.[optional]
    rclient.Lock()
    tokenlookup := rclient.Redis.Get(TOKEN_AUTH+token)
    rclient.Unlock()
    if tokenlookup.Err() != nil {
        return nil, tokenlookup.Err()
    }
    res, _:= tokenlookup.Result()
    splits := strings.Split(res,":")
    result := make(map[string]string)
    result["loc"] = base64_2_string(splits[0])
    result["userid"] = base64_2_string(splits[1])
    result["created"] = base64_2_string(splits[2])
    if len(splits) >= 4{
        result["cuuid"] = splits[len(splits)-1]
    }
    return result, nil
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
    userid := base64_2_string(splits[1])
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

func (rgeoclient *Redis)QueryNearNeighbourhsByCoor(coordinates string) ([]cPool.Uuid, error){
    query := redis.GeoRadiusQuery{}
    query.Radius =  DEFAULT_Q_RADIUS // This radius should be similar to QueryNearNeigh
    query.Unit = DEFAULT_Q_UNIT
    long, lat := CoorFromString(coordinates)
    rgeoclient.Lock() //////////////////////////////////////// LOCK
    query_out := rgeoclient.Redis.GeoRadius(WORLD,long,lat,&query)
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

