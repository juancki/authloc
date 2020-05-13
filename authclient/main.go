// Package main implements a client for Greeter service.
package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
        "bufio"
	"flag"
	"fmt"
	"net"
	"net/http"

	wspb "github.com/juancki/wsholder/pb"
	authpb "github.com/juancki/authloc/pb"
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

func getJson(url string, postData []byte, target interface{}) error {
    // var myClient = &http.Client{Timeout: 10 * time.Second}
    r, err := http.Post("http://"+url, "application/json", bytes.NewBuffer(postData))
    if err != nil {
        return err
    }
    return json.NewDecoder(r.Body).Decode(target)
}


func authenticate(name string, pass string, location string, url string) (string,error){
    p := new(Person)
    p.Name = name
    p.Pass = pass
    p.Loc = location

    bts, err := json.Marshal(&p)
    if err != nil{ return "", err}

    fmt.Println("Accessing: ",url, " with: ",string(bts))
    if err != nil{ return "", err}

    token := new(TokenResponse)
    err = getJson(url, bts, token)
    if err != nil{ return "", err}
    return token.Token,nil
}

func getNextMessageLength(c net.Conn) (uint64,error){
    bts := make([]byte,binary.MaxVarintLen64)
    _, err := c.Read(bts)
    if err != nil{
        return 0, err
    }
    return binary.ReadUvarint(bytes.NewReader(bts))
}


func receiveMsgs(ip string, token string){
    // Dial
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

    for true {
        rcv := &wspb.UniMsg{}
        rcv.Meta = &wspb.Metadata{}
        rcv.Meta.MsgMime = make(map[string]string)
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

type OkResponse struct {
    Ok string
}

type ChatidResponse struct {
    Chatid string `json:"chatid"`
}

func createchat(name string, description string, url string) (string, *ChatidResponse , error){

    var chat authpb.Chat
    chat.Name = name
    chat.Description = description
    chat.IsOpen = true
    chat.Members = make([]string,1)
    chat.Members[0] = "John"
    chat.More = make(map[string]string)
    chat.More["this"] = "that"
    fmt.Printf("Chat: %+v\n",&chat)
    bts, err := json.Marshal(&chat)
    if err != nil{ return "", nil, err}

    fmt.Println("Accessing: ",url, " with: ",string(bts))
    if err != nil{ return "", nil, err}

    r, err := http.Post("http://"+url, "application/json", bytes.NewBuffer(bts))
    if err != nil{ return "", nil, err}
    var chatresp ChatidResponse
    json.NewDecoder(r.Body).Decode(&chatresp)
    return r.Status, &chatresp, nil
}


func main() {
    // Set up a connection to the server.
    wsholder := flag.String("wsholder", "127.0.0.1:8080", "address to connect")
    authloc := flag.String("authloc", "localhost:8000", "address to connect")
    name := flag.String("name", "joan", "name to auth")
    pass := flag.String("pass", "joan", "password")
    usetoken := flag.String("token", "", "Avoid auth and use token")
    location := flag.String("location", "13.13:20.20", "port to connect")
    flag.Parse()
    token := ""
    var err error
    if len(*usetoken)==0{
        token, err = authenticate(*name,*pass,*location,*authloc+"/auth")
        if err != nil{
            fmt.Println(err)
        }
    }else{
        fmt.Println("-token flag passed, avoiding authentication step")
        token = *usetoken
    }
    fmt.Println("Token: ", token)
    go receiveMsgs(*wsholder,token)

    response, chat, err := createchat("This chat", "description", *authloc+"/create/chat")
    fmt.Println("response:",response, "chat:", chat, "err:",err)

    sendurl := "http://"+*authloc+"/write/chat/"+chat.Chatid
    r, err := Post(sendurl, "application/json", []byte("this msg"))
    r.Header.Add("WS-META","METAA")
    r.Header.Add("Authentication","bearer "+token)
    client := new(http.Client)
    msg, err := client.Do(r)
    fmt.Println(msg,err)
}

func Post(url string, bodyType string, body []byte) (resp *http.Request, err error) {
    req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
    if err != nil {
        return nil, err
    }
    req.Header.Set("Content-Type", bodyType)
    return req, nil
}
