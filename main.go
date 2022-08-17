package main

import (
    "time"
    "fmt"
    "flag"
    "sync"
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
    origin = flag.String("origin", "", "ws.ptt.cc")
    skipSSLCheck = flag.Bool("skip-ssl-check", false, "skip SSL check")
    skipPrimusPingCheck = flag.Bool("skip-primus-ping", false, "skip primus ping")
)

func main() {
    flag.Parse()
    if *url == "" {
        flag.Usage()
        return
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
    fmt.Println("[IFNO] try to connect: ", *url)

    start := time.Now()

    conn, _, err := dialer.Dial(*url, requestHeader)
    if err != nil {
        fmt.Println("[IFNO] connect error: ", err)
        return
    }
    elapsed := time.Since(start)
    fmt.Println("[IFNO] connected, time cost: ", elapsed, ". Press CTRL+C to disconnect")

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
