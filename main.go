package main

import (
    "time"
    "fmt"
    "flag"
    "sync"
    "net"
    "net/http"
    "crypto/tls"
    "strings"
    "os"
    "bufio"
    ws "github.com/gorilla/websocket"
)

// https://pkg.go.dev/crypto/tls
type zeroSource struct{}
func (zeroSource) Read(b []byte) (n int, err error) {
    for i := range b {
        b[i] = 0
    }
    return len(b), nil
}

var (
    url = flag.String("url", "wss://ws.ptt.cc/bbs", "wss://ws.ptt.cc/bbs")
    origin = flag.String("origin", "https://term.ptt.cc", "https://term.ptt.cc")
    skipSSLCheck = flag.Bool("skip-ssl-check", false, "skip SSL check")
    skipPrimusPingCheck = flag.Bool("skip-primus-ping", false, "skip primus ping")
    setupEchoService = flag.Bool("echo-service", false, "set up an echo server and connect to it")
)

func main() {
    flag.Parse()
    connectToURL := *url
    if connectToURL == "" {
        flag.Usage()
        return
    }

    if *setupEchoService {
            fmt.Println("[INFO] set up an echo server...")       
            listener, err := net.Listen("tcp", "127.0.0.1:0")
            if err != nil {
                panic(err)
            }
            fmt.Println("[INFO] using port:", listener.Addr().(*net.TCPAddr).Port)

            connectToURL = fmt.Sprintf("ws://127.0.0.1:%d", listener.Addr().(*net.TCPAddr).Port)
            fmt.Println("[INFO] change url to connect echo-service:", connectToURL)

            var upgrader = ws.Upgrader{
                CheckOrigin: func(r *http.Request) bool {
                    fmt.Println("[INFO][ECHO-SERVER] upgrader CheckOrigin")
                    return true
	            },
            }
            http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
                fmt.Println("[INFO][ECHO-SERVER] / init")
                c, err := upgrader.Upgrade(w, r, nil)
                if err != nil {
                    fmt.Println("[INFO][ECHO-SERVER] upgrader.Upgrade error:", err)
                    return
                }
                defer c.Close()
                for {
                    mt, message, err := c.ReadMessage()
                    if err != nil {
                        fmt.Println("[INFO][ECHO-SERVER] Recv Error:", err)
                        break
                    }
                    if err = c.WriteMessage(mt, message); err != nil {
                        fmt.Println("[INFO][ECHO-SERVER] Send Error:", err, "message:", message)
                        break
                    }
                }
            })
        go func() {
            fmt.Println("[INFO] echo-service up")
            http.Serve(listener, nil)
            fmt.Println("[INFO] echo-service down")
        }()
        fmt.Println("[INFO] echo-service setup done")
    }
    
    var requestHeader http.Header
    if *origin != "" {
        requestHeader = http.Header{"Origin": {*origin}}
    }
    
    var dialer ws.Dialer
    if *skipSSLCheck {
        fmt.Println("[IFNO] Skip SSL check")
        // https://github.com/gorilla/websocket/blob/master/client.go
        // https://github.com/gorilla/websocket/blob/master/tls_handshake.go
        dialer = ws.Dialer{
            Subprotocols: []string{}, 
            ReadBufferSize: 2048,
            WriteBufferSize: 2048,
            TLSClientConfig: &tls.Config{
                Rand: zeroSource{},
                InsecureSkipVerify: true,
            },
        }
    } else {
        dialer = ws.Dialer{
            Subprotocols: []string{}, 
            ReadBufferSize: 2048,
            WriteBufferSize: 2048,
        }
    }
    fmt.Println("[IFNO] try to connect to: ", connectToURL)

    start := time.Now()

    conn, resp, err := dialer.Dial(connectToURL, requestHeader)
    if err != nil {
        fmt.Println("[IFNO] connect error:", err)
        fmt.Println("[INFO] resp.StatusCode:", resp.StatusCode)
        return
    }
    elapsed := time.Since(start)
    fmt.Println("[IFNO] connected, time cost: ", elapsed, ", response code:", resp.StatusCode, ", Press CTRL+C to disconnect")

    wg := sync.WaitGroup{}
    wg.Add(2)

    go func() {
        defer func() {
            wg.Done()
        }()
        for {
            _, p, err := conn.ReadMessage()
            if err == nil {
                cmd := string(p)
                fmt.Println("SERVER> ", cmd)
                if *skipPrimusPingCheck == false {
                    if strings.Index(cmd, "\"primus::ping::") >= 0 {
                        text := fmt.Sprintf("\"primus::ping::%d\"", time.Now().UnixMilli())
                        fmt.Println("CLIENT:AUTO> [",text,"]")
                        conn.WriteMessage(ws.TextMessage, []byte(text))
                    }
                }
            } else {
                fmt.Println("SERVER:ERROR> ", err)
                break
            }
        }
        fmt.Println("[IFNO] server error")
    }()

    go func() {
        defer func() {
            wg.Done()
        }()
        reader := bufio.NewReader(os.Stdin)
        for {
            text, err := reader.ReadString('\n')
            if err != nil {
                fmt.Println("CLIENT:ERROR> ", err)
                break
            } else if text == "\n" {
                continue
            } else {
                text = text[:len(text)-1]
                fmt.Println("CLIENT:SEND> [", text,"]")
                if err := conn.WriteMessage(ws.TextMessage, []byte(text)) ; err != nil {
                    fmt.Println("CLIENT:SEND:ERROR> ", err)
                } else {
                    fmt.Println("CLIENT:SEND:DONE>")
                }
            }
        }
    }()

    wg.Wait()
    fmt.Println("Disconnected")
}
