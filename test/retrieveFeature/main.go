// Package main implements a client for Greeter service.
package main

import (
	"flag"
	"time"
	"fmt"
        authc "github.com/juancki/authloc/authclient"

)

func PleaeReceiveMsgs(mesaclient *authc.Client) {
    count := 0
    defer fmt.Println("Total number of received  messages:", count)
    c, _ := mesaclient.ReceiveMessageChan()
    for {
        _, ok := <-c
        if !ok{
            return
        }
        count += 1
    }
}



func retrieveFromChat(mesaclient *authc.Client, chatidflag, message *string){
    chatid := *chatidflag
    if len(*chatidflag)==0{
        chat, err := mesaclient.CreateChat("This chat", "description", nil)
        fmt.Println("chat:", chat, "err:",err)
        chatid = chat.Chatid
    }else {
        fmt.Println("-chat flag passed, avoiding chat creation step")
    }
    fmt.Println("sending only 10 messages to chat: ",chatid)
    for i:=0;i<10;i++ {
        pre := (fmt.Sprintf("CHAT:%d:",i))
        _, err := mesaclient.SendBytesToChat(chatid, []byte(pre+*message), nil)
        if err != nil{
            fmt.Println("Error sending message: ",err)
            return
        }
        // fmt.Println("Message sent:",msg.Status,msg.Header.Get("Content-Type"))
        // time.Sleep(time.Second)
    }
    fmt.Println("Restore the messages")
    c, err := mesaclient.RetrieveMessageChan(chatid, time.Now().Add(-time.Hour), time.Now())
    if err != nil{
        fmt.Println("Error creating retrieve chan",err)
        return
    }
    for {
        msg, ok := <-c
        if !ok{
            fmt.Println("Retrieve chan closed.")
            return
        }
        fmt.Println("Message rcv:",string(msg.Msg),msg.Meta.MsgMime["Content-Type"])
    }
}


func main() {
    // Set up a connection to the server.
    wsholder := flag.String("wsholder", "127.0.0.1:8080", "address to connect")
    authloc := flag.String("authloc", "localhost:8000", "address to connect")
    name := flag.String("name", "joan", "name to auth")
    pass := flag.String("pass", "joan", "password")
    usetoken := flag.String("token", "", "Avoid auth and use token")
    location := flag.String("location", "13.13:20.20", "port to connect")
    chatidflag := flag.String("chat", "", "chat to write to")
    message := flag.String("message", "Empty words mean nothing.", "Message content")
    flag.Parse()
    var mesaclient *authc.Client
    var err error
    if len(*usetoken)==0{
        mesaclient, err = authc.NewClient("http://"+*authloc, *wsholder, *name, *pass, *location)
        if err != nil{
            fmt.Println(*chatidflag)
            fmt.Println(err)
            return
        }
    }else{
        fmt.Println("-token flag passed, avoiding authentication step")
        mesaclient = &authc.Client{
            Token:*usetoken,
            Urlbase: "http://"+*authloc,
            WSaddr: *wsholder,
            Username: *name,
            Password: *pass,
            Location: *location}
    }
    fmt.Println("Token: ", mesaclient.Token)
    go PleaeReceiveMsgs(mesaclient)
    retrieveFromChat(mesaclient,chatidflag,message)
    retrieveFromGeo(mesaclient,message)

}

func retrieveFromGeo(mesaclient *authc.Client, message *string){
    fmt.Println("sending only 10 messages to geo chat")
    for i:=0;i<10;i++ {
        pre := (fmt.Sprintf("GEO:%d:",i))
        _, err := mesaclient.SendBytesToGeoChat([]byte(pre+*message), nil)
        if err != nil{
            fmt.Println("Error sending message: ",err)
            return
        }
        // fmt.Println("Message sent:",msg.Status,msg.Header.Get("Content-Type"))
        // time.Sleep(time.Second)
    }
    fmt.Println("Restore the geo messages")
    c, err := mesaclient.RetrieveGeoChan(time.Now().Add(-time.Hour), time.Now())
    if err != nil{
        fmt.Println("Error creating retrieve chan",err)
        return
    }
    for msg := range c {
        fmt.Println("Message rcv:",string(msg.Msg),msg.Meta.MsgMime["Content-Type"])
    }
    fmt.Println("Retrieve chan closed.")
}
