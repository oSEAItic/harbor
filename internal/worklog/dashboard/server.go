package dashboard

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"time"

	"github.com/oseaitic/harbor/internal/worklog"
)

//go:embed assets/*
var assets embed.FS

type FeatureView struct {
	worklog.FeatureStats
	Events   []worklog.Event          `json:"events"`
	Sessions []worklog.SessionBinding `json:"sessions"`
	Scope    []worklog.ScopeItem      `json:"scope"`
	EndAt    time.Time                `json:"end_at"`
}

type Data struct {
	GeneratedAt time.Time      `json:"generated_at"`
	Summary     worklog.Report `json:"summary"`
	Features    []FeatureView  `json:"features"`
}

func Handler(store *worklog.Store) (http.Handler, error) {
	static, err := fs.Sub(assets, "assets")
	if err != nil {
		return nil, fmt.Errorf("opening dashboard assets: %w", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/dashboard", func(w http.ResponseWriter, r *http.Request) {
		data, err := loadData(r.Context(), store, time.Now().UTC())
		if err != nil {
			http.Error(w, "failed to read worklog", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if err := json.NewEncoder(w).Encode(data); err != nil {
			return
		}
	})
	mux.Handle("GET /", http.FileServer(http.FS(static)))
	return securityHeaders(mux), nil
}

func Serve(ctx context.Context, store *worklog.Store, address string, ready func(string)) error {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", address, err)
	}
	defer listener.Close()
	handler, err := Handler(store)
	if err != nil {
		return err
	}
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	if ready != nil {
		ready("http://" + listener.Addr().String())
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	err = server.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func loadData(ctx context.Context, store *worklog.Store, now time.Time) (Data, error) {
	report, err := store.BuildReport(ctx, time.Time{}, now)
	if err != nil {
		return Data{}, err
	}
	data := Data{GeneratedAt: now, Summary: report}
	for _, stats := range report.Features {
		detail, err := store.Detail(ctx, stats.Feature.ID)
		if err != nil {
			return Data{}, err
		}
		endAt := now
		if stats.Feature.Status == worklog.StatusShipped {
			for i := len(detail.Events) - 1; i >= 0; i-- {
				if detail.Events[i].Kind == worklog.EventShipped {
					endAt = detail.Events[i].CreatedAt
					break
				}
			}
		}
		data.Features = append(data.Features, FeatureView{
			FeatureStats: stats,
			Events:       detail.Events,
			Sessions:     detail.Sessions,
			Scope:        detail.Scope,
			EndAt:        endAt,
		})
	}
	return data, nil
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}
