// Package main implements a client for Greeter service.
package authclient

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
// 	"time"

	authpb "github.com/juancki/authloc/pb"
	wspb "github.com/juancki/wsholder/pb"
	"google.golang.org/protobuf/proto"
)


type TokenResponse struct {
    Token string
}

type Client struct {
    Token string
    Urlbase string
    WSaddr string
    Username string
    Password string
    Location string
    chats []string
    isRecvChanOk bool
    receiveChannel chan *wspb.UniMsg
    // Username, pass, ...
}

type Person struct {
    Name string
    Pass string
    Loc string
}


func NewClient(urlbase, wSaddr, name, pass, location string) (*Client, error){
    client := new(Client)
    client.Urlbase = urlbase
    client.WSaddr = wSaddr
    client.Username = name
    client.Password = pass
    client.Location = location
    err := client.authenticate()
    if err != nil{
        return nil, err
    }
    return client, nil
}

func (mclient *Client) authenticate() (error){
    p := new(Person)
    p.Name  = mclient.Username
    p.Pass = mclient.Password
    p.Loc  = mclient.Location
    url := mclient.Urlbase + "/auth"

    bts, err := json.Marshal(&p)
    if err != nil{ return  err}

    fmt.Println("Accessing: ",url, " with: ",string(bts))
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

func (mclient *Client) ReceiveMessage() *wspb.UniMsg {
    if ! mclient.isRecvChanOk {
        mclient.launchReceiver()
    }
    select {
    case msg, ok:= <-mclient.receiveChannel:
        if !ok {
            // No default retry policy
            mclient.isRecvChanOk = false
            return nil
        }
        return msg
    default:
        return nil
    }
}

func (mclient *Client) ReceiveMessageChan() <-chan *wspb.UniMsg {
    if ! mclient.isRecvChanOk {
        mclient.launchReceiver()
    }
    return mclient.receiveChannel
}

func (mclient *Client) launchReceiver() error {
    if !mclient.IsAuthenticated(){
        err := mclient.authenticate()
        return err
    }
    mclient.receiveChannel = make(chan *wspb.UniMsg)
    go receiveMsgs(mclient.WSaddr, mclient.Token, mclient.receiveChannel)
    mclient.isRecvChanOk = true
    return nil
}

func (mclient *Client) ReceiveWaitMessage() *wspb.UniMsg {
    if ! mclient.isRecvChanOk {
        mclient.launchReceiver()
    }
    result, ok :=  <-mclient.receiveChannel
    if !ok {
        // No default retry policy
        mclient.isRecvChanOk = false
        return nil
    }
    return result
}
func receiveMsgs(ip string, token string, respchan chan<- *wspb.UniMsg){
    // Dial
    defer close(respchan)
    conn, err := net.Dial("tcp", ip)
    if err != nil{ fmt.Print(err); return}
    // Send token
    sendbytes := make([]byte,len(token)+2)
    sendbytes[0] = ':'
    sendbytes[len(token)+2-1] = '\n'
    copy(sendbytes[1:],token[:])
    writer := bufio.NewWriter(conn)
    writer.WriteString(string(sendbytes))
    writer.Flush()
    rcv := &wspb.UniMsg{}
    rcv.Meta = &wspb.Metadata{}
    rcv.Meta.MsgMime = make(map[string]string)

    for true {
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
        respchan <- rcv
        fmt.Print(len(rcv.GetMsg()),"B ")
        if rcv == nil || rcv.Meta == nil{
            continue
        }
        fmt.Print(rcv.GetMeta().Resource, " ", rcv.Meta.Poster)
        fmt.Print(rcv.GetMeta().GetMsgMime())
        if tpe, ok := rcv.GetMeta().GetMsgMime()["Content-Type"]; ok && tpe != "bytes"{
            fmt.Print(" `",string(rcv.GetMsg()),"`")
        }
        fmt.Println()
    }
}

func authPost(token, url, bodyType string, body[]byte)(resp *http.Request, err error) {
    post, err := post(url,bodyType,body)
    if err != nil {
        return nil, err
    }
    post.Header.Add("Authentication","bearer "+token)

    return post, nil
}

func post(url string, bodyType string, body []byte) (resp *http.Request, err error) {
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

func (mclient *Client) CreateChat(name, description string)(*ChatidResponse , error){
    if !mclient.IsAuthenticated(){
        mclient.authenticate()
    }
    _, crep, err:= mclient.createchat(name,description)
    return crep,err

}

func (mclient *Client) createchat(name string, description string) (string, *ChatidResponse , error){

    var chat authpb.Chat
    chat.Name = name
    chat.Description = description
    chat.IsOpen = true
    chat.Members = make([]string,1)
    chat.Members[0] = "John"
    chat.More = make(map[string]string)
    chat.More["this"] = "that"
    bts, err := json.Marshal(&chat)
    if err != nil{ return "", nil, err}

    url :=  mclient.Urlbase +"/create/chat"
    fmt.Println("Accessing: ",url, " with: ",string(bts))
    r, err := authPost(mclient.Token,url, "text/plain", bts)
    if err != nil{
        return "", nil, err
    }
    msg, err := http.DefaultClient.Do(r)
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

func (mclient *Client) sendmsgtochat(chatid string, message []byte) (*http.Response, error){
    return mclient.sendmsgtochatheaders(chatid,message,nil)
}

func (mclient *Client) sendmsgtochatheaders(chatid string, message []byte,headers map[string]string) (*http.Response, error){
    sendurl := mclient.Urlbase+"/write/chat/"+chatid
    r, _ := authPost(mclient.Token,sendurl, "text/plain", message)
    r.Header.Add("WS-META","META")
    r.Header.Add("Content-Type","text/plain")
    for k,v := range headers{
        r.Header.Add(k,v)
    }
    client := new(http.Client)
    return client.Do(r)
}
func (mclient *Client) SendBytesToChat(chatid string, message []byte, headers map[string]string)(*http.Response, error){
    // mclient.SendBytesToChat(chatid, message, nil)
    // mclient.SendBytesToChat(chatid, message, headers)
    if !mclient.IsAuthenticated(){
        mclient.authenticate()
    }
    return mclient.sendmsgtochatheaders(chatid,message,headers)
}

func (mclient *Client) IsAuthenticated()bool {
    if len(mclient.Token) != 0 {
        return true
    }else{
        return false
    }
}

func getJson(url string, postData []byte, target interface{}) error {
    // var myClient = &http.Client{Timeout: 10 * time.Second}
    r, err := http.Post(url, "application/json", bytes.NewBuffer(postData))
    if err != nil {
        return err
    }
    return json.NewDecoder(r.Body).Decode(target)
}
