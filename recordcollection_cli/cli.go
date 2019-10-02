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
	"google.golang.org/grpc"

	pbgd "github.com/brotherlogic/godiscogs"
	pbrc "github.com/brotherlogic/recordcollection/proto"

	//Needed to pull in gzip encoding init
	_ "google.golang.org/grpc/encoding/gzip"
)

func main() {
	host, port, err := utils.Resolve("recordcollection", "recordcollection-cli")

	if err != nil {
		log.Fatalf("Unable to locate recordcollection server")
	}

	conn, _ := grpc.Dial(host+":"+strconv.Itoa(int(port)), grpc.WithInsecure())
	defer conn.Close()

	registry := pbrc.NewRecordCollectionServiceClient(conn)

	ctx, cancel := utils.ManualContext("recordcollectioncli-"+os.Args[1], "recordcollection", time.Second*5)
	defer cancel()

	switch os.Args[1] {
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
	case "getsales":
		rec, err := registry.GetRecords(ctx, &pbrc.GetRecordsRequest{Filter: &pbrc.Record{Release: &pbgd.Release{}, Metadata: &pbrc.ReleaseMetadata{}}}, grpc.MaxCallRecvMsgSize(1024*1024*1024))
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("%v records\n", len(rec.GetRecords()))
		for i, rec := range rec.GetRecords() {
			if rec.GetMetadata().SalePrice <= 500 && rec.GetMetadata().SalePrice > 0 && rec.GetMetadata().Category == pbrc.ReleaseMetadata_LISTED_TO_SELL {
				fmt.Printf("%v. %v - %v\n", i, rec.GetRelease().Title, rec.GetRelease().GetFormats())
			}
		}

	case "get":
		i, _ := strconv.Atoi(os.Args[2])
		rec, err := registry.GetRecords(ctx, &pbrc.GetRecordsRequest{Caller: "cli-get", Force: true, Filter: &pbrc.Record{Release: &pbgd.Release{Id: int32(i)}}})

		if err == nil {
			fmt.Printf("Time Taken: %v\n", rec.InternalProcessingTime)
			for _, r := range rec.GetRecords() {
				fmt.Println()
				fmt.Printf("Release: %v\n", r.GetRelease())
				fmt.Printf("Metadata: %v\n", r.GetMetadata())
			}
		} else {
			fmt.Printf("Error: %v", err)
		}
	case "adjust_sales":
		rec, err := registry.GetRecords(ctx, &pbrc.GetRecordsRequest{Force: true, Filter: &pbrc.Record{Release: &pbgd.Release{}}}, grpc.MaxCallRecvMsgSize(1024*1024*1024))

		if err == nil {
			fmt.Printf("Time Taken: %v\n", rec.InternalProcessingTime)
			for _, r := range rec.GetRecords() {
				if r.GetMetadata().Category == pbrc.ReleaseMetadata_LISTED_TO_SELL || r.GetMetadata().Category == pbrc.ReleaseMetadata_STALE_SALE {
					r.GetMetadata().SalePrice = r.GetMetadata().CurrentSalePrice
					r.GetMetadata().SaleDirty = true
					up, err := registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Update: r})
					if err != nil {
						fmt.Printf("Error: %v", err)
					} else {
						fmt.Printf("Update %v", up)
					}
				}
			}
		} else {
			fmt.Printf("Error: %v", err)
		}

	case "sget":
		i, _ := strconv.Atoi(os.Args[2])
		rec, err := registry.GetRecords(ctx, &pbrc.GetRecordsRequest{Caller: "sget", Force: true, Filter: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}}})

		if err == nil {
			for _, r := range rec.GetRecords() {
				fmt.Printf("Release: %v\n", r.GetRelease())
				fmt.Printf("Metadata: %v\n", r.GetMetadata())
			}
		} else {
			fmt.Printf("Error: %v", err)
		}

		srec, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: int32(i)})

		if err == nil {
			fmt.Printf("Release: %v\n", srec.GetRecord().GetRelease())
			fmt.Printf("Metadata: %v\n", srec.GetRecord().GetMetadata())
		} else {
			fmt.Printf("Error: %v", err)
		}

	case "pget":
		rec, err := registry.GetRecords(ctx, &pbrc.GetRecordsRequest{Force: true, Filter: &pbrc.Record{Release: &pbgd.Release{FolderId: int32(1362206)}}})

		if err == nil {
			if len(rec.GetRecords()) > 0 {
				for _, r := range rec.GetRecords() {
					if r.GetMetadata().Purgatory == pbrc.Purgatory_NEEDS_STOCK_CHECK {
						fmt.Printf("Release: %v\n", r.GetRelease())
						fmt.Printf("Metadata: %v\n", r.GetMetadata())
						return
					}
				}

				r := rec.GetRecords()[0]
				fmt.Printf("Release: %v\n", r.GetRelease())
				fmt.Printf("Metadata: %v\n", r.GetMetadata())
			}
		} else {
			fmt.Printf("Req Error: %v", err)
		}
	case "all":
		t1 := time.Now()
		rec, err := registry.GetRecords(ctx, &pbrc.GetRecordsRequest{Filter: &pbrc.Record{}}, grpc.UseCompressor("gzip"))
		t2 := time.Now()

		if err == nil {
			fmt.Printf("%v records in the collection\n", len(rec.GetRecords()))
			fmt.Printf("Pull took %v (%v) for %v bytes\n", t2.Sub(t1), rec.InternalProcessingTime, proto.Size(rec))
		}
	case "allstrip":
		t1 := time.Now()
		rec, err := registry.GetRecords(ctx, &pbrc.GetRecordsRequest{MoveStrip: true, Filter: &pbrc.Record{}})
		t2 := time.Now()

		if err == nil {
			fmt.Printf("%v records in the collection\n", len(rec.GetRecords()))
			fmt.Printf("Pull took %v (%v) for %v bytes\n", t2.Sub(t1), rec.InternalProcessingTime, proto.Size(rec))
			size := 0
			sizePb := &pbrc.Record{}
			for _, r := range rec.GetRecords() {
				if proto.Size(r) > size {
					size = proto.Size(r)
					sizePb = r
				}
			}
			fmt.Printf("Largest: %v, %v\n", sizePb.GetRelease().Title, size)
		}

	case "force":
		i, _ := strconv.Atoi(os.Args[2])
		rec, err := registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{LastCache: 1, LastSyncTime: 1}}})
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	case "folder":
		i, _ := strconv.Atoi(os.Args[2])
		f, _ := strconv.Atoi(os.Args[3])
		rec, err := registry.GetRecords(ctx, &pbrc.GetRecordsRequest{Filter: &pbrc.Record{Release: &pbgd.Release{Id: int32(i)}}})
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		if len(rec.GetRecords()) == 1 {
			rec, err := registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(rec.GetRecords()[0].GetRelease().InstanceId)}, Metadata: &pbrc.ReleaseMetadata{GoalFolder: int32(f), MoveFolder: int32(-1)}}})
			if err != nil {
				log.Fatalf("Error: %v", err)
			}
			fmt.Printf("Updated: %v", rec)
		} else {
			fmt.Printf("Not Found!")
		}
	case "onetime":
		rec, err := registry.GetRecords(ctx, &pbrc.GetRecordsRequest{Filter: &pbrc.Record{Metadata: &pbrc.ReleaseMetadata{Dirty: false, Category: pbrc.ReleaseMetadata_STAGED_TO_SELL}, Release: &pbgd.Release{}}})
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		time.Sleep(time.Second * 10)

		for _, r := range rec.GetRecords() {
			_, err := registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(r.GetRelease().InstanceId)}, Metadata: &pbrc.ReleaseMetadata{Category: pbrc.ReleaseMetadata_PREPARE_TO_SELL}}})
			if err != nil {
				log.Fatalf("Error: %v", err)
			}
			fmt.Printf("Run on %v\n", r.GetRelease().Title)
		}
		fmt.Printf("Updated: %v\n", len(rec.GetRecords()))
	case "spfolder":
		i, _ := strconv.Atoi(os.Args[2])
		f, _ := strconv.Atoi(os.Args[3])
		rec, err := registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{GoalFolder: int32(f)}}})
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
	case "reset":
		i, _ := strconv.Atoi(os.Args[2])
		recs, err := registry.GetRecords(ctx, &pbrc.GetRecordsRequest{Filter: &pbrc.Record{Release: &pbgd.Release{Id: int32(i)}}})
		if err != nil {
			log.Fatalf("Error: %v", err)
		}

		up := &pbrc.UpdateRecordRequest{Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(recs.GetRecords()[0].GetRelease().InstanceId)}, Metadata: &pbrc.ReleaseMetadata{SetRating: -1, Category: pbrc.ReleaseMetadata_UNLISTENED}}}
		rec, err := registry.UpdateRecord(ctx, up)
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

	case "hreset":
		i, _ := strconv.Atoi(os.Args[2])
		recs, err := registry.GetRecords(ctx, &pbrc.GetRecordsRequest{Caller: "hreset-cli", Filter: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}}})
		if err != nil {
			log.Fatalf("Error: %v", err)
		}

		up := &pbrc.UpdateRecordRequest{Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(recs.GetRecords()[0].GetRelease().InstanceId), FolderId: 1}, Metadata: &pbrc.ReleaseMetadata{SetRating: -1, SaleId: -1, Category: pbrc.ReleaseMetadata_UNLISTENED}}}
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
	case "stock":
		i, _ := strconv.Atoi(os.Args[2])
		recs, err := registry.GetRecords(ctx, &pbrc.GetRecordsRequest{Caller: "cli-stock", Filter: &pbrc.Record{Release: &pbgd.Release{Id: int32(i)}}})
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		for _, r := range recs.GetRecords() {
			up := &pbrc.UpdateRecordRequest{Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: r.GetRelease().InstanceId}, Metadata: &pbrc.ReleaseMetadata{LastStockCheck: time.Now().Unix()}}}
			rec, err := registry.UpdateRecord(ctx, up)
			if err != nil {
				log.Fatalf("Error: %v", err)
			}
			fmt.Printf("Updated: %v", rec)
		}
	case "stockall":
		recs, err := registry.GetRecords(ctx, &pbrc.GetRecordsRequest{Filter: &pbrc.Record{Release: &pbgd.Release{FolderId: int32(1362206)}}})
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		for _, r := range recs.GetRecords() {
			if r.GetMetadata().Category == pbrc.ReleaseMetadata_ASSESS {
				up := &pbrc.UpdateRecordRequest{Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: r.GetRelease().InstanceId}, Metadata: &pbrc.ReleaseMetadata{LastStockCheck: time.Now().Unix()}}}
				rec, err := registry.UpdateRecord(ctx, up)
				if err != nil {
					log.Fatalf("Error: %v", err)
				}
				fmt.Printf("Updated: %v", rec)
			}
		}
	case "delete":
		i, _ := strconv.Atoi(os.Args[2])
		up := &pbrc.DeleteRecordRequest{InstanceId: int32(i)}
		rec, err := registry.DeleteRecord(ctx, up)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	case "save":
		i, _ := strconv.Atoi(os.Args[2])
		rec, err := registry.GetRecords(ctx, &pbrc.GetRecordsRequest{Caller: "rccli-save", Force: true, Filter: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}}})
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		data, _ := proto.Marshal(rec.GetRecords()[0])
		ioutil.WriteFile(fmt.Sprintf("%v.data", rec.GetRecords()[0].GetRelease().Id), data, 0644)
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
	case "massgoal":
		i, _ := strconv.Atoi(os.Args[2])
		recs, err := registry.GetRecords(ctx, &pbrc.GetRecordsRequest{Filter: &pbrc.Record{Release: &pbgd.Release{FolderId: int32(i)}}})
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		for _, r := range recs.GetRecords() {
			update := &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(r.GetRelease().InstanceId)}, Metadata: &pbrc.ReleaseMetadata{GoalFolder: int32(i)}}
			_, err := registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Update: update})
			if err != nil {
				log.Fatalf("Error: %v", err)
			}
		}
	case "massfix":
		recs, err := registry.GetRecords(ctx, &pbrc.GetRecordsRequest{Filter: &pbrc.Record{Release: &pbgd.Release{FolderId: int32(673768)}}})
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		for _, r := range recs.GetRecords() {
			if r.GetMetadata().Category == pbrc.ReleaseMetadata_PRE_HIGH_SCHOOL && time.Now().Sub(time.Unix(r.GetMetadata().DateAdded, 0)) > time.Hour*24*30*3 {
				fmt.Printf("Update: %v\n", r.GetRelease().Title)
				update := &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(r.GetRelease().InstanceId)}, Metadata: &pbrc.ReleaseMetadata{SetRating: int32(5)}}
				_, err := registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Update: update})
				if err != nil {
					log.Fatalf("Error: %v", err)
				}
			}
		}
	case "need_pricing":
		recs, err := registry.GetRecords(ctx, &pbrc.GetRecordsRequest{Filter: &pbrc.Record{Release: &pbgd.Release{}}}, grpc.MaxCallRecvMsgSize(1024*1024*1024))
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		for _, r := range recs.GetRecords() {
			if time.Now().Sub(time.Unix(r.GetMetadata().DateAdded, 0)) < time.Hour*24*7*12 {
				if r.GetMetadata().Cost == 0 && r.GetMetadata().GoalFolder != 1433217 && r.GetMetadata().GoalFolder != 1727264 {
					fmt.Printf("%v - %v\n", r.GetRelease().Id, r.GetRelease().Title)
				}
			}
		}
	case "need_stock":
		recs, err := registry.GetRecords(ctx, &pbrc.GetRecordsRequest{Filter: &pbrc.Record{Release: &pbgd.Release{}}}, grpc.MaxCallRecvMsgSize(1024*1024*1024))
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		for _, r := range recs.GetRecords() {
			if (r.GetRelease().FolderId == 812802 || r.GetRelease().FolderId == 673768) && time.Now().Sub(time.Unix(r.GetMetadata().LastStockCheck, 0)) > time.Hour*24*265 {
				fmt.Printf("%v %v\n", r.GetRelease().Id, r.GetRelease().Title)
			}
		}
	case "massfix2":
		recs, err := registry.GetRecords(ctx, &pbrc.GetRecordsRequest{Filter: &pbrc.Record{Release: &pbgd.Release{FolderId: int32(812802)}}})
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		for _, r := range recs.GetRecords() {
			if r.GetMetadata().Category == pbrc.ReleaseMetadata_PRE_FRESHMAN && time.Now().Sub(time.Unix(r.GetMetadata().DateAdded, 0)) > time.Hour*24*30*3 {
				fmt.Printf("Update: %v\n", r.GetRelease().Title)
				update := &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(r.GetRelease().InstanceId)}, Metadata: &pbrc.ReleaseMetadata{SetRating: int32(5)}}
				_, err := registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Update: update})
				if err != nil {
					log.Fatalf("Error: %v", err)
				}
			}
		}
	case "staged":
		recs, err := registry.GetRecords(ctx, &pbrc.GetRecordsRequest{Filter: &pbrc.Record{Release: &pbgd.Release{}}}, grpc.MaxCallRecvMsgSize(1024*1024*1024))
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		for _, r := range recs.GetRecords() {
			if r.GetMetadata().Category == pbrc.ReleaseMetadata_PRE_FRESHMAN {
				fmt.Printf("%v %v - %v\n", r.GetMetadata().DateAdded, r.GetRelease().Title, r.GetRelease().Formats)
			}
		}

	}
}
