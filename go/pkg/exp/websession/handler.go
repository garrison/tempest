package websession

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"capnproto.org/go/capnp/v3"
	"github.com/gobwas/ws"
	"zenhack.net/go/tempest/capnp/util"
	websession "zenhack.net/go/tempest/capnp/web-session"
	"zenhack.net/go/tempest/go/pkg/exp/websession/websocket"
)

// A Handler implements http.Handler on top of a WebSession. NOTE: this is work in progress
// and does not handle all requests gracefully (sometime codepaths simply panic("TODO")).
type Handler struct {
	Session websession.WebSession
}

// maxNonStreamingBodySize is the maximum size (in bytes) of a request body that we
// will read into memory; anything larger than this must be serviced with one of
// the streaing methods.
const maxNonStreamingBodySize = 1 << 16

// ServeHTTP implements http.Handler.ServeHTTP
func (h Handler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case "GET":
		if req.Header.Get("Upgrade") == "websocket" {
			h.doWebsocket(w, req)
		} else {
			h.doGet(w, req, false)
		}
	case "HEAD":
		h.doGet(w, req, true)
	case "POST", "PUT":
		length, err := strconv.Atoi(req.Header.Get("Content-Length"))
		if err != nil || length < 0 || length > maxNonStreamingBodySize {
			h.doStreamingPostLike(w, req)
			return
		}
		fallthrough
	case "PATCH":
		h.doNonStreamingPostLike(w, req)
	default:
		panic("TODO")
	}
}

func (h Handler) doWebsocket(w http.ResponseWriter, req *http.Request) {
	clientProtos := strings.Split(req.Header.Get("Sec-WebSocket-Protocol"), ",")
	for i, p := range clientProtos {
		clientProtos[i] = strings.TrimSpace(p)
	}
	pw := newPromiseWriter()
	clientW := websession.WebSocketStream_ServerToClient(
		websocket.WriterStream{W: pw},
	)
	fut, rel := h.Session.OpenWebSocket(
		req.Context(),
		func(p websession.WebSession_openWebSocket_Params) error {
			// NOTE: we leave responseStream null, since it isn't actually
			// used by openWebSocket.
			err := placePathContext(p, req, util.ByteStream{})
			if err != nil {
				return err
			}
			argProtos, err := p.NewProtocol(int32(len(clientProtos)))
			if err != nil {
				return err
			}
			for i, v := range clientProtos {
				argProtos.Set(i, v)
			}
			return p.SetClientStream(clientW)
		})
	defer rel()
	replyErr := func(err error) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		pw.fulfill(io.Discard)
	}
	res, err := fut.Struct()
	if err != nil {
		replyErr(err)
		return
	}
	retProto, err := res.Protocol()
	if err != nil {
		replyErr(err)
		return
	}
	srvProto := make([]string, 0, retProto.Len())
	for i := 0; i < retProto.Len(); i++ {
		p, err := retProto.At(i)
		if err != nil {
			replyErr(err)
			return
		}
		srvProto = append(srvProto, p)
	}
	conn, bufRw, _, err := ws.HTTPUpgrader{
		Protocol: func(s string) bool {
			for _, p := range srvProto {
				if s == p {
					return true
				}
			}
			return false
		},
	}.Upgrade(req, w)
	if err != nil {
		replyErr(err)
		return
	}
	if err = bufRw.Flush(); err != nil {
		replyErr(err)
		return
	}
	pw.fulfill(conn)
	srvW := websocket.StreamWriter{
		Context: req.Context(),
		Stream:  res.ServerStream(),
	}
	io.Copy(srvW, conn)
	<-req.Context().Done()
}

// wraps a Writer and blocks until ready is closed, then shells out to w.
// TODO: once go-capnp supports creating promise/fulfiller pairs, drop this
// and use that instead.
type promiseWriter struct {
	ready chan struct{}
	w     io.Writer
}

func newPromiseWriter() *promiseWriter {
	return &promiseWriter{
		ready: make(chan struct{}),
	}
}

func (p *promiseWriter) fulfill(w io.Writer) {
	p.w = w
	close(p.ready)
}

func (p *promiseWriter) Write(data []byte) (n int, err error) {
	<-p.ready
	return p.w.Write(data)
}

