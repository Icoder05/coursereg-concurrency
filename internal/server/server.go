package server

import (
	"fmt"
	"net/http"

	"go-student-service/internal/config"
	"go-student-service/internal/events"
	"go-student-service/internal/handlers"
	"go-student-service/internal/services"
	"go-student-service/internal/simulator"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func New(cfg *config.Config) *http.Server {
	hub := events.NewHub()             //fan-out for live dashboard events
	svc := services.NewService(hub)    //Creating Service Layer
	sim := simulator.New(svc)          //optional fake traffic for demos
	h := handlers.NewHandler(svc, sim) //Creating Handler Layer
	r := chi.NewRouter()               //Creating Router
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Logger, middleware.Recoverer)
	r.Use(cors(cfg.CORSOrigins))
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })
	h.RegisterRoutes(r) //Registering all routes(h is there for calling handlers)

	addr := fmt.Sprintf(":%s", cfg.HTTPPort) //Building Server Address(addr = ":8080")
	srv := &http.Server{Addr: addr, Handler: r}
	// SSE connections never finish on their own; close them so Shutdown can complete.
	srv.RegisterOnShutdown(func() { sim.Stop(); hub.Close() })
	return srv
}

// cors allows a separately hosted frontend (e.g. Vite dev server) to call the API.
func cors(origin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if origin != "" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
