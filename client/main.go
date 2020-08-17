package main

import (
	"flag"
	"github.com/softpro-junior-assignment/pb"
	"io"
	"log"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

// должен совпадать с адресом сервера
const addr = "localhost:9001"

func main() {
	option := flag.Int("o", 1, "Command to run")
	flag.Parse()

	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	client := pb.NewSportsLinesServiceClient(conn)

	switch *option {
	case 1:
		SubscribeOnSportsLines(client)
	}
}

func SubscribeOnSportsLines(client pb.SportsLinesServiceClient) {
	stream, err := client.SubscribeOnSportsLines(context.Background())
	log.SetFlags(log.Ltime)
	if err != nil {
		log.Fatal(err)
	}

	req := pb.SubscribeOnSportsLinesRequest{
		Interval:   1,
		SportNames: []string{"baseball", "football"},
	}

	doneCh := make(chan struct{})
	go func() {
		for {
			res, err := stream.Recv()
			if err == io.EOF {
				log.Println("got EOF by stream.Recv()")
				doneCh <- struct{}{}
				break
			}
			if err != nil {
				log.Println(err)
				doneCh <- struct{}{}
				break
			}

			if res != nil {
				log.Println(res.SportInfos)
			}
		}
	}()

	err = stream.Send(&req)
	if err != nil {
		log.Println(err)
	}

	req = pb.SubscribeOnSportsLinesRequest{
		Interval:   2,
		SportNames: []string{"baseball", "football", "soccer"},
	}
	time.Sleep(time.Second * 5)

	err = stream.Send(&req)
	if err != nil {
		log.Println(err)
	}

	req = pb.SubscribeOnSportsLinesRequest{
		Interval:   1,
		SportNames: []string{"baseball"},
	}
	time.Sleep(time.Second * 5)

	err = stream.Send(&req)
	if err != nil {
		log.Println(err)
	}

	req = pb.SubscribeOnSportsLinesRequest{
		Interval:   1,
		SportNames: []string{"soccer"},
	}
	time.Sleep(time.Second * 5)

	err = stream.Send(&req)
	if err != nil {
		log.Println(err)
	}
	time.Sleep(time.Second * 5)

	stream.CloseSend()
	<-doneCh
}
