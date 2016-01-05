package main

import (
	"database/sql"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	_ "github.com/mattn/go-sqlite3"
	"github.com/russross/meddler"
	"gopkg.in/yaml.v2"
)

type Config struct {
	AppId       string `yaml:"app_id"`
	ReviewCount int    `yaml:"review_count"`
	BotName     string `yaml:"bot_name"`
	IconEmoji   string `yaml:"icon_emoji"`
	MessageText string `yaml:"message_text`
	WebHookUri  string `yaml:"web_hook_uri"`
	DbPath      string `yaml:"db_path"`
}

type Review struct {
	Id        int
	Author    string
	AuthorUri string `meddler:"author_uri"`
	Title     string
	Message   string
	Rate      string
	UpdatedAt time.Time `meddler:"updated_at,localtime"`
}

type DBH struct {
	*sql.DB
}

const (
	TABLE_NAME                = "review"
	BASE_URL                  = "https://play.google.com/store/apps/details"
	REVIEW_CLASS_NAME         = ".single-review"
	AUTHOR_NAME_CLASS_NAME    = ".review-info span.author-name a"
	REVIEW_DATE_CLASS_NAME    = ".review-info .review-date"
	REVIEW_TITLE_CLASS_NAME   = ".review-body .review-title"
	REVIEW_MESSAGE_CLASS_NAME = ".review-body"
	REVIEW_RATE_CLASS_NAME    = ".review-info-star-rating .tiny-star"
	RAITING_EMOJI             = ":start:"
	MAX_REVIEW_NUM            = 40
)

var (
	dbh        *DBH
	configFile = flag.String("c", "config.yml", "config file")
)

func init() {
	meddler.Default = meddler.SQLite
}

func GetDBH() *DBH {
	return dbh
}

func (dbh *DBH) LastInsertId(tableName string) int {
	row := dbh.QueryRow(`SELECT id FROM ` + tableName + ` ORDER BY id DESC LIMIT 1`)

	var id int
	err := row.Scan(&id)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return 0
		}
		log.Fatal(err)
	}

	return id
}

func NewConfig(path string) (config Config, err error) {
	config = Config{}

	data, err := ioutil.ReadFile(path)
	if err != nil {
		return config, err
	}

	if err := yaml.Unmarshal(data, &config); err != nil {
		return config, err
	}

	if config.AppId == "" {
		return config, fmt.Errorf("Please Set Your Google Play App Id.")
	}

	if config.ReviewCount > MAX_REVIEW_NUM || config.ReviewCount < 1 {
		return config, fmt.Errorf("Please Set Num Between 1 and 40.")
	}

	db, err := sql.Open("sqlite3", config.DbPath)
	if err != nil {
		return config, err
	}

	err = db.Ping()
	if err != nil {
		return config, err
	}

	dbh = &DBH{db}

	uri := fmt.Sprintf("%s?id=%s", BASE_URL, config.AppId)
	res, err := http.Get(uri)
	if err != nil {
		return config, err
	}

	if res.StatusCode == http.StatusNotFound {
		return config, fmt.Errorf("AppID: %s is not exists", config.AppId)
	}

	return config, err
}

func main() {
	flag.Parse()

	config, err := NewConfig(*configFile)
	if err != nil {
		log.Println(err)
		return
	}
	reviews, err := GetReview(config)
	if err != nil {
		log.Println(err)
		return
	}

	reviews, err = SaveReviews(reviews)
	if err != nil {
		log.Println(err)
		return
	}
}

func GetReview(config Config) ([]Review, error) {
	uri := fmt.Sprintf("%s?id=%s", BASE_URL, config.AppId)
	doc, err := goquery.NewDocument(uri)

	if err != nil {
		return nil, err
	}

	reviews := []Review{}

	doc.Find(REVIEW_CLASS_NAME).Each(func(i int, s *goquery.Selection) {
		authorNode := s.Find(AUTHOR_NAME_CLASS_NAME)

		authorName := authorNode.Text()
		authorUri, _ := authorNode.Attr("href")

		dateNode := s.Find(REVIEW_DATE_CLASS_NAME)

		date, err := time.Parse("2006年01月02日", dateNode.Text())
		if err != nil {
			return
		}

		reviewTitle := s.Find(REVIEW_TITLE_CLASS_NAME).Text()
		reviewMessage := s.Find(REVIEW_MESSAGE_CLASS_NAME).Text()

		reviewRateNode := s.Find(REVIEW_RATE_CLASS_NAME)
		rateMessage, _ := reviewRateNode.Attr("aria-label")

		rate := parseRate(rateMessage)

		review := Review{
			Author:    authorName,
			AuthorUri: authorUri,
			Title:     reviewTitle,
			Message:   reviewMessage,
			Rate:      rate,
			UpdatedAt: date,
		}

		reviews = append(reviews, review)
	})

	return reviews, nil
}

func parseRate(message string) string {
	rate := ""

	switch {
	case strings.Contains(message, "1つ星"):
		rate = strings.Repeat(RAITING_EMOJI, 1)
	case strings.Contains(message, "2つ星"):
		rate = strings.Repeat(RAITING_EMOJI, 2)
	case strings.Contains(message, "3つ星"):
		rate = strings.Repeat(RAITING_EMOJI, 3)
	case strings.Contains(message, "4つ星"):
		rate = strings.Repeat(RAITING_EMOJI, 4)
	case strings.Contains(message, "5つ星"):
		rate = strings.Repeat(RAITING_EMOJI, 5)
	}

	return rate
}

func SaveReviews(reviews []Review) ([]Review, error) {
	postReviews := []Review{}

	for _, review := range reviews {
		var id int
		row := dbh.QueryRow(`SELECT id FROM `+TABLE_NAME+` WHERE author_uri = ?`, review.AuthorUri)
		err := row.Scan(&id)

		if err != nil {
			if err.Error() != "sql: no rows in result set" {
				return postReviews, err
			}

			id = 0
		}
		if id == 0 {
			dbh := GetDBH()
			review.Id = dbh.LastInsertId(TABLE_NAME) + 1
			err := meddler.Insert(dbh, TABLE_NAME, review)
			if err != nil {
				return postReviews, err
			}
			postReviews = append(postReviews, review)
		}
		fmt.Println(review.Author)
	}

	return postReviews, nil
}
