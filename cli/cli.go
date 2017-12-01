package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/brotherlogic/goserver/utils"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	pbgd "github.com/brotherlogic/godiscogs"

	pbrc "github.com/brotherlogic/recordcollection/proto"
)

func main() {
	host, port, err := utils.Resolve("recordcollection")

	if err != nil {
		log.Fatalf("Unable to locate recordcollection server")
	}

	log.Printf("DIALLING %v, %v, %v", host, port, err)
	conn, _ := grpc.Dial(host+":"+strconv.Itoa(int(port)), grpc.WithInsecure())
	defer conn.Close()

	registry := pbrc.NewDiscogsServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	switch os.Args[1] {
	case "get":
		i, _ := strconv.Atoi(os.Args[2])
		rec, err := registry.GetRecords(ctx, &pbrc.GetRecordsRequest{Filter: &pbrc.Record{Release: &pbgd.Release{Id: int32(i)}}})

		if err == nil {

			if len(rec.GetRecords()) == 0 {
				log.Printf("No records found!")
			}

			for _, r := range rec.GetRecords() {
				fmt.Printf("Release: %v", r.GetRelease())
				fmt.Printf("Metadata: %v", r.GetMetdata())
			}
		} else {
			log.Printf("Error: %v", err)
		}
	case "all":
		rec, err := registry.GetRecords(ctx, &pbrc.GetRecordsRequest{Filter: &pbrc.Record{Release: &pbgd.Release{}}})

		if err == nil {

			if len(rec.GetRecords()) == 0 {
				log.Printf("No records found!")
			}

			fmt.Printf("%v records in the collection\n", len(rec.GetRecords()))
			for i, r := range rec.GetRecords() {
				fmt.Printf("%v. %v\n", i, r)
			}
		} else {
			log.Printf("Error: %v", err)
		}
	}
}
