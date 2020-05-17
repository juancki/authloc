// Package main implements a client for Greeter service.
package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	// 	"time"

	authpb "github.com/juancki/authloc/pb"
	wspb "github.com/juancki/wsholder/pb"
	"google.golang.org/protobuf/proto"
)


type Person struct {
    Name string
    Pass string
    Loc string
}


type TokenResponse struct {
    Token string
}

type GranMesaClient struct {
    Token string
    Urlbase string
    Username string
    Password string
    Location string
    chats []string
    // Username, pass, ...
}

func getJson(url string, postData []byte, target interface{}) error {
    // var myClient = &http.Client{Timeout: 10 * time.Second}
    r, err := http.Post(url, "application/json", bytes.NewBuffer(postData))
    if err != nil {
        return err
    }
    return json.NewDecoder(r.Body).Decode(target)
}


func (mclient *GranMesaClient) authenticate() (error){
    p := new(Person)
    p.Name  = mclient.Username
    p.Pass = mclient.Password
    p.Loc  = mclient.Location
    url := mclient.Urlbase + "/auth"

    bts, err := json.Marshal(&p)
    if err != nil{ return  err}

    fmt.Println("Accessing: ",url) // " with: ",string(bts))
    if err != nil{ return  err}

    token := new(TokenResponse)
    err = getJson(url, bts, token)
    if err != nil{ return  err}
    mclient.Token = token.Token
    return nil
}

func getNextMessageLength(c net.Conn) (uint64,error){
    bts := make([]byte,binary.MaxVarintLen64)
    _, err := c.Read(bts)
    if err != nil{
        return 0, err
    }
    return binary.ReadUvarint(bytes.NewReader(bts))
}


func (mclient *GranMesaClient) receiveMsgs(ip string, token string){
    // Dial
    conn, err := net.Dial("tcp", ip)
    if err != nil{ fmt.Print(err); return}
    // Send token
    sendbytes := make([]byte,len(mclient.Token)+2)
    sendbytes[0] = ':'
    sendbytes[len(token)+2-1] = '\n'
    copy(sendbytes[1:],token[:])
    writer := bufio.NewWriter(conn)
    writer.WriteString(string(sendbytes))
    writer.Flush()

    rcv := &wspb.UniMsg{}
    rcv.Meta = &wspb.Metadata{}
    rcv.Meta.MsgMime = make(map[string]string)
    for loop:=0; true; loop++ {
        length, err := getNextMessageLength(conn)
        if err != nil{
            fmt.Println(err)
            return
        }
        bts := make([]byte,length)
        n, err := conn.Read(bts)
        if err != nil || uint64(n) != length{
            fmt.Println(err)
            return
        }
        err = proto.Unmarshal(bts,rcv)
        if err != nil{
            fmt.Println(err)
            return
        }
        if rcv == nil || rcv.Meta == nil{
            continue
        }
        if rcv.Meta != nil{
            // fmt.Println(loop,len(rcv.GetMsg()),"B ",rcv.Meta.Resource)
            name := string(rcv.Msg)
            resource := rcv.Meta.Resource
            if strings.HasPrefix(resource,"/msg/"){ // is a message from client
                key := name[0:4]+strings.Split(resource,"/")[2]
                num, _ := strconv.Atoi(name[4:])
                if _,ok:= chatcount[key]; !ok{
                    chatcount[key] = make([]int,0)
                }
                chatcount[key] = append(chatcount[key],num)
            }else{                          // is a message from chatcreation
                chatmap[name] = resource
                chatid := resource[6:]
                go sendMessagesToChat(chatid, mclient, nmessages[name])
            }
        }else{
            fmt.Println("this should not happen")
        }
    }
}

func AuthPost(token, url, bodyType string, body[]byte)(resp *http.Request, err error) {
    post, err := Post(url,bodyType,body)
    if err != nil {
        return nil, err
    }
    post.Header.Add("Authentication","bearer "+token)

    return post, nil
}

func Post(url string, bodyType string, body []byte) (resp *http.Request, err error) {
    req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
    if err != nil {
        return nil, err
    }
    req.Header.Set("Content-Type", bodyType)
    return req, nil
}

type OkResponse struct {
    Ok string
}

type ChatidResponse struct {
    Chatid string `json:"chatid"`
}

