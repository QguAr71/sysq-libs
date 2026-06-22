// core/interfaces.go — generyczne interfejsy Input/Output dla Lazarus Network Stack.
//
// LNES jest biblioteką w sysq-libs/pkg/lnet/, bez zależności od Lazarusa.
// Każdy nowy protokół wejściowy implementuje Input, każdy backend implementuje Output.
// Router łączy je przez jawną routing table ([]Route).
//
// CONSTITUTION III-37 (EventBus): LNES produkuje zdarzenia wartościowe
// (proxy.request, proxy.error, proxy.metrics). FUTURE — nie w Etapie 1.
// EventBus: FUTURE — lnet publishes proxy.request, proxy.error, proxy.metrics.

package core

import (
	"context"
	"io"
	"net/http"
)

// Input — moduł wejściowy (protokół). Nasłuchuje na porcie/socketcie/stdin,
// przyjmuje requesty z zewnątrz, tłumaczy na wewnętrzny format Request,
// przekazuje do routera.
type Input interface {
	Name() string
	ListenAndServe() error
	Shutdown(ctx context.Context) error
}

// Output — moduł wyjściowy (backend). Forwarduje wewnętrzny Request
// do zewnętrznego API/usługi, zwraca Response.
type Output interface {
	Name() string
	Forward(ctx context.Context, req *Request) (*Response, error)
}

// Request — ustandaryzowany request wewnętrzny.
// Każdy Input tłumaczy swój protokół na tę strukturę.
type Request struct {
	Method  string
	Path    string
	Headers http.Header
	Body    []byte
}

// Response — ustandaryzowana odpowiedź.
// Każdy Output zwraca tę strukturę; Input streamuje ją do klienta.
type Response struct {
	StatusCode int
	Header     http.Header
	Body       io.ReadCloser
}

// Route — jawne mapowanie input → output.
// Router używa tej tablicy do dispatchu. Żadnej magii, żadnego discovery.
type Route struct {
	Input  string
	Output string
}
