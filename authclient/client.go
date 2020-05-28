// Package main implements a client for Greeter service.
package authclient

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

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
    events []string
    isRecvChanOk bool
    receiveChannel chan *wspb.UniMsg
    // Username, pass, ...
}

type Person struct {
    Name string
    Pass string
    Loc string
}

func getJson(url string, postData []byte, target interface{}) error {
    // var myClient = &http.Client{Timeout: 10 * time.Second}
    r, err := http.Post(url, "application/json", bytes.NewBuffer(postData))
    if err != nil {
        return err
    }
    return json.NewDecoder(r.Body).Decode(target)
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

func (mclient *Client) Authenticate() (error){
    return mclient.authenticate()
}

func (mclient *Client) authenticate() (error){
    p := new(Person)
    p.Name  = mclient.Username
    p.Pass = mclient.Password
    p.Loc  = mclient.Location
    url := mclient.Urlbase + "/auth"

    bts, err := json.Marshal(&p)
    if err != nil{
        return  err
    }

    token := new(TokenResponse)
    err = getJson(url, bts, token)
    if err != nil{
        fmt.Println("Error accessing: ",url, " with: ",string(bts))
        return  err
    }
    mclient.Token = token.Token
    return nil
}

func getNextMessageLength(c io.ReadCloser) (uint64,error){
    bts := make([]byte,binary.MaxVarintLen64) // This size should be imported from server lib.
    _, err := c.Read(bts)
    if err != nil{
        return 0, err
    }
    num, _ := binary.ReadUvarint(bytes.NewReader(bts))
    return num, nil
}

func (mclient *Client) retrieveMsgs(out chan<- *wspb.UniMsg, r *http.Response){
    // Dial
    conn := r.Body
    defer close(out)
    for loop:=0; true; loop++ {
        var rcv wspb.UniMsg // New pointer created each time to avoid issues with slow readers
        length, err := getNextMessageLength(conn)
        if err == io.EOF {
            return
        }else if err != nil {
            fmt.Println("Connection error:", err)
            return
        }
        if length == 0 {
            continue
        }
        bts := make([]byte,length)
        n, err := conn.Read(bts)
        if  uint64(n) != length{
            fmt.Println("Connection error: expected to read",length, "B. Read:",n)
            return
        }
        err = proto.Unmarshal(bts,&rcv)
        if err != nil{
            fmt.Println("Unable to unmarshal:",err)
            return
        }
        out <- &rcv
    }
}

type RetrieveGeoRequest struct {
    Location string
    TimeInit string
    TimeFin string
}

func (mclient *Client) RetrieveGeoChan(init,fin time.Time) (<-chan *wspb.UniMsg, error) {
    rcreq := new(RetrieveGeoRequest)
    rcreq.Location = mclient.Location
    rcreq.TimeInit = init.Format(time.RFC3339)
    rcreq.TimeFin = fin.Format(time.RFC3339)
    bts, err := json.Marshal(&rcreq)
    if err!= nil {
        return nil, err
    }
    post, err := authPost(mclient.Token, mclient.Urlbase+"/retrieve/geochat", "application/json", bts)
    if err!= nil {
        return nil, err
    }
    r, err := http.DefaultClient.Do(post)
    c := make(chan *wspb.UniMsg)
    go mclient.retrieveMsgs(c, r)
    return c, nil
}

type RetrieveChatRequest struct {
    Chatid string
    TimeInit string
    TimeFin string
}

func (mclient *Client) RetrieveMessageChan(chatid string, init,fin time.Time) (<-chan *wspb.UniMsg, error) {
    rcreq := new(RetrieveChatRequest)
    rcreq.Chatid = chatid
    rcreq.TimeInit = init.Format(time.RFC3339)
    rcreq.TimeFin = fin.Format(time.RFC3339)
    bts, err := json.Marshal(&rcreq)
    if err!= nil {
        return nil, err
    }
    post, err := authPost(mclient.Token, mclient.Urlbase+"/retrieve/chat", "application/json", bts)
    if err!= nil {
        return nil, err
    }
    r, err := http.DefaultClient.Do(post)
    c := make(chan *wspb.UniMsg)
    go mclient.retrieveMsgs(c, r)
    return c, nil
}

func (mclient *Client) receiveMsgs(out chan<- *wspb.UniMsg){
    // Dial
    defer close(out)
    conn, err := net.Dial("tcp", mclient.WSaddr)
    if err != nil{ fmt.Print(err); return}
    // Send token
    token := mclient.Token
    sendbytes := make([]byte,len(token)+2)
    sendbytes[0] = ':'
    sendbytes[len(token)+2-1] = '\n'
    copy(sendbytes[1:],token[:])
    writer := bufio.NewWriter(conn)
    writer.WriteString(string(sendbytes))
    writer.Flush()
    for loop:=0; true; loop++ {
        var rcv wspb.UniMsg // New pointer created each time to avoid issues with slow readers
        length, err := getNextMessageLength(conn)
        if err != nil{
            fmt.Println(err)
            return
        }
        if length == 0{
            continue
        }
        bts := make([]byte,length)
        n, err := conn.Read(bts)
        if err != nil || uint64(n) != length{
            fmt.Println(err)
            return
        }
        err = proto.Unmarshal(bts,&rcv)
        if err != nil{
            fmt.Println(err)
            return
        }
        out <- &rcv
    }
}

func (mclient *Client) ReceiveMessageChan() (<-chan *wspb.UniMsg, error) {
    c := make(chan *wspb.UniMsg)
    go mclient.receiveMsgs(c)
    return c, nil
}

func authPost(token, url, bodyType string, body[]byte)(resp *http.Request, err error) {
    post, err := post(url,bodyType,body)
    if err != nil {
        return nil, err
    }
    post.Header.Add("Authentication","bearer "+token)

    return post, nil
}

func authGet(token, url, bodyType string, body[]byte) (resp *http.Request, err error) {
    get, err := http.NewRequest("GET", url, bytes.NewBuffer(body))
    if err != nil {
        return nil, err
    }
    get.Header.Add("Authentication","bearer "+token)
    return get, nil
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

type EventidResponse struct {
    Eventid string `json:"eventid"`
    Chatid string `json:"chatid"`
}

func NewEvent(name, description string, members []string) (*authpb.Event){
    var event authpb.Event
    event.Name = name
    event.Description = description
    event.IsOpen = false
    event.Members = members
    event.More = make(map[string]string)

    return &event
}

func (mclient *Client) createevent(name string, description string, members []string) (string, *EventidResponse, error){
    event := NewEvent(name, description, members)
    bts, err := json.Marshal(event)
    if err != nil{ return "", nil, err}

    url :=  mclient.Urlbase +"/create/event"
    r, err := authPost(mclient.Token,url, "application/json", bts)
    if err != nil{
        return "", nil, err
    }
    msg, err := http.DefaultClient.Do(r)
    if err != nil{
        return "", nil, err
    }
    var eventresp EventidResponse
    json.NewDecoder(msg.Body).Decode(&eventresp)
    if mclient.events == nil{
        mclient.events = make([]string,0)
    }
    mclient.events = append(mclient.events, eventresp.Eventid)
    return msg.Status, &eventresp, nil
}

func (mclient *Client) GetEvent(eventid string) (*authpb.Event, error){
    url := mclient.Urlbase + "/event/"+eventid
    r, err := authGet(mclient.Token, url, "application/json", nil)
    if err != nil{
        return nil, err
    }
    msg, err := http.DefaultClient.Do(r)
    if err != nil{
        return nil, err
    }
    var event authpb.Event
    json.NewDecoder(msg.Body).Decode(&event)
    if mclient.events == nil{
        mclient.events = make([]string,0)
    }
    mclient.events = append(mclient.events, eventid)
    return &event, nil
}

func (mclient *Client) CreateEvent(name, description string, members[]string)(*EventidResponse, error){
    if !mclient.IsAuthenticated(){
        mclient.authenticate()
    }
    _, crep, err:= mclient.createevent(name, description, members)
    return crep,err
}

type ChatidResponse struct {
    Chatid string `json:"chatid"`
}

func (mclient *Client) GetChat(chatid string) (*authpb.Chat, error){
    url := mclient.Urlbase + "/chat/"+chatid
    r, err := authGet(mclient.Token, url, "application/json", nil)
    if err != nil{
        return nil, err
    }
    msg, err := http.DefaultClient.Do(r)
    if err != nil{
        return nil, err
    }
    var chat authpb.Chat
    json.NewDecoder(msg.Body).Decode(&chat)
    if mclient.chats == nil{
        mclient.chats = make([]string,0)
    }
    mclient.chats = append(mclient.chats, chatid)
    return &chat, nil
}

func (mclient *Client) CreateChat(name, description string, members[]string)(*ChatidResponse , error){
    if !mclient.IsAuthenticated(){
        mclient.authenticate()
    }
    _, crep, err:= mclient.createchat(name, description, members)
    return crep,err
}

func (mclient *Client) createchat(name string, description string, members []string) (string, *ChatidResponse , error){
    var chat authpb.Chat
    chat.Name = name
    chat.Description = description
    chat.IsOpen = false
    chat.Members = members
    chat.More = make(map[string]string)
    bts, err := json.Marshal(&chat)
    if err != nil{ return "", nil, err}

    url :=  mclient.Urlbase +"/create/chat"
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
        mclient.chats = make([]string,0)
    }
    mclient.chats = append(mclient.chats, chatresp.Chatid)
    return msg.Status, &chatresp, nil
}

func (mclient *Client) sendmsgtochat(chatid string, message []byte) (*http.Response, error){
    return mclient.sendmsgtochatheaders(chatid,message,nil)
}

func (mclient *Client) sendgeomsgheaders(message []byte,headers map[string]string) (*http.Response, error){
    sendurl := mclient.Urlbase+"/writemsg"
    r, _ := authPost(mclient.Token,sendurl, "text/plain", message)
    r.Header.Add("WS-META","META")
    r.Header.Add("Content-Type","text/plain")
    for k,v := range headers{
        r.Header.Add(k,v)
    }
    client := new(http.Client)
    return client.Do(r)
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

func (mclient *Client) SendBytesToGeoChat(message []byte, headers map[string]string)(*http.Response, error){
    // mclient.SendBytesToChat(chatid, message, nil)
    // mclient.SendBytesToChat(chatid, message, headers)
    if !mclient.IsAuthenticated(){
        mclient.authenticate()
    }
    return mclient.sendgeomsgheaders(message,headers)
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