func (mclient *GranMesaClient) createchat(name string, description string, members []string) (string, *ChatidResponse , error){
    var chat authpb.Chat
    chat.Name = name
    chat.Description = description
    chat.IsOpen = false
    chat.Members = members
    chat.More = make(map[string]string)
    chat.More["this"] = "that"
    bts, err := json.Marshal(&chat)
    if err != nil{ return "", nil, err}

    url :=  mclient.Urlbase +"/create/chat"
    r, err := AuthPost(mclient.Token,url, "text/plain", bts)
    if err != nil{
        return "", nil, err
    }
    client := new(http.Client)
    msg, err := client.Do(r)
    if err != nil{
        return "", nil, err
    }
    var chatresp ChatidResponse
    json.NewDecoder(msg.Body).Decode(&chatresp)
    if mclient.chats == nil{
        mclient.chats = make([]string,1)
        mclient.chats[0] = chatresp.Chatid
    } else {
        mclient.chats = append(mclient.chats, chatresp.Chatid)
    }
    return msg.Status, &chatresp, nil
}

func (mclient *GranMesaClient) sendmsgtochat(chatid, message string) (*http.Response, error){
    sendurl := mclient.Urlbase+"/write/chat/"+chatid
    r, _ := AuthPost(mclient.Token,sendurl, "text/plain", []byte(message))
    r.Header.Add("WS-META","META")
    client := new(http.Client)
    return client.Do(r)

}


func sendMessagesToChat(chatid string, mclient *GranMesaClient, M int) {
    for ind:= 0; ind<M; ind++{

        msg := fmt.Sprintf("/%s/%02d",thisclient,ind)
        mclient.sendmsgtochat(chatid,msg)
    }
}

var chatmap map[string]string
var chatcount map[string][]int
var nmessages map[string]int
var thisclient string

func main() {
    // Set up a connection to the server.
    wsholder := flag.String("wsholder", "127.0.0.1:8080", "address to connect")
    authloc := flag.String("authloc", "localhost:8000", "address to connect")
//    name := flag.String("name", "joan", "name to auth")
    pass := flag.String("pass", "joan", "password")
    usetoken := flag.String("token", "", "Avoid auth and use token")
    location := flag.String("location", "13.13:20.20", "port to connect")
    testN := flag.Int("testN", 4, "Number of clients")
    testID := flag.Int("testID", -1, "Number of clients")
    testM := flag.Int("testM", 1, "Message parameters")
    flag.Parse()
    clients := make([]string,*testN)
    nmessages = make(map[string]int, *testN)
    chatmap = make(map[string]string,*testN)
    chatcount = make(map[string][]int)
    defer out()
    for ind := range clients{
        clients[ind] = fmt.Sprintf("c%d",ind+1)
        nmessages[clients[ind]] = *testM//*(ind+1)*(ind+1)
    }
    var tid int = *testID
    if (tid < 1 || tid > *testN) {
        panic("Err not valid input parameters")
    }
    thisclient = clients[tid-1]
    thischat := clients[*testN-tid]

    mesaclient := new(GranMesaClient)
    mesaclient.Urlbase = "http://"+*authloc
    mesaclient.Username = thisclient
    mesaclient.Password = *pass
    mesaclient.Location = *location
    var err error
    if len(*usetoken)==0{
        err = mesaclient.authenticate()
        if err != nil{
            panic(err)
        }
    }else{
        fmt.Println("-token flag passed, avoiding authentication step")
        mesaclient.Token = *usetoken
    }
    go mesaclient.receiveMsgs(*wsholder,mesaclient.Token)
    if len(mesaclient.Token) != 0 {
        fmt.Println("Token non empty.")
    }
    time.Sleep(time.Second)

    response, _, err := mesaclient.createchat(thischat, "description", clients)
    if err != nil{
        panic(err)
    }
    fmt.Println("response:",response, "err:",err)

    time.Sleep(10*time.Second)
}

func out(){
    sortedKeys := make([]string,0,len(chatcount))
    for key := range chatcount{
        sortedKeys = append(sortedKeys,key)
    }
    sort.Strings(sortedKeys)
    for _,k := range sortedKeys{
        s := 0
        v := chatcount[k]
        for _, summand := range v{
            s += summand
        }
        fmt.Printf("%s len:%d sum:%d\n",k,len(v),s)

    }
    fmt.Println(chatmap)
}


