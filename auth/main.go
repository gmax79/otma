package main

import (
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
	_ "github.com/lib/pq"
	"gopkg.in/yaml.v3"
)

const appCookieName = "otma_auth_cookie"

type Config struct {
	Postgres struct {
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`
		Database string `yaml:"dbname"`
	} `yaml:"postgres"`

	Auth struct {
		Listen string `yaml:"listen"`
	} `yaml:"auth"`
}

type Secret struct {
	PostgresUser     string `yaml:"pg_user"`
	PostgresPassword string `yaml:"pg_password"`
}

type User struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type Result struct {
	Status string `json:"status"`
}

type Cookie struct {
	Username string
	UUID     string
}

func readCookie(v string) *Cookie {
	parts := strings.Split(v, ":")
	if len(parts) != 2 {
		return nil
	}

	var c Cookie
	c.UUID = parts[0]
	c.Username = parts[1]
	if c.UUID == "" || c.Username == "" {
		return nil
	}

	return &c
}

var dbconn *sql.DB
var mx sync.Mutex
var cookies map[string]string

func root(c echo.Context) error {
	path := c.Request().URL
	return c.String(http.StatusNotFound, path.String())
}

func health(c echo.Context) error {
	var result Result
	result.Status = "OK"
	return c.JSON(http.StatusOK, result)
}

func readness(c echo.Context) error {
	status := "true"
	return c.String(http.StatusOK, status)
}

func liveness(c echo.Context) error {
	status := "true"
	return c.String(http.StatusOK, status)
}

func getHeader(c echo.Context, name string) string {
	h := c.Request().Header
	path, ok := h[name]
	if !ok || len(path) != 1 {
		return ""
	}

	return path[0]
}

func auth(c echo.Context) error {
	method := getHeader(c, "X-Original-Method")
	if method == "" {
		return c.String(http.StatusUnauthorized, "original method")
	}

	method = strings.ToUpper(method)
	if method == "POST" {
		log.Infof("method: %s ", method)
		return c.NoContent(http.StatusOK) // user create
	}

	// PUT - update
	// GET - read
	// DELETE - delete
	// need authorization

	path := getHeader(c, "X-Original-Uri")
	if path == "" {
		return c.String(http.StatusUnauthorized, "original url")
	}

	parts := strings.Split(path, "/")
	if len(parts) != 3 {
		return c.String(http.StatusUnauthorized, "original url")
	}

	userName := parts[2]
	cookie, err := c.Cookie(appCookieName)
	if err != nil {
		return c.String(http.StatusUnauthorized, err.Error())
	}

	log.Infof("user request: %s, method: %s", userName, method)

	cv := readCookie(cookie.Value)
	if cv != nil {
		success := false

		mx.Lock()
		user, ok := cookies[cv.UUID]
		if ok && user == cv.Username && user == userName { // only himself profile can be accessed
			success = true
		}
		mx.Unlock()

		if success {
			if method == "DELETE" {
				mx.Lock()
				delete(cookies, cv.UUID)
				mx.Unlock()
			}

			return c.NoContent(http.StatusOK)
		}
	}

	return c.NoContent(http.StatusUnauthorized)
}

func loginUser(c echo.Context) error {
	var user User
	err := c.Bind(&user)
	if err != nil {
		return c.String(http.StatusBadRequest, "bad user data")
	}

	request := "SELECT password FROM users WHERE username=$1"
	row := dbconn.QueryRow(request, user.Username)
	err = row.Err()
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}

	var dbHash string
	err = row.Scan(&dbHash)
	if err != nil {
		var result Result
		result.Status = "No user"
		return c.JSON(http.StatusNotFound, result)
	}

	if dbHash == "" {
		return c.String(http.StatusInternalServerError, "get db password")
	}

	hashStr := user.Password + user.Username
	crc := md5.Sum([]byte(hashStr))
	crcStr := hex.EncodeToString(crc[:])

	if dbHash != crcStr {
		return c.String(http.StatusForbidden, "incorrect login or password")
	}

	log.Infof("used logined: %s\n", user.Username)

	// generate cookie
	id := uuid.New()

	mx.Lock()
	cookies[id.String()] = user.Username
	mx.Unlock()

	var cookie http.Cookie
	cookie.Name = appCookieName
	cookie.Value = id.String() + ":" + user.Username
	c.SetCookie(&cookie)

	log.Infof("user: %s, cookie: %s", user.Username, id.String())
	return c.String(http.StatusOK, "logined")
}

func logoutUser(c echo.Context) error {
	cookie, err := c.Cookie(appCookieName)
	if err != nil {
		return c.String(http.StatusNotAcceptable, "not logined")
	}

	cv := readCookie(cookie.Value)
	if cv == nil {
		return c.String(http.StatusNotAcceptable, "not logined")
	}

	success := false
	mx.Lock()
	if v, ok := cookies[cv.UUID]; ok && v == cv.Username {
		success = true
	}
	if success {
		delete(cookies, cv.UUID)
	}
	mx.Unlock()

	if !success {
		return c.String(http.StatusBadRequest, "logout failed")
	}

	return c.String(http.StatusOK, "logout successeded")
}

func readConfig(fpath string, cfg any) error {
	data, err := os.ReadFile(fpath)
	if err != nil {
		return err
	}

	return yaml.Unmarshal(data, cfg)
}

func main() {
	cookies = make(map[string]string)
	fmt.Println("Auth service")
	if err := runService(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func runService() error {
	var err error
	var cfg Config
	err = readConfig("config/config.yml", &cfg)
	if err != nil {
		return err
	}

	var secret Secret
	err = readConfig("secret/secret.yml", &secret)
	if err != nil {
		return err
	}

	// open database
	psqlconn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.Postgres.Host, cfg.Postgres.Port, secret.PostgresUser, secret.PostgresPassword, cfg.Postgres.Database)

	db, err := sql.Open("postgres", psqlconn)
	if err != nil {
		return fmt.Errorf("failed postgres connection: %w", err)
	}

	dbconn = db

	// close database
	defer db.Close()

	// check db
	err = db.Ping()
	if err != nil {
		return fmt.Errorf("failed postgres connection: %w", err)
	}

	fmt.Println("Connected postgres database")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	go func() {
		e := echo.New()
		e.HidePort = true
		e.HideBanner = true
		e.Use(middleware.Recover())
		e.Use(middleware.Logger())
		e.Pre(middleware.RemoveTrailingSlash())
		e.GET("/health", health)
		e.GET("/readness", readness)
		e.GET("/liveness", liveness)

		// auth
		e.GET("/auth", auth)

		// login, logout
		e.POST("/login", loginUser)
		e.POST("/logout", logoutUser)

		// root
		e.GET("/", root)

		fmt.Println("Listen " + cfg.Auth.Listen)
		if err := e.Start(cfg.Auth.Listen); err != nil && !errors.Is(err, http.ErrServerClosed) {
			err = fmt.Errorf("start server: %w", err)
			fmt.Println(err)
			close(sigChan)
			return
		}
	}()

	_, ok := <-sigChan
	if ok {
		close(sigChan)
	}

	fmt.Println("Service stopped")
	return nil
}
