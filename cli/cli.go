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

	conn, _ := grpc.Dial(host+":"+strconv.Itoa(int(port)), grpc.WithInsecure())
	defer conn.Close()

	registry := pbrc.NewRecordCollectionServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	switch os.Args[1] {
	case "get":
		i, _ := strconv.Atoi(os.Args[2])
		rec, err := registry.GetRecords(ctx, &pbrc.GetRecordsRequest{Filter: &pbrc.Record{Release: &pbgd.Release{Id: int32(i)}}})

		if err == nil {
			for _, r := range rec.GetRecords() {
				fmt.Printf("Release: %v\n", r.GetRelease())
				fmt.Printf("Metadata: %v\n", r.GetMetadata())
			}
		}
	case "all":
		rec, err := registry.GetRecords(ctx, &pbrc.GetRecordsRequest{Filter: &pbrc.Record{}})

		if err == nil {
			fmt.Printf("%v records in the collection\n", len(rec.GetRecords()))
		}
	}
}