// placePathContext fills in the path and context fields of p based on the other arguments.
func placePathContext(p hasPathContext, req *http.Request, responseStream util.ByteStream) error {
	if !strings.HasPrefix(req.RequestURI, "/") {
		return fmt.Errorf("Error: malformed RequestURI (no leading slash): %q", req.RequestURI)
	}
	path := req.RequestURI[1:]
	if err := p.SetPath(path); err != nil {
		return err
	}
	wsCtx, err := p.NewContext()
	if err != nil {
		return err
	}

	return placeContext(wsCtx, req, responseStream)
}

// hasPathContext captures the common path and context fields for many WebSession methods.
type hasPathContext interface {
	SetPath(string) error
	NewContext() (websession.Context, error)
}

func makeResponseStream(w http.ResponseWriter) (*responseStreamImpl, util.ByteStream) {
	srv := newResponseStreamImpl(w)
	client := util.ByteStream_ServerToClient(srv)
	return srv, client
}

// doGet makes a request using the WebSession.get() method.
func (h Handler) doGet(w http.ResponseWriter, req *http.Request, ignoreBody bool) {
	// TODO: block sending the body when ignoreBody = true
	srv, client := makeResponseStream(w)
	respFut, rel := h.Session.Get(req.Context(), func(p websession.WebSession_get_Params) error {
		p.SetIgnoreBody(ignoreBody)
		return placePathContext(p, req, client)
	})
	defer rel()
	relayResponse(w, req, respFut, srv)
}

// doDelete makes a request using the WebSession.delete() method.
func (h Handler) doDelete(w http.ResponseWriter, req *http.Request) {
	srv, client := makeResponseStream(w)
	respFut, rel := h.Session.Delete(req.Context(), func(p websession.WebSession_delete_Params) error {
		return placePathContext(p, req, client)
	})
	defer rel()
	relayResponse(w, req, respFut, srv)
}

// Handle a streaming post or put request.
func (Handler) doStreamingPostLike(w http.ResponseWriter, req *http.Request) {
	panic("TODO")
}

// Handle a non-streaming post, put, or patch request.
func (h Handler) doNonStreamingPostLike(w http.ResponseWriter, req *http.Request) {
	length, err := strconv.Atoi(req.Header.Get("Content-Length"))
	if err != nil {
		replyErr(w, err)
		return
	}
	if length < 0 || length > maxNonStreamingBodySize {
		replyErr(w, fmt.Errorf(
			"Request body too big (%v bytes, max %v)",
			length,
			maxNonStreamingBodySize))
		return
	}
	// TODO(perf): Right now, it isn't safe to block inside a go-capnp placeArgs
	// function. Once that's fixed, we should just allocate the buffer from the
	// arguemnt struct directly, to avoid an extra copy.
	body := make([]byte, length)
	if _, err := io.ReadFull(req.Body, body); err != nil {
		replyErr(w, fmt.Errorf("Reading request body: %w", err))
		return
	}
	switch req.Method {
	case "POST":
		callNonStreamingPostLike(h.Session.Post, w, req, body)
	case "PUT":
		callNonStreamingPostLike(h.Session.Put, w, req, body)
	case "PATCH":
		callNonStreamingPostLike(h.Session.Patch, w, req, body)
	}
}

// Invoke a non-streaming post-like method with arguments based on req and body, and marshal
// the response into w. call should be a method on a WebSession with a suitable argument
// & return type.
func callNonStreamingPostLike[Params nonStreamingPostLikeParams](
	call func(context.Context, func(Params) error) (websession.Response_Future, capnp.ReleaseFunc),
	w http.ResponseWriter,
	req *http.Request,
	body []byte,
) {
	srv, client := makeResponseStream(w)
	respFut, rel := call(req.Context(), func(p Params) error {
		content, err := p.NewContent()
		if err != nil {
			return err
		}
		if err = placeRequestContent(content, req, body); err != nil {
			return err
		}
		return placePathContext(p, req, client)
	})
	defer rel()
	relayResponse(w, req, respFut, srv)
}

// nonStreamingPostLikeParams captures common arguments for WebSession.post, put, and patch.
type nonStreamingPostLikeParams interface {
	hasPathContext
	NewContent() (websession.RequestContent, error)
}

// placeRequestContent populates content based on the request, where body
// has already been read from the request.
func placeRequestContent(
	content websession.RequestContent,
	req *http.Request,
	body []byte,
) error {
	encoding := req.Header.Get("Content-Encoding")
	if encoding != "" {
		if err := content.SetEncoding(encoding); err != nil {
			return err
		}
	}
	contentType := req.Header.Get("Content-Type")
	if contentType != "" {
		if err := content.SetMimeType(contentType); err != nil {
			return err
		}
	}
	return content.SetContent(body)
}

