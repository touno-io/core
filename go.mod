module github.com/touno-io/core

go 1.18

require (
	github.com/getsentry/sentry-go v0.13.0
	github.com/go-resty/resty/v2 v2.7.0
	github.com/gofiber/fiber/v2 v2.34.1
	github.com/gofiber/template v1.6.28
	github.com/mileusna/useragent v1.1.0
	github.com/pressly/goose/v3 v3.6.1
	github.com/tmilewski/goenv v1.0.0
	github.com/touno-io/goasa v1.0.1
)

require (
	github.com/gofiber/storage/postgres v0.0.0-20220523092334-6d96fb56afb5 // indirect
	github.com/golang-jwt/jwt/v4 v4.4.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	golang.org/x/net v0.0.0-20220624214902-1bab6f366d9e // indirect
	golang.org/x/text v0.3.7 // indirect
	gopkg.in/alexcesaro/quotedprintable.v3 v3.0.0-20150716171945-2caba252f4dc // indirect
)

require (
	github.com/andybalholm/brotli v1.0.4 // indirect
	github.com/json-iterator/go v1.1.12
	github.com/klauspost/compress v1.15.7 // indirect
	github.com/lib/pq v1.10.6
	github.com/pkg/errors v0.9.1 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasthttp v1.38.0 // indirect
	github.com/valyala/tcplisten v1.0.0 // indirect
	golang.org/x/sys v0.0.0-20220627191245-f75cf1eec38b // indirect
	gopkg.in/mail.v2 v2.3.1
)

replace github.com/touno-io/core/db => ./db

replace github.com/touno-io/core/api => ./api
