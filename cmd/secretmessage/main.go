package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/neufeldtech/secretmessage-go/pkg/secretmessage"
	"github.com/neufeldtech/secretmessage-go/pkg/secretslack"
	"github.com/prometheus/common/log"
	_ "go.elastic.co/apm/module/apmgormv2"
	postgres "go.elastic.co/apm/module/apmgormv2/driver/postgres"

	"github.com/spf13/viper"
	"go.elastic.co/apm/module/apmhttp"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
)

type BotConfig struct {
	Slack struct {
		AppURL       string `yaml:"appURL"`
		SigingSecret string `yaml:"signingSecret"`
		ClientID     string `yaml:"clientID"`
		ClientSecret string `yaml:"clientSecret"`
		CallbackURL  string `yaml:"callbackURL"`
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
		ExpirationTime int    `yaml:"expirationTime"`
	} `yaml:"core"`
}

func initConfig() *BotConfig {
	viper.AddConfigPath("./config")
	viper.SetConfigName("config")
	viper.SetConfigName("secretmessage")
	viper.SetConfigType("yaml")

	viper.SetDefault("secretmessage.server.port", 8080)
	viper.SetDefault("secretmessage.core.expirationTime", 10)

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("error initializing config file: %v", err)
	}

	config := BotConfig{}
	if err := viper.Unmarshal(&config); err != nil {
		log.Fatalf("error unmarshaling config file to struct: %v", err)
	}

	config.Database.URL = fmt.Sprintf(
		"postgres://%s:%s@%s/%s", config.Database.Username, config.Database.Password, config.Database.Host, config.Database.Name,
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
		SlackToken:      "",
		SigningSecret:   config.Slack.SigingSecret,
		AppURL:          config.Slack.AppURL,
		LegacyCryptoKey: config.Core.CryptoKey,
		DatabaseURL:     config.Database.URL,
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

	db, err := gorm.Open(postgres.Open(conf.DatabaseURL), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}
	d, _ := db.DB()
	d.SetMaxIdleConns(10)
	d.SetMaxOpenConns(10)

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
