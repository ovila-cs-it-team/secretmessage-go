package main

import (
	"fmt"
	"github.com/neufeldtech/secretmessage-go/pkg/secretmessage"
	"github.com/neufeldtech/secretmessage-go/pkg/secretslack"
	"github.com/prometheus/common/log"
	"github.com/spf13/viper"
	_ "go.elastic.co/apm/module/apmgormv2"
	postgres "go.elastic.co/apm/module/apmgormv2/driver/postgres"
	"net/http"
	"os"
	"strconv"
	"time"

	"go.elastic.co/apm/module/apmhttp"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
)

type BotConfig struct {
	Slack struct {
		AppURL        string `yaml:"appURL"`
		SigningSecret string `yaml:"signingSecret"`
		ClientID      string `yaml:"clientID"`
		ClientSecret  string `yaml:"clientSecret"`
		CallbackURL   string `yaml:"callbackURL"`
		Token         string `yaml:"token"`
	} `yaml:"slack"`
	Server struct {
		Port int64 `yaml:"port"`
	} `yaml:"server"`
	Database struct {
		Name     string `yaml:"name"`
		Host     string `yaml:"host"`
		Username string `yaml:"username"`
		Password string `yaml:"password"`
		URL      string `yaml:"url"`
	} `yaml:"database"`
	Core struct {
		CryptoKey      string `yaml:"cryptoKey"`
		ExpirationTime int64  `yaml:"expirationTime"`
	} `yaml:"core"`
}

func initConfig() *BotConfig {
	viper.SetDefault("secretmessage.server.port", 8080)
	viper.SetDefault("secretmessage.core.expirationTime", 86400)

	config := BotConfig{}

	port, err := strconv.ParseInt(os.Getenv("SERVER_PORT"), 10, 64)
	if err != nil {
		log.Fatal(err)
	}

	expirationTime, err := strconv.ParseInt(os.Getenv("EXPIRATION_TIME"), 10, 64)
	if err != nil {
		log.Fatal(err)
	}

	config.Server.Port = port
	config.Core.ExpirationTime = expirationTime

	config.Slack.Token = os.Getenv("SLACK_TOKEN")
	config.Slack.ClientID = os.Getenv("SLACK_CLIENT_ID")
	config.Slack.ClientSecret = os.Getenv("SLACK_CLIENT_SECRET")
	config.Slack.CallbackURL = os.Getenv("SLACK_CALLBACK_URL")
	config.Slack.SigningSecret = os.Getenv("SLACK_SECRET")
	config.Slack.AppURL = os.Getenv("SLACK_APP_URL")
	config.Core.CryptoKey = os.Getenv("HASH_CRYPTO_KEY")

	config.Database.URL = fmt.Sprintf(
		"postgres://%s:%s@%s/%s",
		os.Getenv("DATABASE_USERNAME"), os.Getenv("DATABASE_PASSWORD"), os.Getenv("DATABASE_HOST"), os.Getenv("DATABASE_NAME"),
	)

	return &config
}

func main() {
	// Setup custom HTTP Client for calling Slack
	secretslack.SetHTTPClient(apmhttp.WrapClient(
		&http.Client{
			Timeout: time.Second * 5,
		},
	))

	config := initConfig()

	conf := secretmessage.Config{
		Port:            config.Server.Port,
		SlackToken:      config.Slack.Token,
		SigningSecret:   config.Slack.SigningSecret,
		AppURL:          config.Slack.AppURL,
		LegacyCryptoKey: config.Core.CryptoKey,
		DatabaseURL:     config.Database.URL,
		ExpirationTime:  config.Core.ExpirationTime,
		OauthConfig: &oauth2.Config{
			ClientID:     config.Slack.ClientID,
			ClientSecret: config.Slack.ClientSecret,
			RedirectURL:  config.Slack.CallbackURL,
			Scopes:       []string{"chat:write", "commands", "workflow.steps:execute"},
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://slack.com/oauth/v2/authorize",
				TokenURL: "https://slack.com/api/oauth.v2.access",
			},
		},
	}

	log.Info("Opening database connection...")
	db, err := gorm.Open(postgres.Open(conf.DatabaseURL), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}
	d, _ := db.DB()
	d.SetMaxIdleConns(10)
	d.SetMaxOpenConns(10)

	log.Info("Starting database migration...")
	db.AutoMigrate(secretmessage.Secret{})
	db.AutoMigrate(secretmessage.Team{})

	controller := secretmessage.NewController(
		conf,
		db,
	)

	go secretmessage.StayAwake(conf)

	r := controller.ConfigureRoutes()
	log.Infof("Booted and listening on port %v", conf.Port)
	r.Run(fmt.Sprintf("0.0.0.0:%v", conf.Port))
}
