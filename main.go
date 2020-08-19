package main

import (
	"flag"
	"fmt"
	"github.com/gorilla/mux"
	"google.golang.org/grpc"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/softpro-junior-assignment/pb"
	"github.com/softpro-junior-assignment/services"
)

// read only in non main goroutines
var isSynced bool

var AvailableSportNames = Set{
	"soccer":   0,
	"football": 0,
	"baseball": 0,
}

func main() {
	// flags' initialization

	prodFlagPtr := flag.Bool("prod", false, "Provide this flag "+
		"in production. This ensures that a .config file is "+
		"provided before the application starts.")

	setSchemaFlagPtr := flag.Bool("setschema", false, "WARNING: it is destructive action. Provide this flag "+
		"to set the db schema. If '-prod' flag is provided, this flag will be ignored.")

	flag.Parse()

	// the app's config's initialization

	cfg := LoadConfig(*prodFlagPtr)

	// creating services

	s, err := services.NewServices(
		services.WithGorm(cfg.Database.Dialect(), cfg.Database.ConnectionInfo(), int(cfg.StorageConnNumOfAttempts), int(cfg.StorageConnIntervalBWAttempts)),
		services.WithLogMode(cfg.Logmode),
		services.WithSetSchema(!(*prodFlagPtr) && *setSchemaFlagPtr),
	)
	must(err)
	defer s.Close()

	// starting HTTP server

	r := mux.NewRouter()
	r.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		RenderJSON(w, nil, http.StatusNotFound, "No such endpoint exists")
	})
	r.MethodNotAllowedHandler = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		RenderJSON(w, nil, http.StatusMethodNotAllowed, "Wrong http method")
	})

	ReadyHandler := func(w http.ResponseWriter, r *http.Request) {
		// правильно ли я понимаю, что именно в этом заключается проверка соединения с хранилищем?
		// т.е. в использовании Ping()
		if err := s.DB.DB().Ping(); err != nil {
			RenderJSON(w, nil, http.StatusInternalServerError, "Failed to connect to the storage")
			return
		}

		if !isSynced {
			RenderJSON(w, nil, http.StatusInternalServerError, "The storage is not synced with the Lines Provider")
			return
		}

		RenderJSON(w, nil, http.StatusOK, nil)
	}
	r.HandleFunc("/ready", ReadyHandler).Methods(http.MethodGet)

	httpAdress := fmt.Sprintf(cfg.HTTPIP+":%d", cfg.HTTPPort)
	go func() {
		must(http.ListenAndServe(httpAdress, r))
	}()
	fmt.Printf("Started HTTP server on %v\n", httpAdress)

	// try to sync the storage

	linesProviderAddr := fmt.Sprintf("http://%v:%d/api/v1/lines/", cfg.LinesProviderIP, cfg.LinesProviderPort)

	errs := make(chan error)
	var n sync.WaitGroup
	var globalErrSlice []error
	var localErrSlice []error

	for i := 0; i < int(cfg.FirstSyncNumOfAttempts); i++ {
		for name := range cfg.Intervals {
			n.Add(1)
			go func(name string) {
				getFirstLine(s.DB, name, linesProviderAddr, errs, &n)
			}(name)
		}

		go func() {
			n.Wait()
			close(errs)
		}()

		for e := range errs {
			localErrSlice = append(localErrSlice, e)
		}

		if localErrSlice == nil {
			isSynced = true
			break
		}

		globalErrSlice = append(globalErrSlice, localErrSlice...)
		localErrSlice = nil
		errs = make(chan error)

		log.Printf("Can't sync the storage with the lines provider, next try in %d second(s) (%d attempt of %d)\n", cfg.FirstSyncIntervalBWAttempts, i+1, cfg.FirstSyncNumOfAttempts)
		time.Sleep(time.Duration(cfg.FirstSyncIntervalBWAttempts) * time.Second)
	}

	if !isSynced {
		log.Println("Failed to sync the storage\n")
		for _, err := range globalErrSlice {
			log.Println(err)
		}
		os.Exit(1)
	}

	// launch workers

	abort := make(chan struct{})

	for name, N := range cfg.Intervals {
		n.Add(1)
		go func(N uint, name string) {
			getLine(N, s.DB, name, linesProviderAddr, abort, &n)
		}(N, name)
	}

	go func() {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)
		sig := <-sigs
		log.Printf("Got <%v> signal, shutting down the workers...", sig)
		close(abort)
	}()

	// start gRPC server

	grpcAdress := fmt.Sprintf(cfg.GRPCIP+":%d", cfg.GRPCPort)
	lis, err := net.Listen("tcp", grpcAdress)
	if err != nil {
		s.Close()
		log.Fatalf("Failed to listen tcp port for gRPC server: %v", err)
	}
	server := grpc.NewServer()
	pb.RegisterSportsLinesServiceServer(server, &sportsLinesServer{db: s.DB})
	go func() {
		must(server.Serve(lis))
	}()
	fmt.Printf("Started gRPC server on %v\nSend SIGINT or SIGTERM to exit correctly\n", grpcAdress)

	n.Wait()
}
