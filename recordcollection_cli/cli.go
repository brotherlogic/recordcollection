package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/brotherlogic/goserver/utils"
	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"

	pbgd "github.com/brotherlogic/godiscogs"
	pbgs "github.com/brotherlogic/goserver/proto"
	pbrc "github.com/brotherlogic/recordcollection/proto"
	pbt "github.com/brotherlogic/tracer/proto"

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

	ctx, cancel := utils.BuildContext("recordcollectioncli-"+os.Args[1], "recordcollection", pbgs.ContextType_MEDIUM)
	defer cancel()

	switch os.Args[1] {
	case "testing":
		rec, err := registry.GetRecords(ctx, &pbrc.GetRecordsRequest{Filter: &pbrc.Record{Release: &pbgd.Release{}, Metadata: &pbrc.ReleaseMetadata{GoalFolder: 242017}}})
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		log.Printf("GOT %v records", len(rec.GetRecords()))
	case "getsales":
		rec, err := registry.GetRecords(ctx, &pbrc.GetRecordsRequest{Filter: &pbrc.Record{Release: &pbgd.Release{}, Metadata: &pbrc.ReleaseMetadata{SalePrice: 500}}})
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("%v records\n", len(rec.GetRecords()))
		for i, rec := range rec.GetRecords() {
			fmt.Printf("%v. %v\n", i, rec.GetRelease().Title)
		}

	case "get":
		i, _ := strconv.Atoi(os.Args[2])
		rec, err := registry.GetRecords(ctx, &pbrc.GetRecordsRequest{Force: true, Filter: &pbrc.Record{Release: &pbgd.Release{Id: int32(i)}}})

		if err == nil {
			fmt.Printf("Time Taken: %v\n", rec.InternalProcessingTime)
			for _, r := range rec.GetRecords() {
				fmt.Printf("Release: %v\n", r.GetRelease())
				fmt.Printf("Metadata: %v\n", r.GetMetadata())
				fmt.Printf("Labels: %v\n", len(r.GetRelease().Labels))
				fmt.Printf("1 %v, %v, %v %v", r.GetMetadata().GetDateAdded() > (time.Now().AddDate(0, -3, 0).Unix()), r.GetMetadata().DateAdded, r.GetRelease().Rating == 0, r.GetRelease().Rating)
			}
		} else {
			fmt.Printf("Error: %v", err)
		}
	case "sget":
		i, _ := strconv.Atoi(os.Args[2])
		rec, err := registry.GetRecords(ctx, &pbrc.GetRecordsRequest{Force: true, Filter: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}}})

		if err == nil {
			for _, r := range rec.GetRecords() {
				fmt.Printf("Release: %v\n", r.GetRelease())
				fmt.Printf("Metadata: %v\n", r.GetMetadata())
				fmt.Printf("Labels: %v\n", len(r.GetRelease().Labels))
				fmt.Printf("1 %v, %v, %v %v", r.GetMetadata().GetDateAdded() > (time.Now().AddDate(0, -3, 0).Unix()), r.GetMetadata().DateAdded, r.GetRelease().Rating == 0, r.GetRelease().Rating)
			}
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
		rec, err := registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{LastCache: 1}}})
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
		recs, err := registry.GetRecords(ctx, &pbrc.GetRecordsRequest{Filter: &pbrc.Record{Release: &pbgd.Release{Id: int32(i)}}})
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		for _, r := range recs.GetRecords() {
			if r.GetMetadata().Category == pbrc.ReleaseMetadata_ASSESS || r.GetMetadata().Category == pbrc.ReleaseMetadata_ASSESS_FOR_SALE {
				up := &pbrc.UpdateRecordRequest{Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: r.GetRelease().InstanceId}, Metadata: &pbrc.ReleaseMetadata{LastStockCheck: time.Now().Unix()}}}
				rec, err := registry.UpdateRecord(ctx, up)
				if err != nil {
					log.Fatalf("Error: %v", err)
				}
				fmt.Printf("Updated: %v", rec)
			}
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

	}
	utils.SendTrace(ctx, "recordcollectioncli-"+os.Args[1], time.Now(), pbt.Milestone_END, "recordcollection")
}
