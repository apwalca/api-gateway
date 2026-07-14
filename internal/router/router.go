package router

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sony/gobreaker"

	"myproject/api-gateway/internal/auth"
	"myproject/api-gateway/internal/cache"
	"myproject/api-gateway/internal/config"
	"myproject/api-gateway/internal/logger"
	"myproject/api-gateway/internal/metrics"
	mid "myproject/api-gateway/internal/middleware"
	"myproject/api-gateway/internal/proxy"
	"myproject/api-gateway/internal/ratelimit"
)

type Router struct {
	chi.Router
	config   *config.Config
	logger   *logger.Logger
	auth     *auth.JWTValidator
	limiter  *ratelimit.TokenBucket
	cache    *cache.RedisCache
	proxy    *proxy.ReverseProxy
	metrics  *metrics.Metrics
	routeMap map[string]*Route
}

type Route struct {
	Path      string
	Target    string
	Methods   map[string]bool
	CacheTTL  int
	RateLimit int
	Retries   int
	Timeout   int
}

func New(cfg *config.Config, log *logger.Logger) (*Router, error) {
	r := &Router{
		Router:   chi.NewRouter(),
		config:   cfg,
		logger:   log,
		routeMap: make(map[string]*Route),
	}

	r.Use(middleware.Recoverer)
	r.Use(mid.RequestID)
	r.Use(mid.ClientIP)

	if cfg.Auth.Enabled {
		var err error
		r.auth, err = auth.NewJWTValidator(
			cfg.Auth.JWTPublicKeyPath,
			cfg.Auth.Issuer,
			cfg.Auth.Audience,
			cfg.Auth.Algorithm,
		)
		if err != nil {
			return nil, fmt.Errorf("jwt не инициализирован: %w", err)
		}
	}

	if cfg.RateLimit.Enabled || cfg.Cache.Enabled {
		redisClient, err := cache.NewRedisClient(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB, cfg.Redis.Timeout)
		if err != nil {
			return nil, fmt.Errorf("редис не стартанул: %w", err)
		}

		if cfg.RateLimit.Enabled {
			r.limiter = ratelimit.NewTokenBucket(redisClient, log, cfg.RateLimit.FailOpen)
		}
		if cfg.Cache.Enabled {
			r.cache = cache.NewRedisCache(redisClient, log, cfg.Cache.MaxSize)
		}
	}

	var cbSettings gobreaker.Settings
	if cfg.CircuitBreaker.Enabled {
		cbSettings = gobreaker.Settings{
			Name:        "backend-proxy",
			MaxRequests: uint32(cfg.CircuitBreaker.MaxRequests),
			Interval:    time.Duration(cfg.CircuitBreaker.IntervalSeconds) * time.Second,
			Timeout:     time.Duration(cfg.CircuitBreaker.TimeoutSeconds) * time.Second,
			ReadyToTrip: func(counts gobreaker.Counts) bool {
				return counts.ConsecutiveFailures > uint32(cfg.CircuitBreaker.ConsecutiveFailures)
			},
		}
	}
	r.proxy = proxy.New(log, cfg.CircuitBreaker.Enabled, cbSettings)

	r.metrics = metrics.New()

	r.Get("/health", r.healthHandler)
	r.Get("/ready", r.readyHandler)
	r.Handle("/metrics", promhttp.Handler())

	for _, routeCfg := range cfg.Routes {
		methods := make(map[string]bool)
		for _, m := range routeCfg.Methods {
			methods[m] = true
		}

		r.routeMap[routeCfg.Path] = &Route{
			Path:      routeCfg.Path,
			Target:    routeCfg.Target,
			Methods:   methods,
			CacheTTL:  routeCfg.CacheTTL,
			RateLimit: routeCfg.RateLimit,
			Retries:   routeCfg.Retries,
			Timeout:   routeCfg.Timeout,
		}

		r.Route(routeCfg.Path, func(r chi.Router) {
			r.Use(r.authMiddleware)
			r.Use(r.rateLimitMiddleware(routeCfg.Path))
			r.Handle("/*", r.proxyHandler(routeCfg.Path))
		})
	}

	return r, nil
}

func (r *Router) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if !r.config.Auth.Enabled {
			next.ServeHTTP(w, req)
			return
		}

		claims, err := r.auth.Validate(req)
		if err != nil {
			r.logger.Warn("ошибка авторизации", "error", err, "path", req.URL.Path)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if sub, ok := (*claims)["sub"].(string); ok {
			req.Header.Set("X-User-ID", sub)
		}
		if roles, ok := (*claims)["roles"].([]interface{}); ok {
			rolesStr := make([]string, len(roles))
			for i, r := range roles {
				rolesStr[i] = fmt.Sprintf("%v", r)
			}
			req.Header.Set("X-User-Roles", strings.Join(rolesStr, ","))
		}

		next.ServeHTTP(w, req)
	})
}

func (r *Router) rateLimitMiddleware(routePath string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if !r.config.RateLimit.Enabled {
				next.ServeHTTP(w, req)
				return
			}

			route, ok := r.routeMap[routePath]
			if !ok || route.RateLimit <= 0 {
				next.ServeHTTP(w, req)
				return
			}
