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
	"syscall"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	_ "github.com/lib/pq"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Postgres struct {
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`
		Database string `yaml:"dbname"`
	} `yaml:"postgres"`

	App struct {
		Listen string `yaml:"listen"`
	} `yaml:"app"`
}

type Secret struct {
	PostgresUser     string `yaml:"pg_user"`
	PostgresPassword string `yaml:"pg_password"`
}

type User struct {
	UserName  string `json:"username"`
	FirstName string `json:"firstname"`
	LastName  string `json:"lastname"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
}

type UserWithPassword struct {
	User
	Password string `json:"password"`
}

type Result struct {
	Status string `json:"status"`
}

var dbconn *sql.DB

func root(c echo.Context) error {
	path := c.Request().URL
	return c.String(http.StatusOK, path.String())
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

func getUser(c echo.Context) error {
	idstr := c.Param("id")
	request := "SELECT username, firstname, lastname, email, phone FROM users WHERE username=$1"
	row := dbconn.QueryRow(request, idstr)
	err := row.Err()
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}

	var user User
	err = row.Scan(&user.UserName, &user.FirstName, &user.LastName, &user.Email, &user.Phone)
	if err != nil {
		var result Result
		result.Status = "No user"
		return c.JSON(http.StatusNotFound, result)
	}

	return c.JSON(http.StatusOK, user)
}

func createUser(c echo.Context) error {
	var user UserWithPassword
	err := c.Bind(&user)
	if err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}

	if user.UserName == "" {
		return c.String(http.StatusBadRequest, "invalid username")
	}

	if user.Password == "" {
		return c.String(http.StatusBadRequest, "empty password")
	}

	hashStr := user.Password + user.UserName
	crc := md5.Sum([]byte(hashStr))
	crcStr := hex.EncodeToString(crc[:])

	request := "INSERT INTO users (username, firstname, lastname, email, phone, password) VALUES ($1, $2, $3, $4, $5, $6)"

	_, err = dbconn.Exec(request, user.UserName, user.FirstName, user.LastName, user.Email, user.Phone, crcStr)
	if err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}

	var result Result
	result.Status = "User created"
	return c.JSON(http.StatusOK, result)
}

func deleteUser(c echo.Context) error {
	idstr := c.Param("id")
	request := "DELETE FROM users WHERE username=$1"
	_, err := dbconn.Exec(request, idstr)
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}

	var result Result
	result.Status = "User deleted"
	return c.JSON(http.StatusOK, result)
}

func updateUser(c echo.Context) error {
	idstr := c.Param("id")

	var user User
	err := c.Bind(&user)
	if err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}

	request := "UPDATE users SET firstname=$2, lastname=$3, email=$4, phone=$5 WHERE username=$1"
	_, err = dbconn.Exec(request, idstr, user.FirstName, user.LastName, user.Email, user.Phone)
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}

	var result Result
	result.Status = "User updated"
	return c.JSON(http.StatusOK, result)
}

func exitIfError(err error) {
	if err == nil {
		return
	}

	fmt.Println(err)
	os.Exit(1)
}

func readConfig(fpath string, cfg any) error {
	data, err := os.ReadFile(fpath)
	if err != nil {
		return err
	}

	return yaml.Unmarshal(data, cfg)
}

func main() {
	fmt.Println("App service")

	var err error
	var cfg Config
	err = readConfig("config/config.yml", &cfg)
	exitIfError(err)

	var secret Secret
	err = readConfig("secret/secret.yml", &secret)
	exitIfError(err)

	// open database
	psqlconn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.Postgres.Host, cfg.Postgres.Port, secret.PostgresUser, secret.PostgresPassword, cfg.Postgres.Database)

	db, err := sql.Open("postgres", psqlconn)
	exitIfError(err)

	dbconn = db

	// close database
	defer db.Close()

	// check db
	err = db.Ping()
	exitIfError(err)

	fmt.Println("Connected database")

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

		// api
		e.POST("/create", createUser)
		e.GET("/user/:id", getUser)
		e.DELETE("/user/:id", deleteUser)
		e.PUT("/user/:id", updateUser)

		e.GET("/", root)

		fmt.Println("Listen " + cfg.App.Listen)
		if err := e.Start(cfg.App.Listen); err != nil && !errors.Is(err, http.ErrServerClosed) {
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
}
