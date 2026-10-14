package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/satoricorp/tennis"
)

// maxRequestBody bounds what a handler will read. A write of many documents is
// legitimately large; unbounded is not — without a cap a runaway client feeds
// the JSON decoder forever.
const maxRequestBody = 32 << 20

// The HTTP API exists so tennis is usable from languages that will never have
// a Go SDK. One binary speaks JSON on a local port and a client in any language
// is a hundred lines, which beats maintaining five real SDKs.
//
// It binds to loopback by default and has no authentication, because it is an
// interface to a local file rather than a service. --addr will happily bind
// elsewhere; do not, unless you have put something in front of it.
func cmdServe(args []string) error {
	fs_ := flag.NewFlagSet("serve", flag.ExitOnError)
	dbPath := fs_.String("db", defaultDB(), "database file")
	addr := fs_.String("addr", "127.0.0.1:8817", "listen address (loopback only unless you know why)")
	fs_.Parse(args)

	db, err := open(*dbPath, false)
	if err != nil {
		return err
	}
	defer db.Close()

	if host, _, err := net.SplitHostPort(*addr); err == nil && !isLoopback(host) {
		fmt.Fprintf(os.Stderr,
			"tennis: warning — binding to %s exposes an unauthenticated API on your network\n", *addr)
	}

	srv := &http.Server{
		Addr:              *addr,
		Handler:           newServeMux(db),
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Ctrl-C finishes in-flight requests rather than killing them mid-write.
	// SQLite would recover through the WAL either way; exiting cleanly means it
	// never has to.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	fmt.Fprintf(os.Stderr, "tennis: serving %s on http://%s\n", db.Path(), *addr)

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		fmt.Fprintln(os.Stderr, "tennis: shutting down")
		// Deliberately context.Background(), not ctx: by this point ctx.Done()
		// has already fired, so deriving the timeout from ctx would hand
		// WithTimeout an already-canceled parent and the deadline would be
		// instantly exceeded — Shutdown would kill every in-flight request
		// immediately instead of giving them 5 seconds. A fresh context is
		// what makes the grace period real.
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}

// nsCache holds one open *tennis.Namespace per name for the life of the
// server. Opening a namespace rebuilds its embedder, and for the built-in model
// that means reading the whole ~123MB weights file; paying that on every
// request made each call cost a model load. One handle per namespace amortizes
// it to once. The mutex also serializes first-time creates, so two concurrent
// first writes to a new namespace no longer race on the insert.
type nsCache struct {
	db   *tennis.DB
	mu   sync.Mutex
	open map[string]*tennis.Namespace
}

func newNSCache(db *tennis.DB) *nsCache {
	return &nsCache{db: db, open: map[string]*tennis.Namespace{}}
}

// get returns the cached handle, opening it on first use.
func (c *nsCache) get(ctx context.Context, name string) (*tennis.Namespace, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ns, ok := c.open[name]; ok {
		return ns, nil
	}
	ns, err := c.db.Namespace(ctx, name)
	if err != nil {
		return nil, err
	}
	c.open[name] = ns
	return ns, nil
}

// getOrCreate is get, creating the namespace with opts when it does not exist.
// Only "not found" triggers a create; any other open failure (missing key,
// model mismatch) is reported as itself.
func (c *nsCache) getOrCreate(ctx context.Context, name string, opts tennis.NamespaceOptions) (*tennis.Namespace, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ns, ok := c.open[name]; ok {
		return ns, nil
	}
	ns, err := c.db.Namespace(ctx, name)
	if errors.Is(err, tennis.ErrNamespaceNotFound) {
		ns, err = c.db.CreateNamespace(ctx, name, opts)
	}
	if err != nil {
		return nil, err
	}
	c.open[name] = ns
	return ns, nil
}

// newServeMux builds the API. Separate from cmdServe so tests can drive it
// through httptest without a real listener or signal handling.
func newServeMux(db *tennis.DB) http.Handler {
	mux := http.NewServeMux()
	cache := newNSCache(db)
	mux.HandleFunc("GET /v1/namespaces", func(w http.ResponseWriter, r *http.Request) {
		infos, err := db.ListNamespaces(r.Context())
		respond(w, infos, err)
	})
	mux.HandleFunc("POST /v1/namespaces/{ns}/write", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Documents []tennis.Document `json:"documents"`
			Model     string            `json:"model,omitempty"`
			OpenAI    string            `json:"openai_model,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			respondStatus(w, http.StatusBadRequest, err)
			return
		}
		ns, err := cache.getOrCreate(r.Context(), r.PathValue("ns"), tennis.NamespaceOptions{
			Model: body.Model, OpenAIModel: body.OpenAI,
		})
		if err != nil {
			respond(w, nil, err)
			return
		}
		res, err := ns.Write(r.Context(), body.Documents)
		respond(w, res, err)
	})
	mux.HandleFunc("POST /v1/namespaces/{ns}/query", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Text  string `json:"text"`
			TopK  int    `json:"top_k"`
			Mode  string `json:"mode"`
			Where string `json:"where"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			respondStatus(w, http.StatusBadRequest, err)
			return
		}
		ns, err := cache.get(r.Context(), r.PathValue("ns"))
		if err != nil {
			respond(w, nil, err)
			return
		}
		filter, err := parseWhere(body.Where)
		if err != nil {
			respondStatus(w, http.StatusBadRequest, err)
			return
		}
		results, err := ns.Query(r.Context(), tennis.Query{
			Text: body.Text, TopK: body.TopK, Mode: tennis.Mode(body.Mode), Filter: filter,
		})
		respond(w, map[string]any{"results": results}, err)
	})
	mux.HandleFunc("POST /v1/namespaces/{ns}/delete", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			IDs []string `json:"ids"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			respondStatus(w, http.StatusBadRequest, err)
			return
		}
		ns, err := cache.get(r.Context(), r.PathValue("ns"))
		if err != nil {
			respond(w, nil, err)
			return
		}
		n, err := ns.Delete(r.Context(), body.IDs)
		respond(w, map[string]int{"deleted": n}, err)
	})
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		respond(w, map[string]string{"status": "ok", "version": buildVersion(), "db": db.Path()}, nil)
	})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
		mux.ServeHTTP(w, r)
	})
}

// respond writes a JSON body, mapping known failures to useful statuses. The
// sentinel is matched with errors.Is; the substring checks catch the remaining
// errors that have no sentinel yet. Everything unrecognized is a 500 rather
// than being guessed at.
func respond(w http.ResponseWriter, v any, err error) {
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, tennis.ErrNamespaceNotFound), strings.Contains(err.Error(), "not found"):
			status = http.StatusNotFound
		case strings.Contains(err.Error(), "invalid"), strings.Contains(err.Error(), "is empty"),
			strings.Contains(err.Error(), "cannot parse"):
			status = http.StatusBadRequest
		}
		respondStatus(w, status, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func respondStatus(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

func isLoopback(host string) bool {
	if host == "localhost" || host == "" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
