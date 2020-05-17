// Package main implements a client for Greeter service.
package main

import (
	"flag"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	authc "github.com/juancki/authloc/authclient"
)


type TestClient struct {
    chatmap map[string]string
    chatcount map[string][]int
    nmessages map[string]int
    chatmem []string
    thisclient string
    thischat string
    mesaclient *authc.Client
}

func (tclient *TestClient) out(){
    sortedKeys := make([]string,0,len(tclient.chatcount))
    for key := range tclient.chatcount{
        sortedKeys = append(sortedKeys,key)
    }
    sort.Strings(sortedKeys)
    for _,k := range sortedKeys{
        s := 0
        v := tclient.chatcount[k]
        for _, summand := range v{
            s += summand
        }
        fmt.Printf("%s len:%d sum:%d\n",k,len(v),s)

    }
    fmt.Println(tclient.chatmap)
}

func sendMessagesToChat(chatid string, mc *TestClient, M int) {
    for ind:= 0; ind<M; ind++{
        msg := fmt.Sprintf("/%s/%02d",mc.thisclient,ind)
        mc.mesaclient.SendBytesToChat(chatid,[]byte(msg),nil)
    }
}

func RegisterIncomingMsg(mc *TestClient) {
    c := mc.mesaclient.ReceiveMessageChan()
    for c != nil{
        rcv, ok := <-c
        if !ok {
            fmt.Println("Channel Closed")
            c = nil
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
                if _,ok:= mc.chatcount[key]; !ok{
                    mc.chatcount[key] = make([]int,0)
                }
                mc.chatcount[key] = append(mc.chatcount[key],num)
            }else{                          // is a message from chatcreation
                mc.chatmap[name] = resource
                chatid := resource[6:]
                go sendMessagesToChat(chatid, mc, mc.nmessages[name])
            }
        }else{
            fmt.Println("this should not happen")
        }
    }
}

func (tclient *TestClient) SetupAndRun(){
    defer tclient.out()
    var err error
    err = tclient.mesaclient.Authenticate()
    if err != nil{
        panic(err)
    }
    // Read first message
    msg := tclient.mesaclient.ReceiveWaitMessage()
    if msg == nil{
        panic("AHHH")
    }
    go RegisterIncomingMsg(tclient)
    time.Sleep(time.Second)

    _, err = tclient.mesaclient.CreateChat(tclient.thischat, "description",tclient.chatmem)
    if err != nil{
        panic(err)
    }
    time.Sleep(10*time.Second)
}

func main() {
    // Set up a connection to the server.
    wsholder := flag.String("wsholder", "127.0.0.1:8080", "address to connect")
    authloc := flag.String("authloc", "localhost:8000", "address to connect")
//    name := flag.String("name", "joan", "name to auth")
    pass := flag.String("pass", "joan", "password")
    usetoken := flag.String("token", "", "Avoid auth and use token")
    location := flag.String("location", "13.13:20.20", "port to connect")
    testN := flag.Int("testN", 4, "Number of clients")
    testM := flag.Int("testM", 1, "Message parameters")
    flag.Parse()
    chatMembers := make([]string,*testN)

    testClients := make([]*TestClient,*testN)

    for ind := range chatMembers{
        chatMembers[ind] = fmt.Sprintf("c%d",ind+1)
    }
    for ind := range chatMembers{
        clt := new(TestClient)
        clt.chatmap = make(map[string]string,*testN)
        clt.chatcount = make(map[string][]int)
        clt.nmessages = make(map[string]int, *testN)
        for jnd := range  chatMembers{
            clt.nmessages[chatMembers[jnd]] = *testM//*(ind+1)*(ind+1)
        }

        clt.thisclient =chatMembers[ind]
        clt.thischat =chatMembers[*testN-ind-1]
        clt.chatmem = chatMembers

        clt.mesaclient = new(authc.Client)
        clt.mesaclient.Token = *usetoken
        clt.mesaclient.WSaddr = *wsholder
        clt.mesaclient.Urlbase = "http://"+*authloc
        clt.mesaclient.Username = clt.thisclient
        clt.mesaclient.Password = *pass
        clt.mesaclient.Location = *location
        testClients[ind] = clt
    }

    wg := sync.WaitGroup{}
    for _, tc := range testClients{
        wg.Add(1)
        go func(tc *TestClient){
            defer wg.Done()
            tc.SetupAndRun()
        }(tc)
    }
    wg.Wait()

}


