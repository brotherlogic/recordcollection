package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/brotherlogic/goserver/utils"
	"github.com/golang/protobuf/proto"

	pbgd "github.com/brotherlogic/godiscogs"
	pbrc "github.com/brotherlogic/recordcollection/proto"

	//Needed to pull in gzip encoding init
	_ "google.golang.org/grpc/encoding/gzip"
)

func main() {
	t := time.Now()
	ctx, cancel := utils.ManualContext("recordcollectioncli-"+os.Args[1], "recordcollection", time.Second*10, true)
	defer cancel()

	conn, err := utils.LFDialServer(ctx, "recordcollection")
	fmt.Printf("Dialled in %v\n", time.Now().Sub(t))
	if err != nil {
		log.Fatalf("Cannot reach rc: %v", err)
	}
	defer conn.Close()

	registry := pbrc.NewRecordCollectionServiceClient(conn)

	switch os.Args[1] {
	case "trigger":
		res, err := registry.Trigger(ctx, &pbrc.TriggerRequest{})
		fmt.Printf("%v,%v\n", res, err)
	case "stock":
		i, _ := strconv.Atoi(os.Args[2])
		srec, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: int32(i)})

		if err != nil {
			log.Fatalf("Error getting record: %v", err)
		}

		up := &pbrc.UpdateRecordRequest{Reason: "CLI-stock", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: srec.GetRecord().GetRelease().InstanceId}, Metadata: &pbrc.ReleaseMetadata{LastStockCheck: time.Now().Unix()}}}
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
				fmt.Printf("Release: %v\n", r.GetRecord().GetRelease())
				fmt.Println()
				fmt.Printf("Metadata: %v\n", r.GetRecord().GetMetadata())
			}
		} else {
			fmt.Printf("Error: %v", err)
		}
	case "reset_sale_price":
		i, f := strconv.Atoi(os.Args[2])
		if f != nil {
			log.Fatalf("Hmm %v", f)
		}
		r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: int32(i)})
		if err != nil {
			log.Fatalf("Error: %v -> %v,%v,%v\n", err, int32(i), i, os.Args[2])
		}
		r.GetRecord().GetMetadata().SalePrice = r.GetRecord().GetMetadata().CurrentSalePrice
		u, err := registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Reason: "recordcollection-cli_reset-sale-price", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{SalePrice: r.GetRecord().GetMetadata().CurrentSalePrice, SaleDirty: true}}})
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Println()
		fmt.Printf("Release: %v\n", u.GetUpdated().GetRelease())
		fmt.Printf("Metadata: %v\n", u.GetUpdated().GetMetadata())

	case "sget":
		i, _ := strconv.Atoi(os.Args[2])
		force := int32(0)
		if len(os.Args) > 3 {
			i2, _ := strconv.Atoi(os.Args[3])
			force = int32(i2)
		}

		srec, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: int32(i), Force: force})

		if err == nil {
			fmt.Printf("Release: %v\n", srec.GetRecord().GetRelease())
			fmt.Printf("Metadata: %v, %v\n", srec.GetRecord().GetMetadata(), srec.GetRecord().GetMetadata().GetMatch())
		} else {
			fmt.Printf("Error: %v", err)
		}

	case "force":
		i, _ := strconv.Atoi(os.Args[2])
		rec, err := registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Reason: "forcing sync from cli", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{LastCache: 1, LastSyncTime: 1}}})
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	case "expire":
		i, _ := strconv.Atoi(os.Args[2])
		rec, err := registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{ExpireSale: true}}})
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	case "spfolder":
		i, _ := strconv.Atoi(os.Args[2])
		f, _ := strconv.Atoi(os.Args[3])
		rec, err := registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Reason: "CLI-spfolder", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{GoalFolder: int32(f)}}})
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
		up := &pbrc.UpdateRecordRequest{Reason: "CLI-reset_sale", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{Category: pbrc.ReleaseMetadata_PREPARE_TO_SELL, SaleId: -1, SaleState: pbgd.SaleState_EXPIRED}}}
		rec, err := registry.UpdateRecord(ctx, up)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	case "reset_sale_state":
		i, _ := strconv.Atoi(os.Args[2])
		up := &pbrc.UpdateRecordRequest{Reason: "CLI-reset_sale", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{SaleState: pbgd.SaleState_EXPIRED, Category: pbrc.ReleaseMetadata_LISTED_TO_SELL}}}
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
	case "gsell":
		i, _ := strconv.Atoi(os.Args[2])
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_ReleaseId{int32(i)}})

		if err == nil && len(ids.GetInstanceIds()) == 1 {

			up := &pbrc.UpdateRecordRequest{Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(ids.GetInstanceIds()[0])}, Metadata: &pbrc.ReleaseMetadata{SetRating: -1, GoalFolder: 267116, MoveFolder: 673768, Category: pbrc.ReleaseMetadata_STAGED_TO_SELL}}}
			rec, err := registry.UpdateRecord(ctx, up)
			if err != nil {
				log.Fatalf("Error: %v", err)
			}
			fmt.Printf("Updated: %v", rec)
		}

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
		up := &pbrc.UpdateRecordRequest{Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{NewSalePrice: int32(p)}}}
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
		up := &pbrc.UpdateRecordRequest{Reason: "cli-addsale", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{SaleId: int32(i2)}}}
		rec, err := registry.UpdateRecord(ctx, up)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	case "saleprice":
		i, _ := strconv.Atoi(os.Args[2])
		i2, _ := strconv.Atoi(os.Args[3])
		up := &pbrc.UpdateRecordRequest{Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{NewSalePrice: int32(i2)}}}
		rec, err := registry.UpdateRecord(ctx, up)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	case "soldoffline":
		i, _ := strconv.Atoi(os.Args[2])
		up := &pbrc.UpdateRecordRequest{Reason: "cli-soldoffline", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{Category: pbrc.ReleaseMetadata_SOLD_OFFLINE}}}
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
		up := &pbrc.UpdateRecordRequest{Reason: "cli-cost", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{Cost: int32(c)}}}
		rec, err := registry.UpdateRecord(ctx, up)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	case "unlisten":
		i, _ := strconv.Atoi(os.Args[2])
		up := &pbrc.UpdateRecordRequest{Reason: "cli-unlisten", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{Category: pbrc.ReleaseMetadata_UNLISTENED}}}
		rec, err := registry.UpdateRecord(ctx, up)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	case "keep":
		i, _ := strconv.Atoi(os.Args[2])
		up := &pbrc.UpdateRecordRequest{Reason: "rccli-keep", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{Keep: pbrc.ReleaseMetadata_KEEPER}}}
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
	case "save":
		i, _ := strconv.Atoi(os.Args[2])
		rec, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: int32(i)})
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		data, _ := proto.Marshal(rec.GetRecord())
		ioutil.WriteFile(fmt.Sprintf("%v.data", rec.GetRecord().GetRelease().Id), data, 0644)
	}
}
