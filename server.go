package main

import (
	"net/http"
	"github.com/labstack/echo"
	"os"
	"log"
	"github.com/dgrijalva/jwt-go"
	"github.com/joho/godotenv"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gocraft/dbr"
	"time"
	"github.com/gernest/mention"
	"strings"
	"github.com/labstack/echo/middleware"
	"strconv"
	"unicode/utf8"
)

type (
	spik struct {
		Id int64 `db:"id" json:"id"`
		Content string `db:"content" json:"content"`
		CreatedAt dbr.NullTime `db:"created_at" json:"created_at"`
	}

	hashtag struct {
		Id int64 `db:"id" json:"id"`
		Name string `db:"name" json:"name"`
		CreatedAt dbr.NullTime `db:"created_at" json:"created_at"`
	}

	spikHashtag struct {
		SpikId     int64 `db:"spik_id" json:"spik_id"`
		HashtagId  int64 `db:"hashtag_id" json:"hashtag_id"`
		CreatedAt dbr.NullTime `db:"created_at" json:"created_at"`
	}

	userFollowHashtag struct{
		UserId string `db:"user_id" json:"user_id"`
		HashtagId  int64 `db:"hashtag_id" json:"hashtag_id"`
		CreatedAt dbr.NullTime `db:"created_at" json:"created_at"`
	}

	responseSpik struct {
		Spiks []spik `json:"spiks"`
	}

	responseHashtag struct {
		Hashtags []hashtag `json:"hashtags"`
	}
)

var (
	spikTable = "spiks"
	hashtagTable = "hashtags"
	spikHashtagTable = "spik_hashtags"
	userFollowingHashtagTable = "user_follow_hashtags"
	sess *dbr.Session
)

func main() {

	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	conn, _ := dbr.Open("mysql", os.Getenv("DBCON"), nil)
	sess = conn.NewSession(nil)

	StartServer()
}

func StartServer() {

	e := echo.New()

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Hello")
	})

	e.GET("/ping", func(c echo.Context) error {
		return c.String(http.StatusOK, "ping")
	})

	r := e.Group("/api")

	secret := []byte(os.Getenv("AUTH0_CLIENT_SECRET"))

	if len(secret) == 0 {
		log.Fatal("AUTH0_CLIENT_SECRET is not set")
	}

	r.Use(middleware.JWT(secret))
	r.GET("/ping", restrictedPing)

	// spiks related -> tested
	r.POST("/spiks", addSpiks)

	// see all available hashtags -> tested
	r.GET("/hashtags", listTags)

	// see user follow hashtags
	r.GET("/following", followingHashtags)

	// follow to hashtags
	r.POST("/following", followHashtag)

	// timeline related -> tested
	r.GET("/timeline", listTimeline)

	// unfollow to hashtags
	r.POST("/unfollow", unfollowHashtag)

	e.Logger.Fatal(e.Start(":"+os.Getenv("SERVER_PORT")))
}


func restrictedPing(c echo.Context) error {

	return c.String(http.StatusOK, "ping !")
}

func addSpiks(c echo.Context) error {

	var s spik
	s.Content = c.FormValue("content")

	// limit number of strings
	if utf8.RuneCountInString(s.Content) > 140 {
		return c.NoContent(http.StatusNotAcceptable)
	}

	s.CreatedAt = dbr.NewNullTime(time.Now().UTC())
	spikRes, _ := sess.InsertInto(spikTable).Columns("content","created_at").Record(s).Exec()
	s.Id, _ = spikRes.LastInsertId()

	tags := mention.GetTags('#', strings.NewReader(s.Content))
	for _, v := range tags {
		var tag hashtag
		sess.Select("id").From(hashtagTable).Where("name = ?", v).Load(&tag)

		if tag.Id == 0 {
			tag.Name = v
			tag.CreatedAt = dbr.NewNullTime(time.Now().UTC())
			tagRes, _ := sess.InsertInto(hashtagTable).Columns("name", "created_at").Record(tag).Exec()
			tag.Id, _ = tagRes.LastInsertId()
		}

		var spiktag spikHashtag
		spiktag.SpikId = s.Id
		spiktag.HashtagId = tag.Id
		spiktag.CreatedAt =dbr.NewNullTime(time.Now().UTC())
		sess.InsertInto(spikHashtagTable).Columns("spik_id", "hashtag_id", "created_at").Record(&spiktag).Exec()
	}

	return c.JSON(http.StatusOK, s)
}

