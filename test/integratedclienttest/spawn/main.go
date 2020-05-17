

package main

import (
	"bytes"
	"flag"
	"fmt"
	"os/exec"
	"sync"
)

func processTest(arguments string, ind int) {
    testID := fmt.Sprintf("-testID=%d",ind)
    cmd := exec.Command(arguments, testID)
    stdout,_ := cmd.StdoutPipe()
    stderr,_ := cmd.StderrPipe()
    cmd.Run()
    buf := new(bytes.Buffer)
    buf.ReadFrom(stdout)
    globalmap[ind] = buf.String()
    buf = new(bytes.Buffer)
    buf.ReadFrom(stderr)
    fmt.Println(buf.String())

}


var globalmap map[int]string

func main() {
    wsholder := flag.String("wsholder", "127.0.0.1:8080", "address to connect")
    authloc := flag.String("authloc", "localhost:8000", "address to connect")
    location := flag.String("location", "13.13:20.20", "port to connect")
    testN := flag.Int("testN", 4, "Number of clients")
    testM := flag.Int("testM", 1, "Message parameters")
    flag.Parse()
    // Replace `ls` (and its arguments) with something more interesting
    compile := exec.Command("go build ../main.go")
    compile.Run()
    compile.Wait()
    globalmap = make(map[int]string)


    group := &sync.WaitGroup{}
    args := fmt.Sprintf("../main -location=%s -authloc=%s -wsholder=%s -testN=%d -testM=%d",*location,*authloc,*wsholder,*testN,*testM)
    for ind:=0; ind < *testN; ind++{
        globalmap[ind] = ""

        group.Add(1)
        go func(args string, ind int){
            processTest(args,ind)
            group.Done()
        }(args,ind)
    }
    group.Wait()
    fmt.Println(globalmap)

}
