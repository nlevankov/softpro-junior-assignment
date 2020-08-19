package services

import (
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	_ "github.com/lib/pq"
	"log"
	"os"
	"time"
)

type ServicesConfig func(*Services) error

type Services struct {
	DB      *gorm.DB
	logFile *os.File
}

func NewServices(cfgs ...ServicesConfig) (*Services, error) {
	var s Services
	for _, cfg := range cfgs {

		if err := cfg(&s); err != nil {
			return nil, err
		}
	}
	return &s, nil
}

func WithGorm(dialect, connectionInfo string, num, interval int) ServicesConfig {
	return func(s *Services) error {
		var err error
		for i := 0; i < num; i++ {
			db, err := gorm.Open(dialect, connectionInfo)
			if err == nil {
				log.Println("Successfully connected to the storage")
				s.DB = db
				return nil
			}

			log.Printf("Can't connect to the storage, next try in %d second(s) (%d attempt of %d)\n", interval, i+1, num)
			time.Sleep(time.Duration(interval) * time.Second)
		}

		log.Println("Can't connect to the storage, check your connection info in .config (if provided) or default connection info or the storage availability.")
		return err
	}
}

func WithLogMode(mode bool) ServicesConfig {
	return func(s *Services) error {
		if mode {
			var err error
			s.logFile, err = os.OpenFile("log.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				s.CloseStorage()
				log.Fatal(err)
			}

			// разделяем сессии работы приложения
			if _, err := s.logFile.Write([]byte("\r\n")); err != nil {
				s.Close()
				log.Fatal(err)
			}

			s.DB.SetLogger(log.New(s.logFile, "\r\n", log.LstdFlags))
			return s.DB.LogMode(true).Error
		}
		return nil
	}
}

func WithSetSchema(mode bool) ServicesConfig {
	return func(s *Services) error {
		if mode {
			return setSchema(s.DB)
		}
		return nil
	}
}

func (s *Services) Close() {
	if s.logFile != nil {
		if err := s.logFile.Close(); err != nil {
			log.Println(err)
		}
	}
	s.CloseStorage()
}

func (s *Services) CloseStorage() {
	if err := s.DB.Close(); err != nil {
		log.Println(err)
	}
}

func setSchema(db *gorm.DB) error {
	err := db.Debug().Exec("DROP SCHEMA public CASCADE").Error
	if err != nil {
		return err
	}

	err = db.Debug().Exec("CREATE SCHEMA public").Error
	if err != nil {
		return err
	}

	err = db.Debug().CreateTable(
		&Baseball{},
		&Football{},
		&Soccer{},
	).Error
	if err != nil {
		return err
	}

	return nil
}

type Baseball struct {
	ID   *uint `gorm:"primary_key"`
	Line *float32
}

type Football struct {
	ID   *uint `gorm:"primary_key"`
	Line *float32
}

type Soccer struct {
	ID   *uint `gorm:"primary_key"`
	Line *float32
}