// relayResponse relays a response received from a WebSession back to an http.ResponseWriter.
//
// responseStream should be the value of websession.Context.responseStream that was passed in
// to the request.
func relayResponse(
	w http.ResponseWriter,
	req *http.Request,
	respFut websession.Response_Future,
	responseStream *responseStreamImpl,
) {
	resp, err := respFut.Struct()
	if err != nil {
		replyErr(w, err)
		return
	}

	status, err := responseStatus(req, resp)
	if err != nil {
		replyErr(w, err)
		close(responseStream.ready)
		return
	}

	if resp.Which() == websession.Response_Which_content {
		content := resp.Content()
		body := content.Body()
		if body.Which() == websession.Response_content_body_Which_stream {
			defer body.Stream().Release()
			responseStream.used = true

			select {
			case <-req.Context().Done():
				return
			case size, ok := <-responseStream.size:
				if ok {
					w.Header().Set("Content-Length", strconv.FormatUint(size, 10))
				}
			}
			if err := populateResponseHeaders(w, req, resp); err != nil {
				replyErr(w, err)
				return
			}
			w.WriteHeader(status)
			close(responseStream.ready)
			select {
			case <-req.Context().Done():
			case <-responseStream.done:
			case <-responseStream.shutdown:
			}
			return
		}
	}

	close(responseStream.ready)
	if err := populateResponseHeaders(w, req, resp); err != nil {
		replyErr(w, err)
		return
	}
	data, err := responseBodyBytes(resp)
	if err != nil {
		replyErr(w, err)
		return
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(status)
	w.Write(data)
}

// responseStatus returns the correct HTTP status code for the the response.
// req is the original request that this is a response to.
func responseStatus(req *http.Request, resp websession.Response) (int, error) {
	switch resp.Which() {
	case websession.Response_Which_content:
		successCode := resp.Content().StatusCode()
		status, ok := successCodeStatuses[successCode]
		if ok {
			return status, nil
		}
		return 0, fmt.Errorf("Unknown success code enumerant: %v", successCode)
	case websession.Response_Which_noContent:
		if resp.NoContent().ShouldResetForm() {
			return http.StatusResetContent, nil
		} else {
			return http.StatusNoContent, nil
		}
	case websession.Response_Which_preconditionFailed:
		if (req.Method == "GET" || req.Method == "HEAD") &&
			req.Header.Get("If-None-Match") != "" {

			return http.StatusNotModified, nil
		}
		return http.StatusPreconditionFailed, nil
	case websession.Response_Which_redirect:
		r := resp.Redirect()
		switch {
		case r.IsPermanent() && r.SwitchToGet():
			return http.StatusMovedPermanently, nil
		case !r.IsPermanent() && r.SwitchToGet():
			return http.StatusSeeOther, nil
		case r.IsPermanent() && !r.SwitchToGet():
			return http.StatusPermanentRedirect, nil
		default: // !r.IsPermanent() && !r.SwitchToGet():
			return http.StatusTemporaryRedirect, nil
		}
	case websession.Response_Which_clientError:
		errorCode := resp.ClientError().StatusCode()
		status, ok := clientErrorCodeStatuses[errorCode]
		if ok {
			return status, nil
		}
		return 0, fmt.Errorf("Unknown error code enumerant: %v", errorCode)
	case websession.Response_Which_serverError:
		return http.StatusInternalServerError, nil
	default:
		return 0, fmt.Errorf("Unknown response variant: %v", resp.Which())
	}
}

// Return a []byte with the body of the response. This will return an error for
// streaming responses.
func responseBodyBytes(resp websession.Response) ([]byte, error) {
	switch resp.Which() {
	case websession.Response_Which_content:
		body := resp.Content().Body()
		switch body.Which() {
		case websession.Response_content_body_Which_bytes:
			return body.Bytes()
		case websession.Response_content_body_Which_stream:
			return nil, fmt.Errorf("Can't get []byte for streaming body")
		default:
			return nil, fmt.Errorf("Unknown body variant: %v", body.Which())
		}
	case websession.Response_Which_noContent:
		return nil, nil
	case websession.Response_Which_preconditionFailed:
		return nil, nil
	case websession.Response_Which_redirect:
		return nil, nil
	case websession.Response_Which_clientError:
		return errorBodyBytes(resp.ClientError())
	case websession.Response_Which_serverError:
		return errorBodyBytes(resp.ServerError())
	default:
		return nil, fmt.Errorf("Unknown response variant: %v", resp.Which())
	}
}

// errorBodyBytes is a helper for responseBodyBytes; it handles the clientError and serverError
// cases.
func errorBodyBytes(r hasErrorBody) ([]byte, error) {
	if !r.HasNonHtmlBody() {
		str, err := r.DescriptionHtml()
		return []byte(str), err
	}
	errBody, err := r.NonHtmlBody()
	if err != nil {
		return nil, err
	}
	return errBody.Data()
}

func populateErrorBodyHeaders(dst http.Header, src hasErrorBody) error {
	if !src.HasNonHtmlBody() {
		dst.Set("Content-Type", "text/html")
		return nil
	}
	errBody, err := src.NonHtmlBody()
	if err != nil {
		return err
	}
	return populateHasContentHeaders(dst, errBody)
}

type hasErrorBody interface {
	DescriptionHtml() (string, error)
	NonHtmlBody() (websession.ErrorBody, error)
	HasNonHtmlBody() bool
}

// populateResponseHeaders fills in the response headers based on the contents of the response.
func populateResponseHeaders(w http.ResponseWriter, req *http.Request, resp websession.Response) error {
	isHttps := req.TLS != nil

	setCookies, err := resp.SetCookies()
	if err != nil {
		return err
	}
	for i := 0; i < setCookies.Len(); i++ {
		setCookie := setCookies.At(i)
		name, err := setCookie.Name()
		if err != nil {
			return err
		}
		value, err := setCookie.Value()
		if err != nil {
			return err
		}
		path, err := setCookie.Path()
		if err != nil {
			return err
		}
		cookie := &http.Cookie{
			Name:   name,
			Value:  value,
			Secure: isHttps,
			Path:   path,
		}
		expires := setCookie.Expires()
		switch expires.Which() {
		case websession.Cookie_expires_Which_none:
		case websession.Cookie_expires_Which_absolute:
			cookie.Expires = time.Unix(expires.Absolute(), 0)
		case websession.Cookie_expires_Which_relative:
			cookie.Expires = time.Now().Add(time.Duration(expires.Relative()) * time.Second)
		}
		http.SetCookie(w, cookie)
	}
	// TODO: cachePolicy

	additionalHeaders, err := resp.AdditionalHeaders()
	if err != nil {
		return err
	}
	wHeaders := w.Header()
	for i := 0; i < additionalHeaders.Len(); i++ {
		item := additionalHeaders.At(i)
		name, err := item.Key()
		if err != nil {
			return err
		}
		v, err := item.Value()
		if err != nil {
			return err
		}
		k := http.CanonicalHeaderKey(name)
		if ResponseHeaderFilter.Allows(k) {
			wHeaders[k] = append(wHeaders[k], v)
		}
	}

	switch resp.Which() {
	case websession.Response_Which_content:
		return populateContentResponseHeaders(wHeaders, resp.Content())
	case websession.Response_Which_noContent:
		nc := resp.NoContent()
		if nc.HasETag() {
			etag, err := nc.ETag()
			if err != nil {
				return err
			}
			return setETag(wHeaders, etag)
		}
		return nil
	case websession.Response_Which_preconditionFailed:
		pf := resp.PreconditionFailed()
		if pf.HasMatchingETag() {
			etag, err := pf.MatchingETag()
			if err != nil {
				return err
			}
			return setETag(wHeaders, etag)
		}
		return nil
	case websession.Response_Which_redirect:
		loc, err := resp.Redirect().Location()
		if err != nil {
			return err
		}
		wHeaders.Set("Location", loc)
		return nil
	case websession.Response_Which_clientError:
		return populateErrorBodyHeaders(w.Header(), resp.ClientError())
	case websession.Response_Which_serverError:
		return populateErrorBodyHeaders(w.Header(), resp.ServerError())
	default:
		return fmt.Errorf("Unknown response variant: %v", resp.Which())
	}
}

func setETag(h http.Header, etag websession.ETag) error {
	s, err := eTagStr(etag)
	if err != nil {
		return err
	}
	h.Set("ETag", s)
	return nil
}

func eTagStr(etag websession.ETag) (string, error) {
	value, err := etag.Value()
	if err != nil {
		return "", err
	}
	if etag.Weak() {
		// FIXME: do we need to escape this?
		return "W/\"" + value + "\"", nil
	}
	return "\"" + value + "\"", nil
}

func populateContentResponseHeaders(h http.Header, r websession.Response_content) error {
	if err := populateHasContentHeaders(h, r); err != nil {
		return err
	}
	disposition := r.Disposition()
	switch disposition.Which() {
	case websession.Response_content_disposition_Which_normal:
		// Default
	case websession.Response_content_disposition_Which_download:
		filename, err := disposition.Download()
		if err != nil {
			return err
		}
		h.Set("Content-Disposition", mime.FormatMediaType(
			"attachment",
			map[string]string{"filename": filename},
		))
	}
	if r.HasETag() {
		etag, err := r.ETag()
		if err != nil {
			return err
		}
		return setETag(h, etag)
	}
	return nil
}

func populateHasContentHeaders(dst http.Header, src hasContent) error {
	if src.HasEncoding() {
		v, err := src.Encoding()
		if err != nil {
			return err
		}
		dst.Set("Content-Encoding", v)
	}
	if src.HasLanguage() {
		v, err := src.Language()
		if err != nil {
			return err
		}
		dst.Set("Content-Language", v)
	}
	if src.HasMimeType() {
		contentType, err := src.MimeType()
		if err != nil {
			return err
		}
		// parse & re-format to ensure well-formedness:
		mimeType, params, err := mime.ParseMediaType(contentType)
		if err != nil {
			return err
		}
		contentType = mime.FormatMediaType(mimeType, params)
		dst.Set("Content-Type", contentType)
	}
	return nil
}

type hasContent interface {
	HasEncoding() bool
	Encoding() (string, error)
	HasLanguage() bool
	Language() (string, error)
	HasMimeType() bool
	MimeType() (string, error)
}

// replyErr responds to the request with a 500 status and the given error. If any headers had been
// set in the response, they will be cleared.
func replyErr(w http.ResponseWriter, err error) {
	hdr := w.Header()

	// Delete any headers we may have set when trying to build a normal response.
	for k, _ := range hdr {
		delete(hdr, k)
	}

	body := []byte(err.Error())

	hdr.Set("Content-Type", "text/plain")
	hdr.Set("Content-Length", strconv.Itoa(len(body)))

	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte(err.Error()))
}

