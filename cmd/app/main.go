package main

import (
	"expvar"
	"runtime"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/xbanchon/image-processing-service/internal/auth"
	"github.com/xbanchon/image-processing-service/internal/db"
	"github.com/xbanchon/image-processing-service/internal/env"
	"github.com/xbanchon/image-processing-service/internal/ratelimiter"
	"github.com/xbanchon/image-processing-service/internal/store"
	"github.com/xbanchon/image-processing-service/internal/store/cache"
	"github.com/xbanchon/image-processing-service/internal/store/supabase"
	"go.uber.org/zap"
)

const version = "1.0.0"

func main() {
	cfg := config{
		addr: env.GetString("ADDR", ":8080"),
		db: dbConfig{
			addr:         env.GetString("DB_ADDR", "postgres://postgres:maneking@localhost/postgres?sslmode=disable"),
			maxOpenConns: env.GetInt("DB_MAX_OPEN_CONNS", 30),
			maxIdleConns: env.GetInt("DB_MAX_IDLE_CONNS", 30),
			maxIdleTime:  env.GetString("DB_MAX_IDLE_TIME", "15m"),
		},
		auth: authConfig{
			secret: env.GetString("AUTH_SECRET", "ips"),
			exp:    60 * time.Minute,
			iss:    "felis somnolento",
		},
		bucketCfg: bucketConfig{
			api_key:   env.GetString("SUPABASE_PROJECT_API_KEY", ""),
			bucket_id: env.GetString("SUPABASE_BUCKET_ID", ""),
		},
		redisCfg: redisConfig{
			addr:    env.GetString("REDIS_ADDR", "localhost:6379"),
			pw:      env.GetString("REDIS_PW", ""),
			db:      env.GetInt("REDIS_DB", 0),
			enabled: env.GetBool("REDIS_ENABLED", false),
		},
		ratelimiter: ratelimiter.Config{
			RequestPerTimeFrame: env.GetInt("RL_REQS_COUNT", 15),
			TimeFrame:           5 * time.Second,
			Enabled:             env.GetBool("RL_ENABLED", true),
		},
	}

	//Authenticator (JWT)
	jwtAuthenticator := auth.NewJWTAuth(
		cfg.auth.secret,
		cfg.auth.iss,
		cfg.auth.iss,
	)

	//Logger (Zap)
	logger := zap.Must(zap.NewProduction()).Sugar()

	defer logger.Sync() //flushes buffer, if any

	//Cache
	var rdb *redis.Client
	if cfg.redisCfg.enabled {
		rdb = cache.NewRedisClient(cfg.redisCfg.addr, cfg.redisCfg.pw, cfg.redisCfg.db)
		logger.Info("cache connection established!")

		defer rdb.Close()
	}

	//Database (Postgres)
	db, err := db.New(
		cfg.db.addr,
		cfg.db.maxOpenConns,
		cfg.db.maxIdleConns,
		cfg.db.maxIdleTime,
	)
	if err != nil {
		logger.Fatal(err)
	}

	defer db.Close()
	logger.Info("database connection pool established!")

	//DB Storage
	store := store.NewStorage(db)
	//Cache Storage
	cacheStore := cache.NewRedisStorage(rdb)

	//Supabase Bucket Storage
	sc := supabase.NewSupabaseClient(cfg.bucketCfg.bucket_id, cfg.bucketCfg.api_key)
	bucket := supabase.NewSupabaseStorage(sc)

	//Rate Limiter
	rateLimiter := ratelimiter.NewFixedWindowLimiter(
		cfg.ratelimiter.RequestPerTimeFrame,
		cfg.ratelimiter.TimeFrame,
	)

	app := &application{
		config:        cfg,
		authenticator: jwtAuthenticator,
		logger:        logger,
		store:         store,
		bucket:        bucket,
		cacheStorage:  cacheStore,
		rateLimiter:   rateLimiter,
	}

	// Metrics
	expvar.NewString("version").Set(version)
	expvar.Publish("database", expvar.Func(func() any {
		return db.Stats()
	}))
	expvar.Publish("goroutines", expvar.Func(func() any {
		return runtime.NumGoroutine()
	}))

	mux := app.mount()

	logger.Fatal(app.run(mux))
}
