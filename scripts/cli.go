package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/brotherlogic/goserver/utils"

	pbgd "github.com/brotherlogic/godiscogs"
	pbrc "github.com/brotherlogic/recordcollection/proto"
	ro "github.com/brotherlogic/recordsorganiser/sales"

	//Needed to pull in gzip encoding init
	_ "google.golang.org/grpc/encoding/gzip"
)

func main() {
	ctx, cancel := utils.ManualContext("recordcollectioncli-"+os.Args[1], "recordcollection", time.Minute, true)
	defer cancel()

	conn, err := utils.LFDialServer(ctx, "recordcollection")
	if err != nil {
		log.Fatalf("Cannot reach rc: %v", err)
	}
	defer conn.Close()

	registry := pbrc.NewRecordCollectionServiceClient(conn)

	switch os.Args[1] {
	case "fix":
		ctx, cancel := utils.ManualContext("recordcollectioncli-"+os.Args[1], "recordcollection", time.Hour, true)
		defer cancel()

		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_All{true}})
		if err != nil {
			log.Fatalf("Bad query: %v", err)
		}

		fmt.Printf("Processing %v records\n", len(ids.GetInstanceIds()))
		for _, id := range ids.GetInstanceIds() {
			r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				log.Fatalf("Bad pull: %v", err)
			}

			if r.GetRecord().GetRelease().GetFolderId() == 242017 {
				if time.Now().Sub(time.Unix(r.GetRecord().GetMetadata().GetLastUpdateTime(), 0)) > time.Hour*24 {
					_, err := registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Reason: "reup", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: id}}})
					if err != nil {
						log.Fatalf("Bad Update: %v", err)
					}

					fmt.Printf("Updated %v\n", r.GetRecord().GetRelease().GetTitle())
					time.Sleep(time.Second * 5)
				}
			}
		}

	case "retrospective":
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_UpdateTime{0}})
		if err != nil {
			log.Fatalf("Bad query: %v", err)
		}

		fmt.Printf("Processing %v records\n", len(ids.GetInstanceIds()))
		for _, id := range ids.GetInstanceIds() {
			r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				log.Fatalf("Error: %v", err)
			}

			t := time.Unix(r.GetRecord().GetMetadata().GetDateAdded(), 0)
			if t.Year() == time.Now().Year()-1 {
				cat := r.GetRecord().GetMetadata().GetCategory()
				if cat != pbrc.ReleaseMetadata_PARENTS &&
					cat != pbrc.ReleaseMetadata_GOOGLE_PLAY &&
					cat != pbrc.ReleaseMetadata_SOLD_ARCHIVE {
					fmt.Printf("%v - %v (%v)\n", t, r.GetRecord().GetRelease().GetTitle(), r.GetRecord().GetMetadata().GetCategory())
				}
			}
		}
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
	case "sale_order":
		meFlags := flag.NewFlagSet("ME", flag.ExitOnError)
		var folder = meFlags.Int("folder", -1, "Id of the record to add")

		if err := meFlags.Parse(os.Args[2:]); err == nil {
			ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_FolderId{int32(*folder)}})

			if err != nil {
				log.Fatalf("Error query: %v", err)
			}
			records := []*pbrc.Record{}
			for _, id := range ids.GetInstanceIds() {
				r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
				if err != nil {
					log.Fatalf("Error getting record: %v", err)
				}
				records = append(records, r.GetRecord())
			}

			sort.Sort(ro.BySaleOrder(records))
			for i := 0; i < 50; i++ {
				fmt.Printf("%v. [%v]: %v - %v\n", i, records[i].GetRelease().GetInstanceId(), records[i].GetRelease().GetArtists()[0].GetName(), records[i].GetRelease().GetTitle())
			}
		}

	case "folder_state":
		meFlags := flag.NewFlagSet("ME", flag.ExitOnError)
		var folder = meFlags.Int("folder", -1, "Id of the record to add")

		if err := meFlags.Parse(os.Args[2:]); err == nil {
			ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_FolderId{int32(*folder)}})
			if err != nil {
				log.Fatalf("Error query: %v", err)
			}
			counts := make(map[pbrc.ReleaseMetadata_Category]int)
			for _, id := range ids.GetInstanceIds() {
				r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
				if err != nil {
					log.Fatalf("Error getting record: %v", err)
				}
				counts[r.GetRecord().GetMetadata().GetCategory()]++
				if r.GetRecord().GetMetadata().GetCategory() == pbrc.ReleaseMetadata_PRE_DISTINGUISHED {
					fmt.Printf("%v - %v\n", r.GetRecord().GetRelease().GetArtists()[0].GetName(), r.GetRecord().GetRelease().GetTitle())
				}
			}

			for cat, count := range counts {
				fmt.Printf("%v - %v\n", cat, count)
			}
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
					fmt.Printf("Error[%v]: %v\n", err)
				}
				fmt.Printf("Updated %v\n", u.GetUpdated().GetRelease().GetId())
			}
		} else {
			fmt.Printf("Error: %v", err)
		}
	case "find_purchased":
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_Category{pbrc.ReleaseMetadata_PURCHASED}})

		if err == nil {
			for _, id := range ids.GetInstanceIds() {
				r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
				if err != nil {
					log.Fatalf("Error: %v\n", err)
				}
				fmt.Printf("[%v] %v - %v\n", id, r.GetRecord().GetRelease().GetArtists()[0].GetName(), r.GetRecord().GetRelease().GetTitle())
			}
		}
	case "pre_high_school":
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_Category{pbrc.ReleaseMetadata_PRE_HIGH_SCHOOL}})

		if err == nil {
			for _, id := range ids.GetInstanceIds() {
				r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
				if err != nil {
					log.Fatalf("Error: %v\n", err)
				}
				fmt.Printf("%v - %v\n", r.GetRecord().GetRelease().GetArtists()[0].GetName(), r.GetRecord().GetRelease().GetTitle())
			}
		} else {
			fmt.Printf("Error: %v", err)
		}

	case "hard_reset_sale_price":
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_Category{pbrc.ReleaseMetadata_LISTED_TO_SELL}})

		if err == nil {
			fmt.Printf("Found %v records\n", len(ids.GetInstanceIds()))
			for _, id := range ids.GetInstanceIds() {
				r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
				if err != nil {
					log.Fatalf("Error: %v\n", err)
				}
				if r.GetRecord().GetMetadata().SalePrice <= 1 {
					r.GetRecord().GetMetadata().NewSalePrice = r.GetRecord().GetMetadata().CurrentSalePrice
					u, err := registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: r.GetRecord().GetRelease().GetInstanceId()}, Metadata: &pbrc.ReleaseMetadata{NewSalePrice: r.GetRecord().GetMetadata().GetCurrentSalePrice()}}, Reason: "reset_sale_price"})
					if err != nil {
						fmt.Printf("Error[%v]: %v\n", err)
					}
					fmt.Printf("Updated %v\n", u.GetUpdated().GetRelease().GetArtists()[0].GetName()+" - "+u.GetUpdated().GetRelease().GetTitle())

				}
			}
		} else {
			fmt.Printf("Error: %v", err)
		}
	case "high_school":
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_Category{pbrc.ReleaseMetadata_HIGH_SCHOOL}})

		if err == nil {
			for _, id := range ids.GetInstanceIds() {
				r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
				if err != nil {
					log.Fatalf("Error: %v\n", err)
				}
				fmt.Printf("%v - %v\n", r.GetRecord().GetRelease().GetArtists()[0].GetName(), r.GetRecord().GetRelease().GetTitle())
			}
		} else {
			fmt.Printf("Error: %v", err)
		}

	case "stuck_sold":
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_Category{pbrc.ReleaseMetadata_SOLD}})

		if err == nil {
			for _, id := range ids.GetInstanceIds() {
				r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
				if err != nil {
					log.Fatalf("Error: %v\n", err)
				}
				if r.GetRecord().GetMetadata().GetSaleId() != 0 {
					fmt.Printf("%v - %v [%v]\n", r.GetRecord().GetMetadata().GetSaleId(), r.GetRecord().GetRelease().GetTitle(), r.GetRecord().GetRelease().GetInstanceId())
				}
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
				if r.GetRecord().GetMetadata().GetSalePrice() < 500 {
					fmt.Printf("%v - %v (%v) [%v]\n", r.GetRecord().GetMetadata().GetCurrentSalePrice()-r.GetRecord().GetMetadata().SalePrice, r.GetRecord().GetRelease().GetTitle(), r.GetRecord().GetMetadata().GetSalePrice(), r.GetRecord().GetRelease().GetInstanceId())
				}
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

	case "oldest_record":
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_UpdateTime{0}})
		if err != nil {
			log.Fatalf("Bad query: %v", err)
		}

		fmt.Printf("Processing %v records\n", len(ids.GetInstanceIds()))
		lowest := time.Now().Unix()
		var rec *pbrc.Record
		for _, id := range ids.GetInstanceIds() {
			r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				log.Fatalf("Error: %v", err)
			}

			if r.GetRecord().GetMetadata().GetDateAdded() < lowest {
				if strings.HasPrefix(fmt.Sprintf("%v", r.GetRecord().GetMetadata().GetCategory()), "PRE") {
					lowest = r.GetRecord().GetMetadata().GetDateAdded()
					rec = r.GetRecord()
				}
			}

		}

		fmt.Printf("%v, %v\n", time.Unix(lowest, 0), rec)
	case "needs_stock":
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_UpdateTime{0}})
		if err != nil {
			log.Fatalf("Bad query: %v", err)
		}

		fmt.Printf("Processing %v records\n", len(ids.GetInstanceIds()))
		for _, id := range ids.GetInstanceIds() {
			r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				log.Fatalf("Error: %v", err)
			}
			if time.Now().Sub(time.Unix(r.GetRecord().GetMetadata().GetLastStockCheck(), 0)) > time.Hour*24*30 {
				_, err := registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Reason: "stock push", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: id}, Metadata: &pbrc.ReleaseMetadata{LastStockCheck: time.Now().Unix()}}})
				if err != nil {
					log.Fatalf("Cannot update: %v\n", err)
				} else {
					fmt.Printf("[%v] is too old (%v)\n", r.GetRecord().GetRelease().GetInstanceId(), r.GetRecord().GetRelease().GetTitle())
				}
				time.Sleep(time.Second)
			}
		}
	case "play_time":
		folder, err := strconv.Atoi(os.Args[2])
		if err != nil {
			log.Fatalf("Hmm: %v", err)
		}
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_FolderId{int32(folder)}})
		if err != nil {
			log.Fatalf("Bad query: %v", err)
		}

		fmt.Printf("Processing %v records\n", len(ids.GetInstanceIds()))
		min := time.Now().Unix()
		var rec *pbrc.Record
		for _, id := range ids.GetInstanceIds() {
			r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				log.Fatalf("Error: %v", err)
			}

			if r.GetRecord().GetRelease().GetFolderId() == int32(folder) {
				if r.GetRecord().GetMetadata().GetLastListenTime() < min {
					if r.GetRecord().GetMetadata().GetLastListenTime() == 0 {
						fmt.Printf("%v - %v\n", r.GetRecord().GetRelease().GetArtists()[0].GetName(), r.GetRecord().GetRelease().GetTitle())
						up := &pbrc.UpdateRecordRequest{Reason: "script-unlisten", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: r.GetRecord().GetRelease().GetInstanceId()}, Metadata: &pbrc.ReleaseMetadata{Category: pbrc.ReleaseMetadata_UNLISTENED}}}
						_, err := registry.UpdateRecord(ctx, up)
						if err != nil {
							fmt.Printf("Error: %v\n", err)
						}
					} else {
						min = r.GetRecord().GetMetadata().GetLastListenTime()
						rec = r.GetRecord()
					}
				}
			}
		}

		fmt.Printf("%v -> %v - %v\n", time.Unix(min, 0), rec.GetRelease().GetArtists()[0].GetName(), rec.GetRelease().GetTitle())
	case "missing":
		folder, err := strconv.Atoi(os.Args[2])
		if err != nil {
			log.Fatalf("Hmm: %v", err)
		}
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_FolderId{int32(folder)}})
		if err != nil {
			log.Fatalf("Bad query: %v", err)
		}

		fmt.Printf("Processing %v records\n", len(ids.GetInstanceIds()))
		for _, id := range ids.GetInstanceIds() {
			r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				log.Fatalf("Error: %v", err)
			}

			if r.GetRecord().GetMetadata().GetSaleId() == 0 {
				if len(r.GetRecord().GetRelease().GetArtists()) > 0 {
					fmt.Printf("[%v] %v - %v\n", r.GetRecord().GetRelease().GetInstanceId(), r.GetRecord().GetRelease().GetArtists()[0].GetName(), r.GetRecord().GetRelease().GetTitle())
				} else {
					fmt.Printf("[%v] %v\n", r.GetRecord().GetRelease().GetInstanceId(), r.GetRecord().GetRelease().GetTitle())
				}
			}
		}
	}
}
