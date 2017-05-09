// Package websession provides helpers for speaking HTTP over the sandstorm API.
//
// The main thing provided by this package HandlerWebSession, which allows
// converting package "net/http"'s Handler type to Sandstorm WebSessions. This
// makes writing web servers pretty much the same between sandstorm and
// non-sandstorm environments.
package websession // import "zenhack.net/go/sandstorm/websession"

// Copyright (c) 2016 Ian Denhardt <ian@zenhack.net>
//
// All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import (
	"bytes"
	"golang.org/x/net/context"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"zenhack.net/go/sandstorm/capnp/grain"
	capnp "zenhack.net/go/sandstorm/capnp/websession"
	"zenhack.net/go/sandstorm/internal/errors"
)

func FromHandler(h http.Handler) HandlerWebSession {
	return HandlerWebSession{h}
}

type HandlerWebSession struct {
	handler http.Handler
}

// make path an absolute path, by prepending a slash if it is not already
// present. http.ServeMux will give us a redirect otherwise, and sandstorm
// will then give us the relative path again, resulting in an infinite
// redirect loop.
//
// As far as I(zenhack) know sandstorm always leaves off the leading slash,
// but I haven't found documentation actually saying so (yet).
func makeAbsolute(path string) string {
	if strings.HasPrefix(path, "/") {
		return path
	} else {
		return "/" + path
	}
}

func contextPopulateRequest(wsCtx *capnp.WebSession_Context, req *http.Request) error {
	// TODO:
	//
	// * cookies
	// * accept
	// * acceptEncoding
	// * eTagPrecondition
	//
	// We don't need responseStream, as that's handled in buildResponse.
	return nil
}

func (h HandlerWebSession) handleRequest(goCtx context.Context, method string, args requestArgs,
	headers map[string][]string,
	body io.ReadCloser,
	wsResponse *capnp.WebSession_Response) error {

	if headers == nil {
		headers = make(map[string][]string)
	}

	path, err := args.Path()
	if err != nil {
		return err
	}
	wsCtx, err := args.Context()
	if err != nil {
		return err
	}

	request := http.Request{
		Method:     method,
		RequestURI: makeAbsolute(path),
		Header:     headers,
		Body:       body,
	}
	request.URL, err = url.ParseRequestURI(request.RequestURI)
	if err = contextPopulateRequest(&wsCtx, &request); err != nil {
		return err
	}

	runHandler(h.handler, &request, func(response *http.Response) {
		buildCapnpResponse(goCtx, response, &wsCtx, wsResponse)
	})
	return nil
}

// Arguments common to all request types
type requestArgs interface {
	Path() (string, error)
	Context() (capnp.WebSession_Context, error)
}

// Websession has both PutContent and PostContent, which are identical
// structs. Merging these in capnp would require ABI breakage. For now,
// we abstract away the differences with this interface.
type pContent interface {
	MimeType() (string, error)
	Content() ([]byte, error)
	Encoding() (string, error)
}

func (h HandlerWebSession) Get(args capnp.WebSession_get) error {
	ignoreBody := args.Params.IgnoreBody()
	method := ""
	if ignoreBody {
		method = "HEAD"
	} else {
		method = "GET"
	}
	return h.handleRequest(args.Ctx, method, args.Params, nil, nil, &args.Results)
}

func (h HandlerWebSession) Post(args capnp.WebSession_post) error {
	content, err := args.Params.Content()
	if err != nil {
		return err
	}
	return h.handleP(args.Ctx, "POST", args.Params, content, &args.Results)
}

// Request handling logic common to Put, Post, and Patch.
func (h HandlerWebSession) handleP(ctx context.Context, method string, args requestArgs, content pContent,
	wsResponse *capnp.WebSession_Response) error {

	payload, err := content.Content()
	if err != nil {
		return err
	}
	mimeType, err := content.MimeType()
	if err != nil {
		return err
	}
	body := ioutil.NopCloser(bytes.NewBuffer(payload))
	headers := map[string][]string{
		"Content-Type": {mimeType},
	}
	encoding, err := content.Encoding()
	if err == nil {
		headers["Content-Encoding"] = []string{encoding}
	}
	return h.handleRequest(ctx, method, args, headers, body, wsResponse)
}

func (h HandlerWebSession) Put(args capnp.WebSession_put) error {
	content, err := args.Params.Content()
	if err != nil {
		return err
	}
	return h.handleP(args.Ctx, "PUT", args.Params, content, &args.Results)
}

func (h HandlerWebSession) Delete(args capnp.WebSession_delete) error {
	return h.handleRequest(args.Ctx, "DELETE", args.Params, nil, nil, &args.Results)
}

func (h HandlerWebSession) Patch(args capnp.WebSession_patch) error {
	content, err := args.Params.Content()
	if err != nil {
		return err
	}
	return h.handleP(args.Ctx, "PATCH", args.Params, content, &args.Results)
}

// Websession stubs:

func (h HandlerWebSession) PostStreaming(capnp.WebSession_postStreaming) error {
	return errors.NotImplemented
}

func (h HandlerWebSession) PutStreaming(capnp.WebSession_putStreaming) error {
	return errors.NotImplemented
}

//// In websocket.go:
// func (h HandlerWebSession) OpenWebSocket(p capnp.WebSession_openWebSocket) error {

// WEBDAV stuff

func (h HandlerWebSession) Propfind(capnp.WebSession_propfind) error {
	return errors.NotImplemented
}

func (h HandlerWebSession) Proppatch(capnp.WebSession_proppatch) error {
	return errors.NotImplemented
}

func (h HandlerWebSession) Mkcol(capnp.WebSession_mkcol) error {
	return errors.NotImplemented
}

func (h HandlerWebSession) Copy(capnp.WebSession_copy) error {
	return errors.NotImplemented
}

func (h HandlerWebSession) Move(capnp.WebSession_move) error {
	return errors.NotImplemented
}

func (h HandlerWebSession) Lock(capnp.WebSession_lock) error {
	return errors.NotImplemented
}

func (h HandlerWebSession) Unlock(capnp.WebSession_unlock) error {
	return errors.NotImplemented
}

func (h HandlerWebSession) Acl(capnp.WebSession_acl) error {
	return errors.NotImplemented
}

func (h HandlerWebSession) Report(capnp.WebSession_report) error {
	return errors.NotImplemented
}

func (h HandlerWebSession) Options(capnp.WebSession_options) error {
	return errors.NotImplemented
}

// UiView stubs.

func (h HandlerWebSession) GetViewInfo(p grain.UiView_getViewInfo) error {
	return errors.NotImplemented
}

func (h HandlerWebSession) NewSession(p grain.UiView_newSession) error {
	handler := WithSessionContext(h.handler, p.Params.Context())

	client := capnp.WebSession_ServerToClient(HandlerWebSession{
		handler: handler,
	}).Client
	p.Results.SetSession(grain.UiSession{Client: client})
	return nil
}

func (h HandlerWebSession) NewRequestSession(p grain.UiView_newRequestSession) error {
	return errors.NotImplemented
}

func (h HandlerWebSession) NewOfferSession(p grain.UiView_newOfferSession) error {
	return errors.NotImplemented
}
