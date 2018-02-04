package main

import (
	"net/http"
	"net/rpc"
	"net/rpc/jsonrpc"
	"os"

	"golang.org/x/net/websocket"

	"github.com/gorilla/mux"
)

var (
	rootDomain = defaultTo(os.Getenv("ROOT_DOMAIN"), "local.sandstorm.io")
	listenAddr = defaultTo(os.Getenv("LISTEN_ADDR"), ":8000")
	apps       = map[string]string{
		"app-1": ":8101",
		"app-2": ":8102",
		"app-3": ":8103",
	}
)

func defaultTo(val, def string) string {
	if val == "" {
		return def
	}
	return val
}

func SetAppHeaders(w http.ResponseWriter) {

}

func main() {
	r := mux.NewRouter()
	rootR := r.Host(rootDomain).Subrouter()

	rootR.Path("/rpc").Handler(websocket.Handler(func(conn *websocket.Conn) {
		server := rpc.NewServer()
		server.Register(&RpcServer{})
		server.ServeCodec(jsonrpc.NewServerCodec(conn))
	}))
	rootR.PathPrefix("/").Handler(http.FileServer(http.Dir("static")))

	r.Host("{app}." + rootDomain).HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ServeApp(mux.Vars(req)["app"], w, req)
	})

	http.Handle("/", r)
	http.ListenAndServe(listenAddr, nil)
}

func badRequest(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte(`<!doctype html>
		<html>
			<head>
				<meta charset="utf-8" />
				<title>Bad Request</title>
			</head>
			<body>
				<p>Bad Request: ` + msg + `</p>
			</body>
		</html>`))
}
