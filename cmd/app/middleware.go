package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/xbanchon/image-processing-service/internal/store"
)

func (app *application) AuthTokenMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			app.unauthorizedErrorResponse(w, r, fmt.Errorf("authorization header is missing"))
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			app.unauthorizedErrorResponse(w, r, fmt.Errorf("authorization header is malformed"))
			return
		}

		token := parts[1]
		jwtToken, err := app.authenticator.ValidateToken(token)
		if err != nil {
			app.unauthorizedErrorResponse(w, r, err)
			return
		}

		claims, _ := jwtToken.Claims.(jwt.MapClaims)

		userID, err := strconv.ParseInt(fmt.Sprintf("%.f", claims["sub"]), 10, 64)
		if err != nil {
			app.unauthorizedErrorResponse(w, r, err)
			return
		}

		ctx := r.Context()

		user, err := app.getUser(ctx, userID)
		if err != nil {
			app.unauthorizedErrorResponse(w, r, err)
			return
		}

		ctx = context.WithValue(ctx, userCtx, user)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (app *application) getUser(ctx context.Context, userID int64) (*store.User, error) {
	return app.store.Users.GetByID(ctx, userID)
}

func (app *application) getImage(ctx context.Context, imageID int64) (*store.Image, error) {
	if !app.config.redisCfg.enabled {
		return app.store.Images.GetByID(ctx, imageID)
	}

	image, err := app.cacheStorage.Images.Get(ctx, imageID)
	log.Printf("Cache Image: %+v", image)
	if err != nil {
		return nil, err
	}

	if image == nil {
		log.Println("image not found in cache, caching it...")
		image, err := app.store.Images.GetByID(ctx, imageID)
		if err != nil {
			return nil, err
		}

		if err := app.cacheStorage.Images.Set(ctx, image); err != nil {
			return nil, err
		}
		log.Println("image registry cached successfully!")
		return image, nil
	}

	return image, nil
}

func (app *application) RateLimiterMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if app.config.ratelimiter.Enabled {
			if allow, retryAfter := app.rateLimiter.Allow(r.RemoteAddr); !allow {
				app.rateLimitExceededResponse(w, r, retryAfter.String())
			}
		}

		next.ServeHTTP(w, r)
	})
}