func listTimeline(c echo.Context) error {
	user := c.Get("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)

	var since_id int64 = 0
	querySinceId := c.QueryParam("since_id")
	if len(querySinceId) > 0 {
		sinceId, errorConv := strconv.ParseInt(querySinceId, 10, 64)
		if errorConv != nil {
			return c.NoContent(http.StatusBadRequest)
		}
		since_id = sinceId
	}


	sub := claims["sub"].(string)

	var hashtagsIds []int64
	sess.Select("hashtag_id").From(userFollowingHashtagTable).Where("user_id = ?", sub).LoadValues(&hashtagsIds)

	var spikIds []int64
	sess.Select("spik_id").From(spikHashtagTable).Where("hashtag_id IN ?", hashtagsIds).LoadValues(&spikIds)

	var spikResponses []spik
	sess.Select( "*").From(spikTable).Where("id in ? and id > ?", spikIds, since_id).Load(&spikResponses)


	var response responseSpik
	response.Spiks = spikResponses
	return c.JSON(http.StatusOK, response)
}

func followHashtag(c echo.Context) error {

	user := c.Get("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)

	sub := claims["sub"].(string)
	hashtagId := c.FormValue("hashtag_id")

	var userFollowingHashtag userFollowHashtag
	sess.Select("hashtag_id").From(userFollowingHashtagTable).Where("user_id = ? AND hashtag_id = ?", sub, hashtagId).LoadStruct(&userFollowingHashtag)

	if userFollowingHashtag.HashtagId != 0 {
		return c.NoContent(http.StatusBadRequest)
	}

	userFollowingHashtag.HashtagId, _ = strconv.ParseInt(hashtagId, 10, 64)
	userFollowingHashtag.UserId = sub
	userFollowingHashtag.CreatedAt = dbr.NewNullTime(time.Now().UTC())

	sess.InsertInto(userFollowingHashtagTable).Columns("user_id", "hashtag_id", "created_at").Record(&userFollowingHashtag).Exec()

	return c.NoContent(http.StatusOK)
}

func unfollowHashtag(c echo.Context) error {

	user := c.Get("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)

	sub := claims["sub"].(string)
	hashtagId := c.FormValue("hashtag_id")

	var userFollowingHashtag userFollowHashtag
	sess.Select("hashtag_id").From(userFollowingHashtagTable).Where("user_id = ? AND hashtag_id = ?", sub, hashtagId).LoadStruct(&userFollowingHashtag)

	if userFollowingHashtag.HashtagId != 0 {
		return c.NoContent(http.StatusBadRequest)
	}

	tagId, _ := strconv.ParseInt(hashtagId, 10, 64)

	sess.DeleteFrom(userFollowingHashtagTable).Where("hashtag_id = ?", tagId)

	return c.NoContent(http.StatusOK)
}

func listTags(c echo.Context) error {

	var tags []hashtag
	sess.Select("*").From(hashtagTable).Load(&tags)

	var res responseHashtag
	res.Hashtags = tags

	return c.JSON(http.StatusOK,res)
}

func followingHashtags(c echo.Context) error {

	user := c.Get("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)

	sub := claims["sub"].(string)

	var hashtagsIds []int64
	sess.Select("hashtag_id").From(userFollowingHashtagTable).Where("user_id = ?", sub).LoadValues(&hashtagsIds)

	var tags []hashtag
	sess.Select("*").From(hashtagTable).Where("id IN ?", hashtagsIds).Load(&tags)

	var res responseHashtag
	res.Hashtags = tags

	return c.JSON(http.StatusOK,res)
}