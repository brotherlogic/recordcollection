package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/brotherlogic/goserver/utils"
	"github.com/golang/protobuf/proto"

	"github.com/andanhm/go-prettytime"

	pbgd "github.com/brotherlogic/godiscogs"
	pbrc "github.com/brotherlogic/recordcollection/proto"

	//Needed to pull in gzip encoding init
	_ "google.golang.org/grpc/encoding/gzip"
)

type wstr struct {
	Stoid       string  `json:"stoid"`
	FolderId    int32   `json:"folder_id"`
	Instance_id int32   `json:"instance_id"`
	Width       float64 `json:"width"`
}

func main() {
	ctx, cancel := utils.ManualContext("recordcollectioncli-"+os.Args[1], time.Minute*60)
	defer cancel()

	conn, err := utils.LFDialServer(ctx, "recordcollection")
	if err != nil {
		log.Fatalf("Cannot reach rc: %v", err)
	}
	defer conn.Close()

	registry := pbrc.NewRecordCollectionServiceClient(conn)

	switch os.Args[1] {
	case "folder":
		i, _ := strconv.Atoi(os.Args[2])
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_FolderId{int32(i)}})
		if err != nil {
			log.Fatalf("Bad: %v", err)
		}
		for _, id := range ids.GetInstanceIds() {
			fmt.Printf("%v\n", id)
			up := &pbrc.UpdateRecordRequest{Reason: "cli-sellrequest", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: id}, Metadata: &pbrc.ReleaseMetadata{SetRating: -1, MoveFolder: 673768, Category: pbrc.ReleaseMetadata_STAGED_TO_SELL}}}
			_, err := registry.UpdateRecord(ctx, up)
			if err != nil {
				log.Fatalf("Bad Update: %v", err)
			}
		}
	case "check":
		checkFlags := flag.NewFlagSet("Check", flag.ExitOnError)
		var id = checkFlags.Int("id", -1, "Id of the record to check")
		if err := checkFlags.Parse(os.Args[2:]); err == nil {
			record, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: int32(*id)})
			if err != nil {
				log.Fatalf("Error getting record: %v", err)
			}

			fmt.Printf("%v - %v\n\n", record.GetRecord().GetRelease().GetArtists(), record.GetRecord().GetRelease().GetTitle())
			fmt.Printf("Added %v\n", prettytime.Format(time.Unix(record.GetRecord().Metadata.GetDateAdded(), 0)))
			fmt.Printf("Filed under %v\n", record.GetRecord().GetMetadata().GetFiledUnder())
			if record.GetRecord().GetMetadata().GetFilePath() != "" || record.GetRecord().GetMetadata().GetCdPath() != "" {
				fmt.Printf("Has CD / File paths %v and %v\n", record.GetRecord().GetMetadata().GetFilePath(), record.GetRecord().GetMetadata().GetCdPath())
			}
			fmt.Printf("Width is %v, Weight is %v\n\n", record.GetRecord().GetMetadata().GetRecordWidth(), record.GetRecord().GetMetadata().GetWeightInGrams())

			switch record.GetRecord().GetMetadata().GetGoalFolder() {
			case 242017:
				fmt.Print("Goal Folder is 12 Inches\n")
			case 2259637:
				fmt.Print("Goal Folder is Keepers\n")
			case 1613206:
				fmt.Print("This record has been sold, but has the wrong goal folder\n")
				if record.GetRecord().GetMetadata().GetSaleId() == 0 {
					fmt.Print("This record needs to have the sale id set\n")
				}
			default:
				fmt.Printf("Don't know goal folder %v\n", record.GetRecord().GetMetadata().GetGoalFolder())
			}
			if record.GetRecord().GetMetadata().GetSaleId() > 0 || record.GetRecord().GetMetadata().GetSaleState() != pbgd.SaleState_NOT_FOR_SALE {
				fmt.Printf("This is in the sales loop (%v) -> %v\n", record.GetRecord().GetMetadata().GetSaleId(), record.GetRecord().GetMetadata().GetSaleState())
			}

			fmt.Printf("\nWas cleaned %v\n", prettytime.Format(time.Unix(record.GetRecord().GetMetadata().GetLastCleanDate(), 0)))
			fmt.Printf("Was played %v\n", prettytime.Format(time.Unix(record.GetRecord().GetMetadata().GetLastListenTime(), 0)))
		}
	case "passwidth":
		i, _ := strconv.Atoi(os.Args[2])
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_FolderId{int32(i)}})
		if err != nil {
			log.Fatalf("Bad: %v", err)
		}
		for _, id := range ids.GetInstanceIds() {
			ctx, cancel2 := utils.ManualContext("recordcollectioncli-"+os.Args[1], time.Second*10)
			defer cancel2()
			srec, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: int32(id)})
			if err != nil {
				log.Fatalf("Unable to get record: %v", err)
			}
			up := wstr{
				Stoid:       os.Args[3],
				FolderId:    int32(i),
				Instance_id: int32(id),
				Width:       float64(srec.GetRecord().Metadata.GetRecordWidth()),
			}

			jsonData, err := json.Marshal(up)
			log.Printf("SEND %v", string(jsonData))

			if err != nil {
				log.Fatalf("Unable to parse json: %v", err)
			}

			resp, err := http.Post("https://straightenthemout-qo2wxnmyfq-uw.a.run.app/straightenthemout.STOService/SetWidth", "application/json",
				bytes.NewBuffer(jsonData))

			if err != nil {
				log.Fatal(err)
			}

			defer resp.Body.Close()
			body, err := ioutil.ReadAll(resp.Body)
			fmt.Printf("%v\n", string(body))
		}

	case "updates":
		i, _ := strconv.Atoi(os.Args[2])
		res, err := registry.GetUpdates(ctx, &pbrc.GetUpdatesRequest{InstanceId: int32(i)})
		if err != nil {
			log.Fatalf("Bad updates: %v", err)
		}
		for i, update := range res.GetUpdates().GetUpdates() {
			fmt.Printf("%v. [%v], %v\n", i, time.Unix(update.GetTime(), 0), update)
		}
		if len(res.GetUpdates().GetUpdates()) == 0 {
			fmt.Printf("No updates for %v\n", i)
		}
	case "trigger":
		res, err := registry.Trigger(ctx, &pbrc.TriggerRequest{})
		fmt.Printf("%v,%v\n", res, err)
	case "order":
		res, err := registry.GetOrder(ctx, &pbrc.GetOrderRequest{Id: "150295-1"})
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
	case "box45":
		i, _ := strconv.Atoi(os.Args[2])
		srec, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: int32(i)})

		if err != nil {
			log.Fatalf("Error getting record: %v", err)
		}

		up := &pbrc.UpdateRecordRequest{Reason: "CLI-box", Update: &pbrc.Record{
			Release: &pbgd.Release{
				InstanceId: srec.GetRecord().GetRelease().InstanceId},
			Metadata: &pbrc.ReleaseMetadata{NewBoxState: pbrc.ReleaseMetadata_IN_45S_BOX, Dirty: true},
		}}
		rec, err := registry.UpdateRecord(ctx, up)
		if err != nil {
			log.Fatalf("error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	case "box":
		i, _ := strconv.Atoi(os.Args[2])
		srec, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: int32(i)})

		if err != nil {
			log.Fatalf("Error getting record: %v", err)
		}

		up := &pbrc.UpdateRecordRequest{Reason: "CLI-box", Update: &pbrc.Record{
			Release: &pbgd.Release{
				InstanceId: srec.GetRecord().GetRelease().InstanceId},
			Metadata: &pbrc.ReleaseMetadata{NewBoxState: pbrc.ReleaseMetadata_IN_THE_BOX, Dirty: true},
		}}
		rec, err := registry.UpdateRecord(ctx, up)
		if err != nil {
			log.Fatalf("error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	case "boxbox":
		i, _ := strconv.Atoi(os.Args[2])
		srec, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: int32(i)})

		if err != nil {
			log.Fatalf("Error getting record: %v", err)
		}

		up := &pbrc.UpdateRecordRequest{Reason: "CLI-box", Update: &pbrc.Record{
			Release: &pbgd.Release{
				InstanceId: srec.GetRecord().GetRelease().InstanceId},
			Metadata: &pbrc.ReleaseMetadata{NewBoxState: pbrc.ReleaseMetadata_IN_BOXSET_BOX, Dirty: true},
		}}
		rec, err := registry.UpdateRecord(ctx, up)
		if err != nil {
			log.Fatalf("error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	case "unbox":
		i, _ := strconv.Atoi(os.Args[2])
		srec, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: int32(i)})

		if err != nil {
			log.Fatalf("Error getting record: %v", err)
		}

		up := &pbrc.UpdateRecordRequest{Reason: "CLI-unbox", Update: &pbrc.Record{
			Release: &pbgd.Release{
				InstanceId: srec.GetRecord().GetRelease().InstanceId},
			Metadata: &pbrc.ReleaseMetadata{NewBoxState: pbrc.ReleaseMetadata_OUT_OF_BOX, Dirty: true},
		}}
		rec, err := registry.UpdateRecord(ctx, up)
		if err != nil {
			log.Fatalf("error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)

	case "sold_price":
		i, _ := strconv.Atoi(os.Args[2])
		date, _ := strconv.Atoi(os.Args[3])
		price, _ := strconv.Atoi(os.Args[4])
		srec, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: int32(i)})

		if err != nil {
			log.Fatalf("Error getting record: %v", err)
		}

		up := &pbrc.UpdateRecordRequest{Reason: "CLI-stock", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: srec.GetRecord().GetRelease().InstanceId}, Metadata: &pbrc.ReleaseMetadata{SoldPrice: int32(price), SoldDate: int64(date)}}}
		rec, err := registry.UpdateRecord(ctx, up)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)

	case "validate":
		i, _ := strconv.Atoi(os.Args[2])
		srec, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: int32(i)})

		if err != nil {
			log.Fatalf("Error getting record: %v", err)
		}

		up := &pbrc.UpdateRecordRequest{Reason: "CLI-stock", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: srec.GetRecord().GetRelease().InstanceId}, Metadata: &pbrc.ReleaseMetadata{LastValidate: time.Now().Unix()}}}
		rec, err := registry.UpdateRecord(ctx, up)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	case "listen":
		i, _ := strconv.Atoi(os.Args[2])
		srec, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: int32(i)})

		if err != nil {
			log.Fatalf("Error getting record: %v", err)
		}

		up := &pbrc.UpdateRecordRequest{Reason: "CLI-lastlisten", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: srec.GetRecord().GetRelease().InstanceId}, Metadata: &pbrc.ReleaseMetadata{LastListenTime: time.Now().Unix()}}}
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

	case "trans":
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_Category{pbrc.ReleaseMetadata_PRE_SOPHMORE}})
		if err != nil {
			fmt.Printf("Error %v\n", err)
		}
		for i, id := range ids.GetInstanceIds() {
			r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			}
			fmt.Printf("%v. %v [%v]\n", i, r.GetRecord().GetRelease().GetTitle(), r.GetRecord().GetRelease().GetInstanceId())
		}
	case "fget":
		i, _ := strconv.Atoi(os.Args[2])
		r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{ReleaseId: int32(i)})
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		}
		fmt.Printf("Release: %v\n", r.GetRecord().GetRelease())
		fmt.Println()
		fmt.Printf("Metadata: %v\n", r.GetRecord().GetMetadata())

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
	case "new_score":
		i, f := strconv.Atoi(os.Args[2])
		ns, _ := strconv.Atoi(os.Args[3])
		if f != nil {
			log.Fatalf("Hmm %v", f)
		}
		u, err := registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Reason: "recordcollection-cli_reset_score",
			Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)},
				Metadata: &pbrc.ReleaseMetadata{SetRating: int32(ns)}}})
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Update: %v", u)
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
			fmt.Println()
			fmt.Printf("Metadata: %v, %v\n", srec.GetRecord().GetMetadata(), srec.GetRecord().GetRelease().GetDigitalVersions())
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
	case "width":
		i, _ := strconv.Atoi(os.Args[2])
		f, _ := strconv.ParseFloat(os.Args[3], 32)
		rec, err := registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Reason: "CLI-spfolder", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{RecordWidth: float32(f)}}})
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	case "is_twelve":
		i, _ := strconv.Atoi(os.Args[2])
		rec, err := registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Reason: "CLI-spfolder", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{RecordWidth: -1, GoalFolder: 242017, FiledUnder: pbrc.ReleaseMetadata_FILE_12_INCH}}})
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	case "is_seven":
		i, _ := strconv.Atoi(os.Args[2])
		rec, err := registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Reason: "CLI-spfolder", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{RecordWidth: -1, FiledUnder: pbrc.ReleaseMetadata_FILE_7_INCH}}})
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	case "is_cd":
		i, _ := strconv.Atoi(os.Args[2])
		rec, err := registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Reason: "CLI-spfolder", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{FiledUnder: pbrc.ReleaseMetadata_FILE_CD}}})
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	case "is_tape":
		i, _ := strconv.Atoi(os.Args[2])
		rec, err := registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Reason: "CLI-spfolder", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{FiledUnder: pbrc.ReleaseMetadata_FILE_TAPE}}})
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	case "is_box":
		i, _ := strconv.Atoi(os.Args[2])
		rec, err := registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Reason: "CLI-spfolder", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{FiledUnder: pbrc.ReleaseMetadata_FILE_BOXSET}}})
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	case "is_digital":
		i, _ := strconv.Atoi(os.Args[2])
		rec, err := registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Reason: "CLI-spfolder", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{FiledUnder: pbrc.ReleaseMetadata_FILE_DIGITAL}}})
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	case "cleaned":
		i, _ := strconv.Atoi(os.Args[2])
		rec, err := registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Reason: "CLI-spfolder", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{LastCleanDate: time.Now().Unix()}}})
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	case "weight":
		i, _ := strconv.Atoi(os.Args[2])
		f, _ := strconv.ParseFloat(os.Args[3], 32)
		rec, err := registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Reason: "CLI-spfolder", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{WeightInGrams: int32(f)}}})
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	case "fixfolder":
		i, _ := strconv.Atoi(os.Args[2])
		f, _ := strconv.Atoi(os.Args[3])
		rec, err := registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Reason: "CLI-spfolder", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i), FolderId: int32(f)}}})
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
	case "remove_sale":
		i, _ := strconv.Atoi(os.Args[2])
		up := &pbrc.UpdateRecordRequest{Reason: "CLI-reset_sale", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{Dirty: true, SaleId: -1, SaleState: pbgd.SaleState_EXPIRED}}}
		rec, err := registry.UpdateRecord(ctx, up)
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
	case "mark_sold":
		i, _ := strconv.Atoi(os.Args[2])
		up := &pbrc.UpdateRecordRequest{Reason: "CLI-reset_sale", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{SaleState: pbgd.SaleState_SOLD}}}
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
		up := &pbrc.UpdateRecordRequest{Reason: "cli-sellrequest", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{SetRating: -1, MoveFolder: 673768, Category: pbrc.ReleaseMetadata_STAGED_TO_SELL}}}
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
	case "arrived":
		i, _ := strconv.Atoi(os.Args[2])
		up := &pbrc.UpdateRecordRequest{Reason: "cli-arrived", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{DateArrived: time.Now().Unix()}}}
		rec, err := registry.UpdateRecord(ctx, up)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	case "farrived":
		i, _ := strconv.Atoi(os.Args[2])
		up := &pbrc.UpdateRecordRequest{Reason: "cli-arrived", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{Dirty: true, Category: pbrc.ReleaseMetadata_ARRIVED,

			DateArrived: time.Now().Unix()}}}
		rec, err := registry.UpdateRecord(ctx, up)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	case "ready_to_listen":
		i, _ := strconv.Atoi(os.Args[2])
		up := &pbrc.UpdateRecordRequest{Reason: "cli-ready_toListne", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{Category: pbrc.ReleaseMetadata_UNLISTENED}}}
		rec, err := registry.UpdateRecord(ctx, up)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	case "unarrived":
		i, _ := strconv.Atoi(os.Args[2])
		up := &pbrc.UpdateRecordRequest{Reason: "cli-arrived", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{Category: pbrc.ReleaseMetadata_PURCHASED, DateArrived: -1}}}
		rec, err := registry.UpdateRecord(ctx, up)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	case "ready":
		i, _ := strconv.Atoi(os.Args[2])
		up := &pbrc.UpdateRecordRequest{Reason: "cli-ready", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{Category: pbrc.ReleaseMetadata_UNLISTENED}}}
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
	case "commit":
		i, _ := strconv.Atoi(os.Args[2])
		rec, err := registry.CommitRecord(ctx, &pbrc.CommitRecordRequest{InstanceId: int32(i)})
		fmt.Printf("Got %v and %v", rec, err)
	case "add":
		i, _ := strconv.Atoi(os.Args[2])
		f, _ := strconv.Atoi(os.Args[3])
		c, _ := strconv.Atoi(os.Args[4])
		rec, err := registry.AddRecord(ctx, &pbrc.AddRecordRequest{
			ToAdd: &pbrc.Record{
				Release:  &pbgd.Release{Id: int32(i)},
				Metadata: &pbrc.ReleaseMetadata{Cost: int32(c), GoalFolder: int32(f), AccountingYear: 2021},
			},
		})
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("%v and %v\n", rec, err)
	case "newfolder":
		i, _ := strconv.Atoi(os.Args[2])
		i2, _ := strconv.Atoi(os.Args[3])
		up := &pbrc.UpdateRecordRequest{Reason: "cli-newfolder", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{MoveFolder: int32(i2)}}}
		rec, err := registry.UpdateRecord(ctx, up)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	default:
		fmt.Printf("Unknown comand: %v\n", os.Args[1])
	}
}
