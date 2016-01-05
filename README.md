# google-play-review-watcher

google-play-review-watcher is an incoming webhook that posts app reviews from the Google Play to Slack

### Installation

1. `go get github.com/Konboi/go-google-play-review-watcher`

2. Create a SQLite database using `sqlite3 DB_NAME < ./schema.sql`

3. Copy `config_test.yml` and Add your webhook url, Google Play app id and the path to your database.

4. `go-google-play-review-watcher -c <EDIT CONFIG FILE PATH>`

5. Run periodically using cron or some other job scheduler
