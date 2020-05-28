package main

import (
	"fmt"
	"time"
	mydb "github.com/juancki/authloc/dbutils"
)



func TestManyManyConnections(){
    redis:=  "@localhost:6379/0"
    // Set up redis
    rclient := mydb.NewRedis(redis)
    for rclient == nil{
        rclient =  mydb.NewRedis(redis)
        time.Sleep(time.Second)
    }
    fmt.Println("Connected")
    count := 0
    for ;;count++ {
        err := rclient.Redis.Set("key", "value", time.Hour*24).Err()
        if err != nil {
            fmt.Println(err)
        }
        result := rclient.Redis.Get("key")
        err = result.Err()
        if err != nil{
            fmt.Println(err)
        }
        if result.Val() != "value" {
            fmt.Printf("Error retrieving from Redis, expected `value`, got `%s`\n", result.Val())
        }
        if count % 1e4 == 0{
            fmt.Println(count)
        }
    }
}

func main() {
    TestManyManyConnections()
}
