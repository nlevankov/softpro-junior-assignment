package main

import (
	"errors"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	"github.com/lib/pq"
	"github.com/softpro-junior-assignment/pb"
	"io"
	"strings"
	"time"
)

type sportsLinesServer struct {
	db *gorm.DB
}

func (s *sportsLinesServer) SubscribeOnSportsLines(stream pb.SportsLinesService_SubscribeOnSportsLinesServer) error {
	errs := make(chan error, 2)
	abortStreamHandler := make(chan struct{}, 1)

	go streamHandler(stream, abortStreamHandler, errs, s.db)

	select {
	case e := <-errs:
		if e == io.EOF {
			return nil
		}

		abortStreamHandler <- struct{}{}

		switch e.(type) {
		case *pq.Error:
			return errors.New("There is a problem with getting lines from the storage")
		}

		return e
	}
}

func streamHandler(stream pb.SportsLinesService_SubscribeOnSportsLinesServer, abortStreamHandler <-chan struct{}, errs chan<- error, db *gorm.DB) {
	prevParamsSet := make(Set)
	abortSendDeltas := make(chan struct{})

	// dummy func, только для того, чтоб при старте когда запрос придет было что "абортить", см далее
	go func() {
		<-abortSendDeltas
	}()

	for {
		req, err := stream.Recv()

		select {
		case <-abortStreamHandler:
			return
		default:
		}

		// стопаем SendDeltas либо dummyfunc при старте streamHandler, строго небуферизированный
		abortSendDeltas <- struct{}{}

		if err != nil {
			errs <- err
			return
		}

		if req.Interval == 0 {
			errs <- errors.New("Interval was not provided")
			return
		}

		if req.SportNames == nil {
			errs <- errors.New("Sport names were not provided")
			return
		}

		if len(req.SportNames) > 3 {
			errs <- errors.New("More than 3 sport names provided")
			return
		}

		for _, name := range req.SportNames {
			_, found := AvailableSportNames[name]
			if !found {
				errs <- errors.New("A sport name must be one of the following: " + strings.Join(AvailableSportNames.GetKeys(), ", "))
				return
			}
		}

		newParamsSet := NewSetFromSlice(req.SportNames)
		if len(newParamsSet) == len(prevParamsSet) && newParamsSet.IsSubsetOf(prevParamsSet) {
			go sendDeltas(req.Interval, db, abortSendDeltas, errs, newParamsSet, stream)
		} else {
			err = sendLines(db, newParamsSet, stream)
			if err != nil {
				errs <- err
				return
			}

			// todo можно подождать, прежде чем присылать почти сразу нулевые дельты, но возможно все-таки этого не стоит делать?
			time.Sleep(time.Duration(req.Interval) * time.Second)

			go sendDeltas(req.Interval, db, abortSendDeltas, errs, newParamsSet, stream)
		}

		prevParamsSet = newParamsSet
	}

}

// в params уже должны быть линии, от которых будут присылаться дельты, эта горутина всегда присылает только дельты
func sendDeltas(interval uint32, db *gorm.DB, abort <-chan struct{}, errs chan<- error, params Set, stream pb.SportsLinesService_SubscribeOnSportsLinesServer) {
	for {
		select {
		case <-abort:
			return
		default:
			// todo возможно ли сделать одним запросом получение всех линий? и так и этак думал, но что-то не придумал
			var resp pb.SubscribeOnSportsLinesResponse
			for sportName, line := range params {
				sportInfo := pb.SportInfo{Name: sportName}
				query := `SELECT line FROM ` + sportName + `s ORDER BY id DESC LIMIT 1`
				err := db.Raw(query).Scan(&sportInfo).Error
				if err != nil {
					errs <- err
					return
				}
				sportInfo.Line = line - sportInfo.Line
				resp.SportInfos = append(resp.SportInfos, &sportInfo)
			}

			err := stream.Send(&resp)
			if err != nil {
				errs <- err
				return
			}

			time.Sleep(time.Duration(interval) * time.Second)
		}
	}
}

func sendLines(db *gorm.DB, params Set, stream pb.SportsLinesService_SubscribeOnSportsLinesServer) error {
	// todo возможно ли сделать одним запросом получение всех линий? и так и этак думал, но что-то не придумал
	var resp pb.SubscribeOnSportsLinesResponse
	for sportName := range params {
		sportInfo := pb.SportInfo{Name: sportName}
		query := `SELECT line FROM ` + sportName + `s ORDER BY id DESC LIMIT 1`
		err := db.Raw(query).Scan(&sportInfo).Error
		if err != nil {
			return err
		}
		params[sportName] = sportInfo.Line
		resp.SportInfos = append(resp.SportInfos, &sportInfo)
	}

	err := stream.Send(&resp)
	if err != nil {
		return err
	}

	return nil
}