// placeContext populates a websession context based on the request, using the supplied
// value for the responseStream field. The reference to responseStream is stolen.
func placeContext(wsCtx websession.Context, req *http.Request, responseStream util.ByteStream) error {
	// Copy in cookies
	reqCookies := req.Cookies()
	wsCookies, err := wsCtx.NewCookies(int32(len(reqCookies)))
	if err != nil {
		return err
	}
	for i, c := range reqCookies {
		wsC := wsCookies.At(i)
		wsC.SetKey(c.Name)
		wsC.SetValue(c.Value)
	}

	// Rig up the response body
	wsCtx.SetResponseStream(responseStream)

	// Process the Accept header
	if accept := req.Header.Get("Accept"); accept != "" {
		types := strings.Split(accept, ",")
		wsTypes, err := wsCtx.NewAccept(int32(len(types)))
		if err != nil {
			return err
		}
		for i, t := range types {
			mimeType, params, err := mime.ParseMediaType(t)
			if err != nil {
				return fmt.Errorf("Error parsing media type at index %v (%q): %w", i, t, err)
			}
			acceptedType := wsTypes.At(i)
			acceptedType.SetMimeType(mimeType)
			qStr, ok := params["q"]
			if ok {
				q, err := strconv.ParseFloat(qStr, 32)
				if err != nil {
					return fmt.Errorf(
						"Error parsing qValue %q in media type %q: %w",
						qStr, t, err)
				}
				acceptedType.SetQValue(float32(q))
			}
		}
	}

	// TODO: acceptEncoding
	// TODO: eTagPrecondition

	additionalHeaders := make(http.Header)
	ContextHeaderFilter.Copy(additionalHeaders, req.Header)
	var numHeaders int32
	for _, vs := range additionalHeaders {
		numHeaders += int32(len(vs))
	}

	dstAdditionalHeaders, err := wsCtx.NewAdditionalHeaders(numHeaders)
	if err != nil {
		return err
	}
	i := 0
	for k, vs := range additionalHeaders {
		for _, v := range vs {
			h := dstAdditionalHeaders.At(i)
			if err := h.SetKey(k); err != nil {
				return err
			}
			if err := h.SetValue(v); err != nil {
				return err
			}
			i++
		}
	}

	//panic("TODO")
	return nil
}
