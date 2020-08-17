package main

import (
	"encoding/json"
	"errors"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	_ "github.com/lib/pq"
	"log"
	"net/http"
	"sync"
	"time"
)

func RenderJSON(w http.ResponseWriter, result interface{}, StatusCode int, err interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(StatusCode)

	enc := json.NewEncoder(w)
	d := map[string]interface{}{"Result": result, "Error": err}
	if err := enc.Encode(d); err != nil {
		log.Println(err)
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func getLine(interval uint, db *gorm.DB, sportName, linesProviderAddr string, abort <-chan struct{}, n *sync.WaitGroup) {
	defer n.Done()
	// todo писал на этот счет в getFirstLine
	firstPartOfQuery := `INSERT  INTO "` + sportName + `s" ("line") VALUES (`
	addr := linesProviderAddr + sportName

	for {
		select {
		case <-abort:
			return
		default:
			resp, err := http.Get(addr)
			if err != nil {
				log.Println(err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				log.Println("Status code isn't OK 200 (from LinesProvider, sport name: " + sportName + ")")
				return
			}

			dst := ParsedJSON{}
			err = json.NewDecoder(resp.Body).Decode(&dst)
			if err != nil {
				log.Println(err)
				return
			}

			for _, line := range dst.Lines {
				// хотя бы частичная параметризация
				err = db.Exec(firstPartOfQuery+`?)`, line).Error
				if err != nil {
					log.Println(err)
					return
				}
			}

			time.Sleep(time.Duration(interval) * time.Second)
		}
	}
}

func getFirstLine(db *gorm.DB, sportName, linesProviderAddr string, e chan<- error, n *sync.WaitGroup) {
	defer n.Done()

	// todo знаю, что это sql инъекция, но пробовал по разному делать, и чз gorm api и чз database/sql. Проблема в том, что
	//  с плейсхолдерами ($1, ($1), ?, (?)) не хочет работать запрос, если подставлять имена таблиц.
	//  Как временное решение от sql инъекции здесь спасает проверка при загрузке конфига на допустимые имена спортов в глобальной мапке AvailableSportNames.
	//  Наткнулся на обсуждение этой проблемы https://github.com/golang/go/issues/18478
	//  Возможно как то можно иначе построить запрос, чтобы избежать такого рода подстановки или вообще есть иное решение, очень
	//  хотелось бы узнать о нем.
	firstPartOfQuery := `INSERT  INTO "` + sportName + `s" ("line") VALUES (`
	addr := linesProviderAddr + sportName

	resp, err := http.Get(addr)
	if err != nil {
		e <- err
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		e <- errors.New("status code isn't OK 200 (from LinesProvider)")
		return
	}

	dst := ParsedJSON{}
	err = json.NewDecoder(resp.Body).Decode(&dst)
	if err != nil {
		e <- err
		return
	}

	for _, line := range dst.Lines {
		// хотя бы частичная параметризация
		err = db.Exec(firstPartOfQuery+`?)`, line).Error
		if err != nil {
			e <- err
			return
		}
	}
}

type ParsedJSON struct {
	Lines map[string]string `json:"lines"`
}
