package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
)

type PostgresConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

func (c PostgresConfig) Dialect() string {
	return "postgres"
}
func (c PostgresConfig) ConnectionInfo() string {
	if c.Password == "" {
		return fmt.Sprintf("host=%s port=%d user=%s dbname=%s "+
			"sslmode=disable", c.Host, c.Port, c.User, c.Name)
	}
	return fmt.Sprintf("host=%s port=%d user=%s password=%s "+
		"dbname=%s sslmode=disable", c.Host, c.Port, c.User,
		c.Password, c.Name)
}

func DefaultPostgresConfig() PostgresConfig {
	return PostgresConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "postgres",
		Password: "123",
		Name:     "sja_dev",
	}
}

type Config struct {
	HTTPPort                    uint            `json:"http_port"` // т.к. в задании указано "Адрес", поэтому две составляющие
	HTTPIP                      string          `json:"http_ip"`
	GRPCPort                    uint            `json:"grpc_port"`
	GRPCIP                      string          `json:"grpc_ip"`
	LinesProviderPort           uint            `json:"lines_provider_port"`
	LinesProviderIP             string          `json:"lines_provider_ip"`
	Logmode                     bool            `json:"log_mode"`
	FirstSyncNumOfAttempts      uint            `json:"first_sync_num_of_attempts"`
	FirstSyncIntervalBWAttempts uint            `json:"first_sync_interval_bw_attempts"`
	Intervals                   map[string]uint `json:"intervals"`
	Database                    PostgresConfig  `json:"database"`
}

func DefaultConfig() Config {
	return Config{
		HTTPPort:                    9000,
		HTTPIP:                      "localhost",
		GRPCPort:                    9001,
		GRPCIP:                      "localhost",
		LinesProviderPort:           8000,
		LinesProviderIP:             "localhost",
		Logmode:                     false,
		FirstSyncNumOfAttempts:      3,
		FirstSyncIntervalBWAttempts: 1,
		Intervals: map[string]uint{
			"baseball": 1,
			"football": 1,
			"soccer":   1,
		},
		Database: DefaultPostgresConfig(),
	}
}

func LoadConfig(configReq bool) Config {
	f, err := os.Open(".config")
	if err != nil {
		if configReq {
			fmt.Println("A .config file must be provided with the -prod flag, shutting down.")
			panic(err)
		}

		fmt.Println("Using the default config...")
		return DefaultConfig()
	}
	defer func() {
		if err := f.Close(); err != nil {
			log.Fatal(err)
		}
	}()

	var c Config
	dec := json.NewDecoder(f)
	err = dec.Decode(&c)
	if err != nil {
		panic(err)
	}

	if len(c.Intervals) != len(AvailableSportNames) {
		log.Fatal("Not all the intervals were provided or count of provided intervals is more than count of available sport names")
	}

	for name, interval := range c.Intervals {
		_, found := AvailableSportNames[name]
		if !found {
			log.Fatal("A sport name must be one of the following: " + strings.Join(AvailableSportNames.GetKeys(), ", "))
		}

		if interval == 0 {
			log.Fatal("An interval can't be 0")
		}
	}

	if c.HTTPPort == 0 {
		log.Fatal("HTTP port can't be 0")
	}
	if c.GRPCPort == 0 {
		log.Fatal("gRPC port can't be 0")
	}
	if c.LinesProviderPort == 0 {
		log.Fatal("LinesProvider's HTTP port can't be 0")
	}
	if c.FirstSyncNumOfAttempts == 0 {
		log.Fatal("A number of attempts can't be 0")
	}
	if c.FirstSyncIntervalBWAttempts == 0 {
		log.Fatal("An interval between attempts can't be 0")
	}

	fmt.Println("Successfully loaded .config")
	return c
}
