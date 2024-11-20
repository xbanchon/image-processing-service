package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/xbanchon/image-processing-service/internal/auth"
	"github.com/xbanchon/image-processing-service/internal/ratelimiter"
	"github.com/xbanchon/image-processing-service/internal/store"
	"github.com/xbanchon/image-processing-service/internal/store/cache"
	"github.com/xbanchon/image-processing-service/internal/store/supabase"
	"go.uber.org/zap"
)

type application struct {
	config        config
	authenticator auth.Authenticator
	logger        *zap.SugaredLogger
	store         store.Storage
	bucket        supabase.Storage
	cacheStorage  cache.Storage
	rateLimiter   ratelimiter.Limiter
}

type config struct {
	addr        string
	db          dbConfig
	auth        authConfig
	bucketCfg   bucketConfig
	redisCfg    redisConfig
	ratelimiter ratelimiter.Config
}

type dbConfig struct {
	addr         string
	maxOpenConns int
	maxIdleConns int
	maxIdleTime  string
}

type redisConfig struct {
	addr    string
	pw      string
	db      int
	enabled bool
}

type authConfig struct {
	secret string
	exp    time.Duration
	iss    string
}

type bucketConfig struct {
	bucket_id string
	api_key   string
}

func (app *application) mount() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	// r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Set a timeout value on the request context (ctx), that will signal
	// through ctx.Done() that the request has timed out and further
	// processing should be stopped.
	r.Use(middleware.Timeout(60 * time.Second))

	r.Post("/login", app.loginUserHandler)
	r.Post("/register", app.registerUserHandler)
	r.Route("/images", func(r chi.Router) {
		r.Use(app.AuthTokenMiddleware)
		r.Get("/", app.getImagesHandler)
		r.Post("/", app.uploadImageHandler)
		r.Get("/{imageID}", app.getImageHandler)
		r.Post("/{imageID}/transform", app.transformImageHandler)
		r.Post("/metadata", app.testMetadataEndpoint)
	})

	//test routes
	r.Post("/transform", app.testBasicTransformation)
	r.Get("/url", app.testImageURL)
	r.Post("/request", app.testTransformationReq)

	return r
}

func (app *application) run(mux http.Handler) error {

	srv := http.Server{
		Addr:         app.config.addr,
		Handler:      mux,
		WriteTimeout: time.Second * 30,
		ReadTimeout:  time.Second * 10,
		IdleTimeout:  time.Minute,
	}

	shutdown := make(chan error)

	go func() {
		quit := make(chan os.Signal, 1)

		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		s := <-quit

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		app.logger.Infow("signal caught", "signal", s.String())

		shutdown <- srv.Shutdown(ctx)
	}()

	app.logger.Infow("server started", "addr", app.config.addr)

	err := srv.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	err = <-shutdown
	if err != nil {
		return err
	}

	app.logger.Infow("server stopped", "addr", app.config.addr)

	return nil
}
