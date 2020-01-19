package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/brotherlogic/goserver/utils"
	"google.golang.org/grpc"

	pbrc "github.com/brotherlogic/recordcollection/proto"

	//Needed to pull in gzip encoding init
	_ "google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/resolver"
)

func init() {
	resolver.Register(&utils.DiscoveryClientResolverBuilder{})
}

func main() {

	conn, err := grpc.Dial("discovery:///recordcollection", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Cannot reach rc: %v", err)
	}
	defer conn.Close()

	registry := pbrc.NewRecordCollectionServiceClient(conn)

	ctx, cancel := utils.ManualContext("recordcollectioncli-"+os.Args[1], "recordcollection", time.Minute*5)
	defer cancel()

	switch os.Args[1] {
	case "most_expensive":
		meFlags := flag.NewFlagSet("ME", flag.ExitOnError)
		var folder = meFlags.Int("folder", -1, "Id of the record to add")

		if err := meFlags.Parse(os.Args[2:]); err == nil {
			ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_FolderId{int32(*folder)}})
			if err != nil {
				log.Fatalf("Error query: %v", err)
			}
			highest := int32(0)
			var rec *pbrc.Record
			for _, id := range ids.GetInstanceIds() {
				r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
				if err != nil {
					log.Fatalf("Error getting record: %v", err)
				}
				if r.GetRecord().GetMetadata().GetCurrentSalePrice() > highest {
					highest = r.GetRecord().GetMetadata().GetCurrentSalePrice()
					rec = r.GetRecord()
				}
			}

			fmt.Printf("Highest [%v] = %v\n", *folder, rec.GetRelease().GetTitle())
		}
	case "reset_sale_price":
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_Category{pbrc.ReleaseMetadata_LISTED_TO_SELL}})

		if err == nil {
			for _, id := range ids.GetInstanceIds() {
				fmt.Printf("Getting record: %v\n", id)
				r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
				if err != nil {
					log.Fatalf("Error: %v\n", err)
				}
				r.GetRecord().GetMetadata().NewSalePrice = r.GetRecord().GetMetadata().CurrentSalePrice
				u, err := registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Update: r.GetRecord()})
				if err != nil {
					log.Fatalf("Error: %v", err)
				}
				fmt.Println()
				fmt.Printf("Release: %v\n", u.GetUpdated().GetRelease())
				fmt.Printf("Metadata: %v\n", u.GetUpdated().GetMetadata())
			}
		} else {
			fmt.Printf("Error: %v", err)
		}
	case "wayward_sale":
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_Category{pbrc.ReleaseMetadata_LISTED_TO_SELL}})

		if err == nil {
			for _, id := range ids.GetInstanceIds() {
				r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
				if err != nil {
					log.Fatalf("Error: %v\n", err)
				}
				fmt.Printf("%v - %v [%v]\n", r.GetRecord().GetMetadata().GetCurrentSalePrice()-r.GetRecord().GetMetadata().SalePrice, r.GetRecord().GetRelease().GetTitle(), r.GetRecord().GetRelease().GetInstanceId())
			}
		} else {
			fmt.Printf("Error: %v", err)
		}

	case "find_missing_costs":
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_UpdateTime{0}})
		if err != nil {
			log.Fatalf("Bad query: %v", err)
		}

		fmt.Printf("Processing %v records\n", len(ids.GetInstanceIds()))
		count := 0
		missingCost := 0
		for _, id := range ids.GetInstanceIds() {
			r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				log.Fatalf("Error: %v", err)
			}

			if time.Unix(r.GetRecord().GetMetadata().GetDateAdded(), 0).Year() == time.Now().Year() {
				count++
				if r.GetRecord().GetMetadata().GetCost() == 0 {
					missingCost++
					fmt.Printf("%v -> %v [%v]\n", r.GetRecord().GetRelease().GetInstanceId(), r.GetRecord().GetRelease().GetTitle(), r.GetRecord().GetMetadata().GetCategory())
					if r.GetRecord().GetMetadata().GetCategory() == pbrc.ReleaseMetadata_GOOGLE_PLAY ||
						r.GetRecord().GetMetadata().GetCategory() == pbrc.ReleaseMetadata_PARENTS {
						r.GetRecord().GetMetadata().Cost = 1
						_, err := registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Update: r.GetRecord()})
						if err != nil {
							log.Fatalf("Error on update: %v", err)
						}
					}
				}

			}

		}

		fmt.Printf("For %v found %v records (%v have missing costs)\n", time.Now().Year(), count, missingCost)

	}
}
