// Package main implements a client for Greeter service.
package main

import (
	"flag"
	"fmt"
        authc "github.com/juancki/authloc/authclient"

)

func PleaeReceiveMsgs(mesaclient *authc.Client) {
    c, _ := mesaclient.ReceiveMessageChan()
    for {
        msg, ok := <-c
        if !ok{
            return
        }
        fmt.Println(msg)
    }
}


func TestChat(mesaclient *authc.Client, message, chatidflag *string) {

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
        msg, err := mesaclient.SendBytesToChat(chatid, []byte(*message), nil)
        if err != nil{
            fmt.Println("Error sending message: ",err)
            return
        }
        fmt.Println("Message sent:",msg.Status,msg.Header.Get("Content-Type"))
        // time.Sleep(time.Second)
    }
}

func TestEvent(mesaclient *authc.Client, eventidflag *string) {
    chatid := ""
    if len(*eventidflag)==0{
        eventresp, err := mesaclient.CreateEvent("This chat", "description", nil)
        chatid = eventresp.Chatid
        fmt.Println("eventresp:",eventresp, "err:",err)
    }else {
        fmt.Println("-chat flag passed, avoiding chat creation step")
    }
    mesaclient.SendBytesToChat(chatid,[]byte("ASDF"),nil)
}

func TestGeoChat(mesaclient *authc.Client, message *string) {
    for j:=0;j<10;j++{
        resp, err := mesaclient.SendBytesToGeoChat([]byte(*message),nil)
        if err != nil{
            fmt.Println("Error sending geo message: ",err)
            return
        }
        fmt.Println("Geo Message sent:",resp.Status,resp.Header.Get("Content-Type"))
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
    eventidflag:= flag.String("event", "", "event access.")
    message := flag.String("message", "Empty words mean nothing.", "Message content")
    flag.Parse()
    var mesaclient *authc.Client
    var err error
    if len(*usetoken)==0{
        mesaclient, err = authc.NewClient("http://"+*authloc, *wsholder, *name, *pass, *location)
        if err != nil{
            fmt.Println(err)
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

    TestGeoChat(mesaclient,message)
    TestChat(mesaclient,message,chatidflag)
    TestEvent(mesaclient, eventidflag)
}

