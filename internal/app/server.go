package app

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"

	"phrasemate/internal/ai"
	"phrasemate/internal/config"
	"phrasemate/internal/enricher"
	"phrasemate/internal/handler"
	"phrasemate/internal/store"
	"phrasemate/web"
)

// Runtime holds shared services for the desktop / web host.
type Runtime struct {
	Cfg      config.Config
	Store    *store.Store
	AI       *ai.Client
	Enricher *enricher.Worker
	Server   *http.Server
	URL      string
	cancel   context.CancelFunc
}

// Start boots SQLite, AI client, enricher and a local HTTP server.
// If listenAddr is empty, it binds 127.0.0.1 on a free port.
func Start(cfg config.Config, listenAddr string) (*Runtime, error) {
	st, err := store.Open(cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	client := ai.New(cfg.APIKey, cfg.BaseURL, cfg.Model)
	en := enricher.New(st, client, nil)
	api := handler.New(cfg, st, client, en)

	mux := http.NewServeMux()
	api.Register(mux)
	mux.Handle("/", staticHandler())

	if strings.TrimSpace(listenAddr) == "" {
		listenAddr = "127.0.0.1:0"
	}

	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		_ = st.Close()
		return nil, fmt.Errorf("监听失败 (%s): %w", listenAddr, err)
	}

	url := "http://" + ln.Addr().String()
	srv := &http.Server{Handler: withCORS(mux)}

	ctx, cancel := context.WithCancel(context.Background())
	go en.Run(ctx)
	en.Kick()

	go func() {
		_ = srv.Serve(ln)
	}()

	return &Runtime{
		Cfg:      cfg,
		Store:    st,
		AI:       client,
		Enricher: en,
		Server:   srv,
		URL:      url,
		cancel:   cancel,
	}, nil
}

// Close shuts down the HTTP server and database.
func (r *Runtime) Close() {
	if r == nil {
		return
	}
	if r.cancel != nil {
		r.cancel()
	}
	if r.Server != nil {
		_ = r.Server.Close()
	}
	if r.Store != nil {
		_ = r.Store.Close()
	}
}

func staticHandler() http.Handler {
	staticRoot := http.FS(web.FS)
	fileServer := http.FileServer(staticRoot)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if f, err := web.FS.Open(path); err == nil {
			_ = f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}
		http.ServeFileFS(w, r, web.FS, "index.html")
	})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
