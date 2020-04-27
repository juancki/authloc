
package main
import (
    "os"
    "fmt"
    "log"
    "time"
    "flag"
    "strings"
    "net/http"

   "github.com/go-redis/redis/v7"
    "github.com/gorilla/handlers"
    "github.com/gorilla/mux"
)


func HomeHandler(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w,"Hello, World Home page")
}

func Authloc(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w,"Auth location")
}

func WriteMsg(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w,"Hello, World page")
}

var redisclient redis.Client
func setupRedis(user string, pass string, addr string){
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: pass, // no password set
		DB:       0,  // use default DB
	})

	pong, err := client.Ping().Result()
	fmt.Println(pong, err)
}

func main(){
    port := flag.String("back-port", "localhost:8000", "port to connect (server)")
    redis:= flag.String("redis", ":@localhost:6379", "user:password@IPAddr:port")
    flag.Parse()
    fport := *port
    if strings.Index(*port,":") == -1{
        fport = "localhost:" + *port
    }
    // Set up redis
    redisconf := strings.Split(*redis,"@")
    redisid := strings.Split(redisconf[0],":")
    redisAddr := redisconf[1]
    // Starting server
    fmt.Println("Starting authlo c...") // ,*frontport,"for front, ",*backport," for back")
    setupRedis(redisid[0],redisid[1],redisAddr)
    fmt.Println("--------------------------------------------------------------- ")


    router := mux.NewRouter()


    router.HandleFunc("/", HomeHandler)
    router.HandleFunc("/auth", Authloc)
    router.HandleFunc("/writemsg", WriteMsg)
    loggedRouter := handlers.LoggingHandler(os.Stdout, router)


    log.Print("Starting Authloc server: ",fport)
    srv := &http.Server{
        Handler: loggedRouter,
        Addr:    fport, // "127.0.0.1:8000",
        // Good practice: enforce timeouts for servers you create!
        WriteTimeout: 15 * time.Second,
        ReadTimeout:  15 * time.Second,
    }
    log.Fatal(srv.ListenAndServe())

}
