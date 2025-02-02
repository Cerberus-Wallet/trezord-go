package server

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Cerberus-Wallet/cerberusd-go/core"
	"github.com/Cerberus-Wallet/cerberusd-go/memorywriter"
	"github.com/Cerberus-Wallet/cerberusd-go/server/api"
	"github.com/Cerberus-Wallet/cerberusd-go/server/status"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

type serverPrivate struct {
	*http.Server
}

type Server struct {
	serverPrivate

	writer io.Writer
}

func New(
	c *core.Core,
	port int,
	stderrWriter io.Writer,
	shortWriter *memorywriter.MemoryWriter,
	longWriter *memorywriter.MemoryWriter,
	version string,
	githash string,
) (*Server, error) {
	longWriter.Log("starting")

	https := &http.Server{
		Addr:              fmt.Sprintf("127.0.0.1:%d", port),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       5 * time.Second,
	}

	allWriter := io.MultiWriter(stderrWriter, shortWriter, longWriter)
	s := &Server{
		serverPrivate: serverPrivate{
			Server: https,
		},
		writer: allWriter,
	}

	r := mux.NewRouter()
	statusRouter := r.PathPrefix("/status").Subrouter()
	postRouter := r.Methods("POST").Subrouter()
	redirectRouter := r.Methods("GET").Path("/").Subrouter()

	status.ServeStatus(statusRouter, c, port, version, githash, shortWriter, longWriter)
	api.ServeAPI(postRouter, c, version, githash, longWriter)

	status.ServeStatusRedirect(redirectRouter)

	var h http.Handler = r

	// Log after the request is done, in the Apache format.
	h = handlers.LoggingHandler(allWriter, h)
	// Log when the request is received.
	h = s.logRequest(h)

	https.Handler = h

	longWriter.Log("server created")
	return s, nil
}

func (s *Server) logRequest(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		text := fmt.Sprintf("%s %s\n", r.Method, r.URL)
		_, err := s.writer.Write([]byte(text))
		if err != nil {
			// give up, just print on stdout
			fmt.Println(err)
		}
		handler.ServeHTTP(w, r)
	})
}

func (s *Server) Run() error {
	return s.ListenAndServe()
}
