package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"

	_ "github.com/bootjp/vrc_panoprama_picture_manage/statik"
	"github.com/garyburd/redigo/redis"
	"github.com/google/uuid"
	"github.com/labstack/echo"
	"github.com/rakyll/statik/fs"
)

const envTempToken = "TEMPORARY_TOKEN"

func main() {
	temporaryToken := uuid.Must(uuid.NewRandom())
	err := os.Setenv(envTempToken, temporaryToken.String())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("current temporary token %s \n", temporaryToken)
	e := echo.New()

	// Routes
	statikFS, err := fs.New()
	if err != nil {
		log.Fatal(err)
	}
	e.GET("/_/", echo.WrapHandler(http.StripPrefix("/_/", http.FileServer(statikFS))))
	e.GET("/r/:key", panoramaHandler)
	e.POST("/api/update", apiHandler)
	e.Logger.Fatal(e.Start(":1323"))
}

func fetchContents(key string) (string string, err error) {
	r := redisConnection()
	defer func() {
		if err := r.Close(); err != nil {
			log.Print(err)
		}

	}()
	return redis.String(r.Do("GET", key))
}

func panoramaHandler(c echo.Context) error {
	key := c.Param("key")
	c.Response().Header().Set("Cache-Control", "no-store")

	url, err := fetchContents(key)
	if err != nil {
		// redirect resource not found.
		log.Println(err)
		return c.NoContent(204)
	}

	return c.Redirect(302, url)
}

type (
	UpdateRequest struct {
		Token string
		Key   string
		URL   string
	}
)

func specialResponseHost(ip string) bool {
	r := redisConnection()
	defer func() {
		if err := r.Close(); err != nil {
			log.Print(err)
		}
	}()
	hosts, err := redis.Strings(r.Do("SMEMBERS", "hosts"))
	if err != nil {
		return false
	}
	for _, v := range hosts {
		re, err := net.LookupHost(v)
		if err != nil {
			return false
		}
		for resolveIP := range re {
			if ip == re[resolveIP] {
				return true
			}
		}
	}

	return false
}

func apiHandler(c echo.Context) error {
	u := &UpdateRequest{}
	if err := c.Bind(u); err != nil {
		return err
	}
	if !ValidToken(u.Token) {
		c.Response().Status = 400
		return nil
	}
	r := redisConnection()
	_, err := r.Do("SET", u.Key, u.URL)
	if err != nil {
		c.Response().Status = 400
		return nil
	}
	err = r.Close()
	if err != nil {
		return err
	}

	return c.String(200, `{"status":"OK"}`)
}

func ValidToken(token string) bool {
	tt := os.Getenv(envTempToken)
	if tt != "" && tt == token {
		return true
	}
	r := redisConnection()
	defer func() {
		if err := r.Close(); err != nil {
			log.Print(err)
		}
	}()

	tokens, err := redis.Strings(r.Do("SMEMBERS", "tokens"))
	if err != nil {
		return false
	}

	for _, v := range tokens {
		if v == token {
			return true
		}
	}

	return false
}

func redisConnection() redis.Conn {
	host := os.Getenv("REDIS_HOST")
	if host == "" {
		host = "0.0.0.0"
	}

	c, err := redis.Dial("tcp", host+":6379")
	if err != nil {
		log.Println(err)
	}
	return c
}
