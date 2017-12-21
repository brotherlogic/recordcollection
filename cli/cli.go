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

	//Needed to pull in gzip encoding init
	_ "google.golang.org/grpc/encoding/gzip"
)

func main() {
	host, port, err := utils.Resolve("recordcollection")

	if err != nil {
		log.Fatalf("Unable to locate recordcollection server")
	}

	conn, _ := grpc.Dial(host+":"+strconv.Itoa(int(port)), grpc.WithInsecure())
	defer conn.Close()

	registry := pbrc.NewRecordCollectionServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	switch os.Args[1] {
	case "get":
		i, _ := strconv.Atoi(os.Args[2])
		rec, err := registry.GetRecords(ctx, &pbrc.GetRecordsRequest{Force: true, Filter: &pbrc.Record{Release: &pbgd.Release{Id: int32(i)}}})

		if err == nil {
			for _, r := range rec.GetRecords() {
				fmt.Printf("Release: %v\n", r.GetRelease())
				fmt.Printf("Metadata: %v\n", r.GetMetadata())
			}
		} else {
			fmt.Printf("Error: %v", err)
		}
	case "all":
		rec, err := registry.GetRecords(ctx, &pbrc.GetRecordsRequest{Filter: &pbrc.Record{}})

		if err == nil {
			fmt.Printf("%v records in the collection\n", len(rec.GetRecords()))
		}
	}
}
