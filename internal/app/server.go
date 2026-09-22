package app

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"

	"phrasemate/internal/ai"
	"phrasemate/internal/config"
	"phrasemate/internal/dict"
	"phrasemate/internal/enricher"
	"phrasemate/internal/gitdata"
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
	Git      *gitdata.Sync
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

	if apiKey, baseURL, model, err := st.LoadAppSettings(); err == nil {
		cfg.MergePersisted(apiKey, baseURL, model)
		// One-time: persist env credentials so UI settings survive without .env.
		if strings.TrimSpace(apiKey) == "" && strings.TrimSpace(cfg.APIKey) != "" {
			_ = st.SetSetting(store.SettingAPIKey, cfg.APIKey)
		}
		if strings.TrimSpace(baseURL) == "" && strings.TrimSpace(cfg.BaseURL) != "" {
			_ = st.SetSetting(store.SettingBaseURL, cfg.BaseURL)
		}
		if strings.TrimSpace(model) == "" && strings.TrimSpace(cfg.Model) != "" {
			_ = st.SetSetting(store.SettingModel, cfg.Model)
		}
	}

	var gs *gitdata.Sync
	if syncer, err := gitdata.Discover(); err != nil {
		log.Printf("生词本保存在本地数据库: %v", err)
	} else {
		gs = syncer
		log.Printf("生词本将同步到 Git 分支 %s", gs.Branch())
		gs.SetStatusHandler(func(branch, errMsg string) {
			st.SetSyncState(branch, errMsg)
		})
		if err := gs.Bootstrap(st); err != nil {
			log.Printf("初始化 %s 分支失败，暂用本地生词本: %v", gs.Branch(), err)
			st.SetSyncState(gs.Branch(), err.Error())
		}
		st.SetAfterWordChange(func() {
			if err := gs.Save(st); err != nil {
				log.Printf("写入 %s 分支失败: %v", gs.Branch(), err)
				st.SetSyncState(gs.Branch(), err.Error())
			}
		})
	}

	client := ai.New(cfg.APIKey, cfg.BaseURL, cfg.Model)
	dictClient := dict.New(cfg.DictURL)
	en := enricher.New(st, client, dictClient, cfg.DictFirst, nil)
	api := handler.New(cfg, st, client, en)

	mux := http.NewServeMux()
	api.Register(mux)
	mux.Handle("/", staticHandler())

	if strings.TrimSpace(listenAddr) == "" {
		listenAddr = "127.0.0.1:0"
	}

	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		if gs != nil {
			gs.Close()
		}
		_ = st.Close()
		return nil, fmt.Errorf("监听失败 (%s): %w", listenAddr, err)
	}

	bound := ln.Addr().String()
	host, port, splitErr := net.SplitHostPort(bound)
	if splitErr == nil && (host == "" || host == "0.0.0.0" || host == "::") {
		bound = "127.0.0.1:" + port
	}
	url := "http://" + bound
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
		Git:      gs,
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
	if r.Git != nil {
		r.Git.Close()
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
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
