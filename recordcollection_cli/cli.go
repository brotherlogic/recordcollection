package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/brotherlogic/goserver/utils"
	"google.golang.org/grpc"

	pbgd "github.com/brotherlogic/godiscogs"
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

	ctx, cancel := utils.ManualContext("recordcollectioncli-"+os.Args[1], "recordcollection", time.Hour*5)
	defer cancel()

	switch os.Args[1] {
	case "stock":
		i, _ := strconv.Atoi(os.Args[2])
		srec, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: int32(i)})

		if err != nil {
			log.Fatalf("Error getting record: %v", err)
		}

		up := &pbrc.UpdateRecordRequest{Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: srec.GetRecord().GetRelease().InstanceId}, Metadata: &pbrc.ReleaseMetadata{LastStockCheck: time.Now().Unix()}}}
		rec, err := registry.UpdateRecord(ctx, up)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)

	case "wants":
		fmt.Printf("WANTS\n")
		rec, err := registry.GetWants(ctx, &pbrc.GetWantsRequest{})
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Found %v wants\n", len(rec.GetWants()))
		for i, want := range rec.GetWants() {
			fmt.Printf("%v. %v\n", i, want)
		}
	case "unlistened":
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_Category{pbrc.ReleaseMetadata_PRE_FRESHMAN}})
		if err != nil {
			fmt.Printf("Error %v\n", err)
		}
		for i, id := range ids.GetInstanceIds() {
			r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			}
			fmt.Printf("%v. %v\n", i, r.GetRecord().GetRelease().GetTitle())
		}
	case "get":
		i, _ := strconv.Atoi(os.Args[2])
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_ReleaseId{int32(i)}})

		if err == nil {
			for _, id := range ids.GetInstanceIds() {
				r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
				if err != nil {
					fmt.Printf("Error: %v\n", err)
				}
				fmt.Println()
				fmt.Printf("Release: %v\n", r.GetRecord().GetRelease())
				fmt.Printf("Metadata: %v\n", r.GetRecord().GetMetadata())
			}
		} else {
			fmt.Printf("Error: %v", err)
		}
	case "reset_sale_price":
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_Category{pbrc.ReleaseMetadata_LISTED_TO_SELL}})

		if err == nil {
			for _, id := range ids.GetInstanceIds() {
				r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
				if err != nil {
					fmt.Printf("Error: %v\n", err)
				}
				r.GetRecord().GetMetadata().SalePrice = r.GetRecord().GetMetadata().CurrentSalePrice
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

	case "sget":
		i, _ := strconv.Atoi(os.Args[2])
		srec, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: int32(i)})

		if err == nil {
			fmt.Printf("Release: %v\n", srec.GetRecord().GetRelease())
			fmt.Printf("Metadata: %v\n", srec.GetRecord().GetMetadata())
		} else {
			fmt.Printf("Error: %v", err)
		}

	case "force":
		i, _ := strconv.Atoi(os.Args[2])
		rec, err := registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{LastCache: 1, LastSyncTime: 1}}})
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	case "spfolder":
		i, _ := strconv.Atoi(os.Args[2])
		f, _ := strconv.Atoi(os.Args[3])
		rec, err := registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{GoalFolder: int32(f)}}})
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	case "rfolder":
		i, _ := strconv.Atoi(os.Args[2])
		f, _ := strconv.Atoi(os.Args[3])
		rec, err := registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{GoalFolder: int32(f), SetRating: -1}}})
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)

	case "force_sale":
		i, _ := strconv.Atoi(os.Args[2])
		rec, err := registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{SaleDirty: true}}})
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	case "reset_sale":
		i, _ := strconv.Atoi(os.Args[2])
		up := &pbrc.UpdateRecordRequest{Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{Category: pbrc.ReleaseMetadata_PREPARE_TO_SELL, SaleId: -1}}}
		rec, err := registry.UpdateRecord(ctx, up)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	case "direct_sale":
		i, _ := strconv.Atoi(os.Args[2])
		up := &pbrc.UpdateRecordRequest{Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{Category: pbrc.ReleaseMetadata_LISTED_TO_SELL}}}
		rec, err := registry.UpdateRecord(ctx, up)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	case "sell":
		i, _ := strconv.Atoi(os.Args[2])
		up := &pbrc.UpdateRecordRequest{Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{SetRating: -1, MoveFolder: 673768, Category: pbrc.ReleaseMetadata_STAGED_TO_SELL}}}
		rec, err := registry.UpdateRecord(ctx, up)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	case "price":
		i, _ := strconv.Atoi(os.Args[2])
		p, _ := strconv.Atoi(os.Args[3])
		up := &pbrc.UpdateRecordRequest{Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{SalePrice: int32(p)}}}
		rec, err := registry.UpdateRecord(ctx, up)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	case "sold":
		i, _ := strconv.Atoi(os.Args[2])
		up := &pbrc.UpdateRecordRequest{Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{Category: pbrc.ReleaseMetadata_PREPARE_TO_SELL}}}
		rec, err := registry.UpdateRecord(ctx, up)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	case "fsold":
		i, _ := strconv.Atoi(os.Args[2])
		up := &pbrc.UpdateRecordRequest{NoSell: true, Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{Category: pbrc.ReleaseMetadata_SOLD}}}
		rec, err := registry.UpdateRecord(ctx, up)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)

	case "assess":
		i, _ := strconv.Atoi(os.Args[2])
		up := &pbrc.UpdateRecordRequest{Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{Category: pbrc.ReleaseMetadata_ASSESS}}}
		rec, err := registry.UpdateRecord(ctx, up)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	case "delete":
		i, _ := strconv.Atoi(os.Args[2])
		up := &pbrc.DeleteRecordRequest{InstanceId: int32(i)}
		rec, err := registry.DeleteRecord(ctx, up)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	case "addsale":
		i, _ := strconv.Atoi(os.Args[2])
		i2, _ := strconv.Atoi(os.Args[3])
		up := &pbrc.UpdateRecordRequest{Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{SaleId: int32(i2)}}}
		rec, err := registry.UpdateRecord(ctx, up)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	case "saleprice":
		i, _ := strconv.Atoi(os.Args[2])
		i2, _ := strconv.Atoi(os.Args[3])
		up := &pbrc.UpdateRecordRequest{Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{SalePrice: int32(i2)}}}
		rec, err := registry.UpdateRecord(ctx, up)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	case "soldoffline":
		i, _ := strconv.Atoi(os.Args[2])
		up := &pbrc.UpdateRecordRequest{Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{Category: pbrc.ReleaseMetadata_SOLD_OFFLINE}}}
		rec, err := registry.UpdateRecord(ctx, up)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	case "parents":
		i, _ := strconv.Atoi(os.Args[2])
		up := &pbrc.UpdateRecordRequest{Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{Category: pbrc.ReleaseMetadata_PARENTS}}}
		rec, err := registry.UpdateRecord(ctx, up)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	case "cost":
		i, _ := strconv.Atoi(os.Args[2])
		c, _ := strconv.Atoi(os.Args[3])
		up := &pbrc.UpdateRecordRequest{Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{Cost: int32(c)}}}
		rec, err := registry.UpdateRecord(ctx, up)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	case "unlisten":
		i, _ := strconv.Atoi(os.Args[2])
		up := &pbrc.UpdateRecordRequest{Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{Category: pbrc.ReleaseMetadata_UNLISTENED}}}
		rec, err := registry.UpdateRecord(ctx, up)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	case "keep":
		i, _ := strconv.Atoi(os.Args[2])
		up := &pbrc.UpdateRecordRequest{Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{Keep: pbrc.ReleaseMetadata_KEEPER}}}
		rec, err := registry.UpdateRecord(ctx, up)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	case "reset_cd":
		i, _ := strconv.Atoi(os.Args[2])
		up := &pbrc.UpdateRecordRequest{Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{}}}
		rec, err := registry.UpdateRecord(ctx, up)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	}
}
