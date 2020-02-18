package sandstormhttpbridge

import (
	"net/http"

	"log"

	"zenhack.net/go/sandstorm/capnp/grain"
	"zenhack.net/go/sandstorm/capnp/powerbox"
	"zenhack.net/go/sandstorm/capnp/sandstormhttpbridge"
)

type hasId interface {
	SetId(string) error
}

func setId(p hasId, req *http.Request) {
	val := req.Header.Get("X-Sandstorm-Session-Id")
	log.Printf("session ID header value: %q", val)
	p.SetId(val)
}

func GetSessionContext(
	bridge sandstormhttpbridge.SandstormHttpBridge,
	req *http.Request,
) grain.SessionContext {
	res, _ := bridge.GetSessionContext(
		req.Context(),
		func(p sandstormhttpbridge.SandstormHttpBridge_getSessionContext_Params) error {
			setId(p, req)
			return nil
		})
	return res.Context()
}

func GetSessionRequest(
	bridge sandstormhttpbridge.SandstormHttpBridge,
	req *http.Request,
) powerbox.PowerboxDescriptor_List {
	resFuture, release := bridge.GetSessionRequest(
		req.Context(),
		func(p sandstormhttpbridge.SandstormHttpBridge_getSessionRequest_Params) error {
			setId(p, req)
			return nil
		})
	defer release()
	res, err := resFuture.Struct()
	if err != nil {
		panic(err)
	}
	ret, err := res.RequestInfo()
	if err != nil {
		panic(err)
	}
	return ret
}
