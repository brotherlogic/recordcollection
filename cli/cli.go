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
				fmt.Printf("Images: %v\n", len(r.GetRelease().GetImages()))
				fmt.Printf("1 %v, %v, %v %v", r.GetMetadata().GetDateAdded() > (time.Now().AddDate(0, -3, 0).Unix()), r.GetMetadata().DateAdded, r.GetRelease().Rating == 0, r.GetRelease().FolderId)
			}
		} else {
			fmt.Printf("Error: %v", err)
		}
	case "all":
		rec, err := registry.GetRecords(ctx, &pbrc.GetRecordsRequest{Filter: &pbrc.Record{}})

		if err == nil {
			fmt.Printf("%v records in the collection\n", len(rec.GetRecords()))
		}
	case "force":
		i, _ := strconv.Atoi(os.Args[2])
		rec, err := registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{LastCache: 1}}})
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	}
}
