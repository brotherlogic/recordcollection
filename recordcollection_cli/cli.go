package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/brotherlogic/goserver/utils"
	"google.golang.org/protobuf/proto"

	"github.com/andanhm/go-prettytime"

	google_protobuf "github.com/golang/protobuf/ptypes/any"

	dspb "github.com/brotherlogic/dstore/proto"
	pbgd "github.com/brotherlogic/godiscogs/proto"
	pbks "github.com/brotherlogic/keystore/proto"
	qpb "github.com/brotherlogic/queue/proto"
	pbrc "github.com/brotherlogic/recordcollection/proto"
	rfpb "github.com/brotherlogic/recordfanout/proto"
)

type wstr struct {
	Stoid       string  `json:"stoid"`
	FolderId    int32   `json:"folder_id"`
	Instance_id int32   `json:"instance_id"`
	Width       float64 `json:"width"`
}

func convertKeep(k pbrc.ReleaseMetadata_KeepState) string {
	switch k {
	case pbrc.ReleaseMetadata_DIGITAL_KEEPER:
		return "digital"
	case pbrc.ReleaseMetadata_KEEPER:
		return "keep"
	case pbrc.ReleaseMetadata_NOT_KEEPER:
		return "none"
	}

	log.Fatalf("Bad state: %v", k)
	return ""
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
	case "find_digital":
		items, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_UpdateTime{0}})
		if err != nil {
			log.Fatalf("Unable to get all records: %v", err)
		}
		for _, item := range items.GetInstanceIds() {
			rec, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: item})
			if err != nil {
				log.Fatalf("Bad records: %v", err)
			}
			if rec.Record.GetMetadata().GetFiledUnder() == pbrc.ReleaseMetadata_FILE_DIGITAL {
				found := false
				for _, form := range rec.GetRecord().GetRelease().GetFormats() {
					if form.GetName() == "File" {
						found = true
					}
				}
				if !found {
					fmt.Printf("%v -> %v / %v\n", rec.Record.GetRelease().GetInstanceId(), rec.GetRecord().Release.GetArtists(), rec.GetRecord().GetRelease().GetTitle())
					return
				}
			}
		}
	case "sales":
		items, err := registry.GetInventory(ctx, &pbrc.GetInventoryRequest{})
		if err != nil {
			log.Fatalf("Unable to get inventory: %v", err)
		}
		for _, item := range items.GetItems() {
			td := time.Unix(item.GetDatePosted(), 0)
			if time.Since(td) > time.Hour*24*7 && item.SalePrice <= 500 {
				fmt.Printf("%v - %v\n", item.GetId(), td)
			}
		}

	case "sanity":
		collection := &pbrc.RecordCollection{}
		conn, err := utils.LFDialServer(ctx, "keystore")
		if err != nil {
			log.Fatalf("Bad Dial: %v", err)
		}
		defer conn.Close()
		ksc := pbks.NewKeyStoreServiceClient(conn)
		res, err := ksc.Read(ctx, &pbks.ReadRequest{Key: "/github.com/brotherlogic/recordcollection/collection"})

		if err != nil {
			log.Fatalf("Bad Read: %v", err)
		}

		proto.Unmarshal(res.GetPayload().GetValue(), collection)

		fmt.Printf("READ: %vkb\n", proto.Size(collection)/(1024))
		fmt.Printf("Size: %v\n", len(collection.GetNeedsPush()))

	case "folder":
		i, _ := strconv.Atoi(os.Args[2])
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_FolderId{int32(i)}})
		if err != nil {
			log.Fatalf("Bad: %v", err)
		}
		for _, id := range ids.GetInstanceIds() {
			fmt.Printf("%v\n", id)
		}
	case "bad_bandcamp":
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_UpdateTime{0}})
		if err != nil {
			log.Fatalf("Bad: %v", err)
		}
		for _, id := range ids.GetInstanceIds() {
			rec, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				log.Fatalf("Bad read: %v", err)
			}

			if rec.Record.GetMetadata().GetGoalFolder() == 1782105 {
				found := false
				for _, format := range rec.GetRecord().GetRelease().GetFormats() {
					if format.Name == "File" {
						found = true
					}
				}

				if !found {
					fmt.Printf("%v\n", id)
				}
			}
		}

	case "transfer":
		i, _ := strconv.ParseInt(os.Args[2], 10, 32)
		ni, _ := strconv.ParseInt(os.Args[3], 10, 32)
		ids, err := registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Reason: "transfer",
			Update: &pbrc.Record{
				Release:  &pbgd.Release{InstanceId: int32(i)},
				Metadata: &pbrc.ReleaseMetadata{TransferTo: int32(ni)},
			}})
		fmt.Printf("Transfer: %v, %v", ids, err)
	case "get_price":
		i, _ := strconv.Atoi(os.Args[2])
		ids, err := registry.GetPrice(ctx, &pbrc.GetPriceRequest{Id: int32(i)})
		fmt.Printf("Result = %v, %v\n", ids, err)
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
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_Category{pbrc.ReleaseMetadata_PRE_SOFT_VALIDATE}})
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
	case "phs":
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_Category{pbrc.ReleaseMetadata_PRE_HIGH_SCHOOL}})
		if err != nil {
			fmt.Printf("Error %v\n", err)
		}
		for i, id := range ids.GetInstanceIds() {
			r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			}
			fmt.Printf("%v. %v %v %v\n", i, r.GetRecord().GetRelease().GetInstanceId(), r.GetRecord().GetRelease().GetTitle(), r.GetRecord().GetMetadata().GetFiledUnder())
		}
	case "phs-ping":
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_Category{pbrc.ReleaseMetadata_HIGH_SCHOOL}})
		if err != nil {
			fmt.Printf("Error %v\n", err)
		}
		for i, id := range ids.GetInstanceIds() {
			r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			}
			fmt.Printf("%v. %v %v %v\n", i, r.GetRecord().GetRelease().GetInstanceId(), r.GetRecord().GetRelease().GetTitle(), r.GetRecord().GetMetadata().GetFiledUnder())
		}
	case "withbudget":
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_UpdateTime{0}})
		if err != nil {
			fmt.Printf("Error %v\n", err)
		}
		for i, id := range ids.GetInstanceIds() {
			r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			}
			if r.GetRecord().GetMetadata().GetPurchaseBudget() == os.Args[2] {
				fmt.Printf("%v %v. %v [%v] %v\n", r.GetRecord().GetMetadata().GetCost(), i, r.GetRecord().GetRelease().GetTitle(), r.GetRecord().GetRelease().GetInstanceId(), r.GetRecord().GetMetadata().GetFiledUnder())
			}
		}
	case "phs-digital":
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_Category{pbrc.ReleaseMetadata_PRE_HIGH_SCHOOL}})
		if err != nil {
			fmt.Printf("Error %v\n", err)
		}
		for i, id := range ids.GetInstanceIds() {
			r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			}
			if r.Record.GetMetadata().GetFiledUnder() == pbrc.ReleaseMetadata_FILE_DIGITAL {
				_, err := registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Reason: "recordcollection-cli_reset_score",
					Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(id)},
						Metadata: &pbrc.ReleaseMetadata{SetRating: int32(5)}}})
				fmt.Printf("%v. %v, %v\n", i, id, err)
			}
		}
	case "arr":
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_Category{pbrc.ReleaseMetadata_ARRIVED}})
		if err != nil {
			fmt.Printf("Error %v\n", err)
		}
		conn, err := utils.LFDialServer(ctx, "recordfanout")
		if err != nil {
			log.Fatalf("Bad: %v", err)
		}
		defer conn.Close()
		client := rfpb.NewRecordFanoutServiceClient(conn)
		for i, id := range ids.GetInstanceIds() {
			r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			}
			_, err = client.Fanout(ctx, &rfpb.FanoutRequest{InstanceId: id})
			fmt.Printf("%v. %v [%v] %v = %v\n", i, r.GetRecord().GetRelease().GetTitle(), r.GetRecord().GetRelease().GetInstanceId(), r.GetRecord().GetMetadata().GetFiledUnder(), err)

		}
	case "ping":
		conn, err := utils.LFDialServer(ctx, "recordfanout")
		if err != nil {
			log.Fatalf("Bad: %v", err)
		}
		defer conn.Close()
		client := rfpb.NewRecordFanoutServiceClient(conn)

		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_Category{pbrc.ReleaseMetadata_HIGH_SCHOOL}})
		if err != nil {
			fmt.Printf("Error %v\n", err)
		}

		for i, id := range ids.GetInstanceIds() {
			r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			}
			_, err = client.Fanout(ctx, &rfpb.FanoutRequest{InstanceId: id})
			fmt.Printf("%v. %v [%v] %v = %v\n", i, r.GetRecord().GetRelease().GetTitle(), r.GetRecord().GetRelease().GetInstanceId(), r.GetRecord().GetMetadata().GetFiledUnder(), err)
		}
		ids, err = registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_Category{pbrc.ReleaseMetadata_STAGED}})
		if err != nil {
			fmt.Printf("Error %v\n", err)
		}

		for i, id := range ids.GetInstanceIds() {
			r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			}
			_, err = client.Fanout(ctx, &rfpb.FanoutRequest{InstanceId: id})
			fmt.Printf("%v. %v [%v] %v = %v\n", i, r.GetRecord().GetRelease().GetTitle(), r.GetRecord().GetRelease().GetInstanceId(), r.GetRecord().GetMetadata().GetFiledUnder(), err)
		}
	case "next":
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_UpdateTime{0}})
		if err != nil {
			fmt.Printf("Error %v\n", err)
		}

		sort.SliceStable(ids.InstanceIds, func(a, b int) bool {
			return ids.InstanceIds[a] < ids.InstanceIds[b]
		})

		for i, id := range ids.GetInstanceIds() {
			r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			}

			if r.GetRecord().GetMetadata().GetBoxState() != pbrc.ReleaseMetadata_OUT_OF_BOX &&
				r.GetRecord().GetMetadata().GetCategory() != pbrc.ReleaseMetadata_SOLD_ARCHIVE {
				fmt.Printf("%v. %v [%v] %v\n", i, r.GetRecord().GetRelease().GetTitle(), r.GetRecord().GetRelease().GetInstanceId(), r.GetRecord().GetMetadata().GetFiledUnder())
			}
		}
	case "all_listen":
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_UpdateTime{0}})
		if err != nil {
			fmt.Printf("Error %v\n", err)
		}

		sort.SliceStable(ids.InstanceIds, func(a, b int) bool {
			return ids.InstanceIds[a] < ids.InstanceIds[b]
		})

		for _, id := range ids.GetInstanceIds() {
			r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			}

			if r.GetRecord().GetMetadata().GetLastCleanDate() > 0 {
				fmt.Printf("%v %v\n", r.GetRecord().GetRelease().GetInstanceId(), r.GetRecord().GetMetadata().GetLastListenTime())
			}
		}
	case "bad":
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_UpdateTime{0}})
		if err != nil {
			fmt.Printf("Error %v\n", err)
		}

		for i, id := range ids.GetInstanceIds() {
			r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			}

			if r.GetRecord().GetMetadata().GetBoxState() != pbrc.ReleaseMetadata_OUT_OF_BOX &&
				r.GetRecord().GetMetadata().GetBoxState() != pbrc.ReleaseMetadata_BOX_UNKNOWN &&
				r.GetRecord().GetMetadata().GetFiledUnder() != pbrc.ReleaseMetadata_FILE_UNKNOWN {
				fmt.Printf("%v. %v [%v] %v\n", i, r.GetRecord().GetRelease().GetTitle(), r.GetRecord().GetRelease().GetInstanceId(), r.GetRecord().GetMetadata().GetFiledUnder())
			}
		}
	case "ul":
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_Category{pbrc.ReleaseMetadata_UNLISTENED}})
		if err != nil {
			fmt.Printf("Error %v\n", err)
		}
		for i, id := range ids.GetInstanceIds() {
			r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			}
			fmt.Printf("%v. %v [%v] %v\n", i, r.GetRecord().GetRelease().GetTitle(), r.GetRecord().GetRelease().GetInstanceId(), r.GetRecord().GetMetadata().GetFiledUnder())
		}
	case "staged":
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_Category{pbrc.ReleaseMetadata_STAGED}})
		if err != nil {
			fmt.Printf("Error %v\n", err)
		}
		var records []*pbrc.Record
		for _, id := range ids.GetInstanceIds() {
			r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			}
			records = append(records, r.GetRecord())
		}
		sort.SliceStable(records, func(i, j int) bool {
			return records[i].GetMetadata().GetLastListenTime() < records[j].GetMetadata().GetLastListenTime()
		})

		for i, r := range records {
			fmt.Printf("%v. %v [%v] %v\n", i, r.GetRelease().GetTitle(), r.GetRelease().GetInstanceId(), r.GetMetadata().GetFiledUnder())
		}
	case "hs":
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_Category{pbrc.ReleaseMetadata_HIGH_SCHOOL}})
		if err != nil {
			fmt.Printf("Error %v\n", err)
		}
		var records []*pbrc.Record
		for _, id := range ids.GetInstanceIds() {
			r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			}
			records = append(records, r.GetRecord())
		}
		sort.SliceStable(records, func(i, j int) bool {
			return records[i].GetMetadata().GetLastListenTime() < records[j].GetMetadata().GetLastListenTime()
		})

		for i, r := range records {
			fmt.Printf("%v. %v [%v] %v\n", i, r.GetRelease().GetTitle(), r.GetRelease().GetInstanceId(), r.GetMetadata().GetFiledUnder())
		}
	case "lp":
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_FolderId{812802}})
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

	case "sts":
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_Category{pbrc.ReleaseMetadata_STAGED_TO_SELL}})
		if err != nil {
			fmt.Printf("Error %v\n", err)
		}
		width := float32(0)
		for i, id := range ids.GetInstanceIds() {
			r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			}
			fmt.Printf("%v. %v [%v]\n", i, r.GetRecord().GetRelease().GetTitle(), r.GetRecord().GetRelease().GetInstanceId())
			width += (r.GetRecord().GetMetadata().GetRecordWidth())
		}
		fmt.Printf("Width = %v -> %v\n", width, width*1.25)
	case "run_full_update":
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_UpdateTime{0}})
		if err != nil {
			fmt.Printf("Error %v\n", err)
		}
		for i, id := range ids.GetInstanceIds() {
			r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			}
			if r.GetRecord().GetMetadata().GetBoxState() == pbrc.ReleaseMetadata_IN_45S_BOX && r.GetRecord().GetMetadata().GetGoalFolder() != 267116 {
				fmt.Printf("%v. %v [%v]\n", i, r.GetRecord().GetRelease().GetTitle(), r.GetRecord().GetRelease().GetInstanceId())
				_, err := registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Reason: "CLI-spfolder", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: id}, Metadata: &pbrc.ReleaseMetadata{GoalFolder: 267116}}})
				if err != nil {
					log.Fatalf("Bad update: %v", err)
				}
			}
		}
	case "inner":
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_UpdateTime{0}})
		if err != nil {
			fmt.Printf("Error %v\n", err)
		}
		for _, id := range ids.GetInstanceIds() {
			r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			}
			if r.GetRecord().GetMetadata().GetSleeve() == pbrc.ReleaseMetadata_VINYL_STORAGE_NO_INNER {
				fmt.Printf("%v - %v\n", r.GetRecord().GetRelease().GetInstanceId(), r.GetRecord().GetRelease().GetTitle())
			}
		}
	case "twelve_scores":
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_FolderId{242017}})
		if err != nil {
			fmt.Printf("Error %v\n", err)
		}
		for _, id := range ids.GetInstanceIds() {
			r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			}
			fmt.Printf("%v %v [%v] - %v\n", r.GetRecord().GetMetadata().GetOverallScore(), r.GetRecord().GetMetadata().GetCurrentSalePrice(), r.GetRecord().GetRelease().GetInstanceId(), r.GetRecord().GetRelease().GetTitle())

		}
	case "tc":
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_Category{pbrc.ReleaseMetadata_ARRIVED}})
		if err != nil {
			fmt.Printf("Error %v\n", err)
		}
		fmt.Printf("Found %v records", len(ids.GetInstanceIds()))
		for i, id := range ids.GetInstanceIds() {
			r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			}
			if r.GetRecord().GetMetadata().GetLastCleanDate() == 0 {
				if r.Record.GetMetadata().GetGoalFolder() == 242017 {
					fmt.Printf("%v %v. %v [%v]\n", time.Since(time.Unix(r.Record.Metadata.GetDateAdded(), 0)).Minutes(), i, r.GetRecord().GetRelease().GetTitle(), r.GetRecord().GetRelease().GetInstanceId())
				}
			}
		}
	case "pic":
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_Category{pbrc.ReleaseMetadata_PRE_IN_COLLECTION}})
		if err != nil {
			fmt.Printf("Error %v\n", err)
		}
		for i, id := range ids.GetInstanceIds() {
			r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			}
			if r.Record.GetMetadata().GetBoxState() == pbrc.ReleaseMetadata_BOX_UNKNOWN ||
				r.Record.GetMetadata().GetBoxState() == pbrc.ReleaseMetadata_OUT_OF_BOX {
				fmt.Printf("%v. %v [%v] %v\n", i, r.GetRecord().GetRelease().GetTitle(), r.GetRecord().GetRelease().GetInstanceId(), r.GetRecord().GetMetadata().GetFiledUnder())
			}
		}
	case "scores":
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_Category{pbrc.ReleaseMetadata_IN_COLLECTION}})
		if err != nil {
			fmt.Printf("Error %v\n", err)
		}
		for i, id := range ids.GetInstanceIds() {
			r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			}
			if r.Record.GetMetadata().GetBoxState() == pbrc.ReleaseMetadata_BOX_UNKNOWN ||
				r.Record.GetMetadata().GetBoxState() == pbrc.ReleaseMetadata_OUT_OF_BOX {
				fmt.Printf("%v %v. %v [%v] %v\n", r.GetRecord().GetRelease().GetRating(), i, r.GetRecord().GetRelease().GetTitle(), r.GetRecord().GetRelease().GetInstanceId(), r.GetRecord().GetMetadata().GetFiledUnder())
			}
		}
	case "pv":
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_Category{pbrc.ReleaseMetadata_PRE_VALIDATE}})
		if err != nil {
			fmt.Printf("Error %v\n", err)
		}
		for i, id := range ids.GetInstanceIds() {
			r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			}
			if r.Record.GetMetadata().GetBoxState() == pbrc.ReleaseMetadata_BOX_UNKNOWN ||
				r.Record.GetMetadata().GetBoxState() == pbrc.ReleaseMetadata_OUT_OF_BOX {
				fmt.Printf("%v %v. %v [%v] %v\n", r.GetRecord().GetRelease().GetRating(), i, r.GetRecord().GetRelease().GetTitle(), r.GetRecord().GetRelease().GetInstanceId(), r.GetRecord().GetMetadata().GetFiledUnder())
			}
		}
	case "scorepic":
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_Category{pbrc.ReleaseMetadata_PRE_IN_COLLECTION}})
		if err != nil {
			fmt.Printf("Error %v\n", err)
		}
		for i, id := range ids.GetInstanceIds() {
			r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			}
			if r.Record.GetMetadata().GetBoxState() == pbrc.ReleaseMetadata_BOX_UNKNOWN ||
				r.Record.GetMetadata().GetBoxState() == pbrc.ReleaseMetadata_OUT_OF_BOX {
				fmt.Printf("%v %v. %v [%v] %v\n", r.GetRecord().GetRelease().GetRating(), i, r.GetRecord().GetRelease().GetTitle(), r.GetRecord().GetRelease().GetInstanceId(), r.GetRecord().GetMetadata().GetFiledUnder())
				if r.GetRecord().GetMetadata().GetFiledUnder() == pbrc.ReleaseMetadata_FILE_DIGITAL || r.GetRecord().GetMetadata().GetFiledUnder() == pbrc.ReleaseMetadata_FILE_CD {
					_, err := registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Reason: "recordcollection-cli_reset_score",
						Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(id)},
							Metadata: &pbrc.ReleaseMetadata{SetRating: int32(5)}}})
					fmt.Printf("%v\n", err)
				}
			}
		}
	case "scorephs":
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_Category{pbrc.ReleaseMetadata_PRE_HIGH_SCHOOL}})
		if err != nil {
			fmt.Printf("Error %v\n", err)
		}
		for i, id := range ids.GetInstanceIds() {
			r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			}
			if r.Record.GetMetadata().GetBoxState() == pbrc.ReleaseMetadata_BOX_UNKNOWN ||
				r.Record.GetMetadata().GetBoxState() == pbrc.ReleaseMetadata_OUT_OF_BOX {
				fmt.Printf("%v %v. %v [%v] %v\n", r.GetRecord().GetRelease().GetRating(), i, r.GetRecord().GetRelease().GetTitle(), r.GetRecord().GetRelease().GetInstanceId(), r.GetRecord().GetMetadata().GetFiledUnder())
				if r.GetRecord().GetMetadata().GetFiledUnder() == pbrc.ReleaseMetadata_FILE_DIGITAL || r.GetRecord().GetMetadata().GetFiledUnder() == pbrc.ReleaseMetadata_FILE_CD {
					_, err := registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Reason: "recordcollection-cli_reset_score",
						Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(id)},
							Metadata: &pbrc.ReleaseMetadata{SetRating: int32(5)}}})
					fmt.Printf("%v\n", err)
				}
			}
		}
	case "scoreut":
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_Category{pbrc.ReleaseMetadata_UNLISTENED}})
		if err != nil {
			fmt.Printf("Error %v\n", err)
		}
		for i, id := range ids.GetInstanceIds() {
			r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			}
			if r.Record.GetMetadata().GetBoxState() == pbrc.ReleaseMetadata_BOX_UNKNOWN ||
				r.Record.GetMetadata().GetBoxState() == pbrc.ReleaseMetadata_OUT_OF_BOX {
				fmt.Printf("%v %v. %v [%v] %v\n", r.GetRecord().GetRelease().GetRating(), i, r.GetRecord().GetRelease().GetTitle(), r.GetRecord().GetRelease().GetInstanceId(), r.GetRecord().GetMetadata().GetFiledUnder())
				if r.GetRecord().GetMetadata().GetFiledUnder() == pbrc.ReleaseMetadata_FILE_DIGITAL {
					_, err := registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Reason: "recordcollection-cli_reset_score",
						Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(id)},
							Metadata: &pbrc.ReleaseMetadata{SetRating: int32(5)}}})
					fmt.Printf("%v\n", err)
				}
			}
		}
	case "limbo":
		limboFlags := flag.NewFlagSet("limbo", flag.ExitOnError)
		var arrived = limboFlags.Bool("arrived", false, "The name of the budget")
		if err := limboFlags.Parse(os.Args[2:]); err == nil {
			ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_FolderId{3380098}})
			if err != nil {
				log.Fatalf("Error %v\n", err)
			}
			for i, id := range ids.GetInstanceIds() {
				r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
				if err != nil {
					fmt.Printf("Error: %v\n", err)
				}
				if r.Record.GetMetadata().GetBoxState() == pbrc.ReleaseMetadata_BOX_UNKNOWN ||
					r.Record.GetMetadata().GetBoxState() == pbrc.ReleaseMetadata_OUT_OF_BOX {
					fmt.Printf("%v. %v [%v] - %v", i, r.GetRecord().GetRelease().GetTitle(), r.GetRecord().GetRelease().GetInstanceId(), r.GetRecord().GetMetadata().GetFiledUnder())
					if *arrived {
						up := &pbrc.UpdateRecordRequest{Reason: "cli-arrived", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: id}, Metadata: &pbrc.ReleaseMetadata{DateArrived: time.Now().Unix()}}}
						_, err := registry.UpdateRecord(ctx, up)
						if err == nil {
							fmt.Printf(": ARRIVED")
						} else {
							fmt.Printf(": Error settings arrived: %v", err)
						}
					}
					fmt.Printf("\n")
				}
			}
		} else {
			fmt.Printf("Cannot parse flags: %v", err)
		}
	case "psv":
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_Category{pbrc.ReleaseMetadata_PRE_SOFT_VALIDATE}})
		if err != nil {
			fmt.Printf("Error %v\n", err)
		}
		for i, id := range ids.GetInstanceIds() {
			r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			}
			if r.Record.GetMetadata().GetBoxState() == pbrc.ReleaseMetadata_BOX_UNKNOWN ||
				r.Record.GetMetadata().GetBoxState() == pbrc.ReleaseMetadata_OUT_OF_BOX {
				fmt.Printf("%v. %v [%v]\n", i, r.GetRecord().GetRelease().GetTitle(), r.GetRecord().GetRelease().GetInstanceId())
			}
		}
	case "twelves":
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_FolderId{242017}})
		if err != nil {
			fmt.Printf("Error %v\n", err)
		}
		for i, id := range ids.GetInstanceIds() {
			fmt.Printf("START %v/%v\n", i, len(ids.GetInstanceIds()))
			r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err == nil && (r.GetRecord().GetMetadata().GetKeep() != pbrc.ReleaseMetadata_RESET_TO_UNKNOWN && r.GetRecord().GetMetadata().GetKeep() != pbrc.ReleaseMetadata_KEEP_UNKNOWN) {
				_, err = registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{
					Reason: "Resetting keep status",
					Update: &pbrc.Record{
						Release:  &pbgd.Release{InstanceId: id},
						Metadata: &pbrc.ReleaseMetadata{Keep: pbrc.ReleaseMetadata_RESET_TO_UNKNOWN},
					}})
				fmt.Printf("RESET %v/%v = %v -> %v\n", i, len(ids.GetInstanceIds()), id, err)
			}
		}
	case "in_coll":
		i, f := strconv.Atoi(os.Args[2])
		if f != nil {
			log.Fatalf("Hmm %v", f)
		}
		u, err := registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Reason: "recordcollection-cli_reset_score",
			Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)},
				Metadata: &pbrc.ReleaseMetadata{Category: pbrc.ReleaseMetadata_IN_COLLECTION}}})
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Update: %v", u)
	case "sleeve":
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_Category{pbrc.ReleaseMetadata_PRE_SOFT_VALIDATE}})
		if err != nil {
			fmt.Printf("Error %v\n", err)
		}
		bid := int32(math.MaxInt32)
		var rec *pbrc.Record
		for _, id := range ids.GetInstanceIds() {
			if id < bid {
				r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
				if err != nil {
					fmt.Printf("Error: %v\n", err)
				}
				if r.GetRecord().GetMetadata().GetFiledUnder() == pbrc.ReleaseMetadata_FILE_12_INCH {
					rec = r.GetRecord()
					bid = id
				}
			}
		}
		if rec != nil {
			fmt.Printf("%v %v [%v]\n", rec.GetRelease().GetId(), rec.GetRelease().GetTitle(), rec.GetRelease().GetInstanceId())
		} else {
			fmt.Printf("Cannot find record for sleeving\n")
		}
	case "sevensleeve":
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_Category{pbrc.ReleaseMetadata_PRE_SOFT_VALIDATE}})
		if err != nil {
			fmt.Printf("Error %v\n", err)
		}
		bid := int32(math.MaxInt32)
		var rec *pbrc.Record
		for _, id := range ids.GetInstanceIds() {
			if id < bid {
				r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
				if err != nil {
					fmt.Printf("Error: %v\n", err)
				}
				if r.GetRecord().GetMetadata().GetFiledUnder() == pbrc.ReleaseMetadata_FILE_7_INCH {
					rec = r.GetRecord()
					bid = id
				}
			}
		}
		if rec != nil {
			fmt.Printf("%v %v [%v]\n", rec.GetRelease().GetId(), rec.GetRelease().GetTitle(), rec.GetRelease().GetInstanceId())
		} else {
			fmt.Printf("Cannot find record for sleeving\n")
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
		i, _ := strconv.ParseInt(os.Args[2], 10, 32)
		rec, err := registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Reason: "forcing sync from cli", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{LastCache: 1, LastSyncTime: 1, LastSalePriceUpdate: 1}}})
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
		rec, err := registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Reason: "CLI-spfolder", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)},
			Metadata: &pbrc.ReleaseMetadata{
				RecordWidth: float32(f),
				//Sleeve:      pbrc.ReleaseMetadata_VINYL_STORAGE_DOUBLE_FLAP,
			}}})
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	case "budget":
		i, _ := strconv.Atoi(os.Args[2])
		rec, err := registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Reason: "CLI-spfolder", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)},
			Metadata: &pbrc.ReleaseMetadata{
				PurchaseBudget: os.Args[3],
				//Sleeve:      pbrc.ReleaseMetadata_VINYL_STORAGE_DOUBLE_FLAP,
			}}})
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	case "salebudget":
		i, _ := strconv.Atoi(os.Args[2])
		rec, err := registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Reason: "CLI-spfolder", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)},
			Metadata: &pbrc.ReleaseMetadata{
				SaleBudget: os.Args[3],
				//Sleeve:      pbrc.ReleaseMetadata_VINYL_STORAGE_DOUBLE_FLAP,
			}}})
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	case "fwidth":
		i, _ := strconv.Atoi(os.Args[2])
		rec, err := registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Reason: "CLI-spfolder", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)},
			Metadata: &pbrc.ReleaseMetadata{
				Sleeve: pbrc.ReleaseMetadata_VINYL_STORAGE_DOUBLE_FLAP,
			}}})
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec.GetUpdated().GetMetadata())
	case "cwidth":
		i, _ := strconv.Atoi(os.Args[2])
		f, _ := strconv.ParseFloat(os.Args[3], 32)
		rec, err := registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Reason: "CLI-spfolder", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)},
			Metadata: &pbrc.ReleaseMetadata{
				RecordWidth: float32(f),
				Sleeve:      pbrc.ReleaseMetadata_VINYL_STORAGE_NO_INNER,
			}}})
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
	case "is_outsize":
		i, _ := strconv.Atoi(os.Args[2])
		rec, err := registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Reason: "CLI-spfolder", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{RecordWidth: -1, GoalFolder: 242017, FiledUnder: pbrc.ReleaseMetadata_FILE_OUTSIZE}}})
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
	case "is_unknown":
		i, _ := strconv.Atoi(os.Args[2])
		rec, err := registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Reason: "CLI-spfolder", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{FiledUnder: -1}}})
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
		i, _ := strconv.ParseInt(os.Args[2], 10, 32)
		f, _ := strconv.ParseInt(os.Args[3], 10, 32)
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
		rec, err := registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Reason: "force_sale", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{SaleState: pbgd.SaleState_FOR_SALE}}})
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
		up := &pbrc.UpdateRecordRequest{Reason: "cli-sellrequest", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{SaleState: pbgd.SaleState_FOR_SALE, SetRating: -1, MoveFolder: 673768, Category: pbrc.ReleaseMetadata_STAGED_TO_SELL}}}
		rec, err := registry.UpdateRecord(ctx, up)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	case "get_cleaned":
		recs, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_UpdateTime{0}})
		if err != nil {
			log.Fatalf("Bad read: %v", err)
		}

		for _, id := range recs.GetInstanceIds() {
			rec, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				log.Fatalf("Bad rec: %v", err)
			}
			if rec.GetRecord().GetMetadata().GetLastCleanDate() > 0 {
				fmt.Printf("%v %v\n", id, rec.GetRecord().GetMetadata().GetLastCleanDate())
			}
		}
	case "get_widths":
		recs, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_UpdateTime{0}})
		if err != nil {
			log.Fatalf("Bad read: %v", err)
		}

		for _, id := range recs.GetInstanceIds() {
			rec, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				log.Fatalf("bad get record: %v", err)
			}
			if rec.GetRecord().GetMetadata().GetRecordWidth() > 0 {
				fmt.Printf("./gram width %v %v\n", id, rec.GetRecord().GetMetadata().GetRecordWidth())
			}
		}
	case "get_keeps":
		recs, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_UpdateTime{0}})
		if err != nil {
			log.Fatalf("Bad read: %v", err)
		}

		for _, id := range recs.GetInstanceIds() {
			rec, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				log.Fatalf("bad get record: %v", err)
			}
			if rec.GetRecord().GetMetadata().GetKeep() != pbrc.ReleaseMetadata_KEEP_UNKNOWN {
				fmt.Printf("./gram keep %v %v\n", id, convertKeep(rec.GetRecord().GetMetadata().GetKeep()))
			}
		}
	case "get_goal_folders":
		recs, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_UpdateTime{0}})
		if err != nil {
			log.Fatalf("Bad read: %v", err)
		}

		for _, id := range recs.GetInstanceIds() {
			rec, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				log.Fatalf("bad get record: %v", err)
			}
			switch rec.GetRecord().GetMetadata().GetGoalFolder() {
			case 242017:
				fmt.Printf("./gram goalfolder %v %v\n", id, "12 Inches")
			case 1727264:
				fmt.Printf("./gram goalfolder %v %v\n", id, "Parents")
			case 267116:
				fmt.Printf("./gram goalfolder %v %v\n", id, "7 Inches")
			case 565206:
				fmt.Printf("./gram goalfolder %v %v\n", id, "Tapes")
			case 1782105:
				fmt.Printf("./gram goalfolder %v %v\n", id, "Bandcap")
			case 242018:
				fmt.Printf("./gram goalfolder %v %v\n", id, "CDs")
			case 1708299:
				fmt.Printf("./gram goalfolder %v %v\n", id, "Limbo")
			case 1607992:
				fmt.Printf("./gram goalfolder %v %v\n", id, "12 Inches")
			case 1613206:
				fmt.Printf("./gram goalfolder %v %v\n", id, "Limbo")
			case 716318:
				fmt.Printf("./gram goalfolder %v %v\n", id, "The Fall")
			case 1435521:
				fmt.Printf("./gram goalfolder %v %v\n", id, "12 Inches")
			case 488127:
				fmt.Printf("./gram goalfolder %v %v\n", id, "Limbo")
			case 288751:
				fmt.Printf("./gram goalfolder %v %v\n", id, "CDs")
			case 267115:
				fmt.Printf("./gram goalfolder %v %v\n", id, "12 Inches")
			case 466902:
				fmt.Printf("./gram goalfolder %v %v\n", id, "12 Inches")
			case 3903712:
				fmt.Printf("./gram goalfolder %v %v\n", id, "Natalie")
			case 857449:
				fmt.Printf("./gram goalfolder %v %v\n", id, "Outside")
			case 2274270:
				fmt.Printf("./gram goalfolder %v %v\n", id, "Computer")
			case 1694651:
				fmt.Printf("./gram goalfolder %v %v\n", id, "CDs")
			case 472403:
				fmt.Printf("./gram goalfolder %v %v\n", id, "7 Inches")
			case 2259637:
				fmt.Printf("./gram goalfolder %v %v\n", id, "Limbo")
			case 1191108:
				fmt.Printf("./gram goalfolder %v %v\n", id, "12 Inches")
			case 1456851:
				fmt.Printf("./gram goalfolder %v %v\n", id, "12 Inches")
			case 1409151:
				fmt.Printf("./gram goalfolder %v %v\n", id, "12 Inches")
			case 1419704:
				fmt.Printf("./gram goalfolder %v %v\n", id, "Tapes")
			case 3007807:
				fmt.Printf("./gram goalfolder %v %v\n", id, "CDs")
			default:
				fmt.Printf("./gram goalfolder %v %v\n", id, "Limbo")
			}
		}
	case "set_arrived":
		recs, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_UpdateTime{0}})
		if err != nil {
			log.Fatalf("Bad read: %v", err)
		}

		for i, id := range recs.GetInstanceIds() {
			rec, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				log.Fatalf("Bad rec: %v", err)
			}

			if rec.Record.GetMetadata().GetBoxState() == pbrc.ReleaseMetadata_IN_45S_BOX {
				if rec.GetRecord().GetMetadata().GetDateArrived() == 0 {
					up := &pbrc.UpdateRecordRequest{Reason: "cli-updatearrived",
						Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(id)},
							Metadata: &pbrc.ReleaseMetadata{DateArrived: rec.GetRecord().GetMetadata().GetDateAdded()}}}
					_, err := registry.UpdateRecord(ctx, up)
					if err != nil {
						log.Fatalf("Error: %v", err)
					}
					fmt.Printf("Update: %v/%v: %v\n", i, len(recs.GetInstanceIds()), id)
				}
			}
		}
	case "update_partents":
		recs, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_UpdateTime{0}})
		if err != nil {
			log.Fatalf("Bad read: %v", err)
		}

		for i, id := range recs.GetInstanceIds() {
			rec, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				log.Fatalf("Bad rec: %v", err)
			}

			if rec.Record.GetMetadata().GetGoalFolder() == 6268933 {
				_, err = registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: id}, Metadata: &pbrc.ReleaseMetadata{WasParents: true, DateArrived: time.Now().Unix()}}})
				if err != nil {
					log.Fatalf("Bad update: %v", err)
				}
				fmt.Printf("%v/%v\n", i, len(recs.GetInstanceIds()))
			}
		}
	case "adjust":
		recs, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_UpdateTime{0}})
		if err != nil {
			log.Fatalf("Bad read: %v", err)
		}

		conn, err := utils.LFDialServer(ctx, "dstore")
		if err != nil {
			log.Fatalf("Bad dial: %v", err)
		}
		client := dspb.NewDStoreServiceClient(conn)
		res, err := client.Read(ctx, &dspb.ReadRequest{Key: fmt.Sprintf("%v/%v", "github.com/brotherlogic/queue/queues", "record_fanout")})
		if err != nil {
			log.Fatalf("Err: %v", err)
		}

		if res.GetConsensus() < 0.5 {
			log.Fatalf("could not get read consensus (%v)", res.GetConsensus())
		}

		queue := &qpb.Queue{}
		err = proto.Unmarshal(res.GetValue().GetValue(), queue)
		if err != nil {
			log.Fatalf("Err: %v", err)
		}

		conn2, err := utils.LFDialServer(ctx, "queue")
		if err != nil {
			log.Fatalf("Bad dial: %v", err)
		}
		client2 := qpb.NewQueueServiceClient(conn2)

		for _, rec := range recs.GetInstanceIds() {
			found := false
			for _, entry := range queue.GetEntries() {
				if entry.GetKey() == fmt.Sprintf("%v", rec) {
					found = true
				}
			}

			if !found {
				fmt.Printf("Not found: %v\n", rec)
				upup := &rfpb.FanoutRequest{
					InstanceId: rec,
				}
				data, _ := proto.Marshal(upup)
				client2.AddQueueItem(ctx, &qpb.AddQueueItemRequest{
					Key:       fmt.Sprintf("%v", rec),
					QueueName: "record_fanout",
					RunTime:   time.Now().Unix(),
					Payload:   &google_protobuf.Any{Value: data},
				})
			}
		}

	case "sold_offline":
		i, _ := strconv.ParseInt(os.Args[2], 10, 32)
		up := &pbrc.UpdateRecordRequest{Reason: "cli-sellrequest", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{SaleState: pbgd.SaleState_SOLD_OFFLINE, SoldDate: time.Now().Unix(), SoldPrice: 1}}}
		rec, err := registry.UpdateRecord(ctx, up)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	case "gprice":
		i, _ := strconv.Atoi(os.Args[2])
		up := &pbrc.GetPriceRequest{Id: int32(i)}
		rec, err := registry.GetPrice(ctx, up)
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
	case "prev":
		i, _ := strconv.Atoi(os.Args[2])
		up := &pbrc.UpdateRecordRequest{Reason: "prev", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{Category: pbrc.ReleaseMetadata_PRE_VALIDATE}}}
		rec, err := registry.UpdateRecord(ctx, up)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	case "setp":
		i, _ := strconv.Atoi(os.Args[2])
		up := &pbrc.UpdateRecordRequest{Reason: "setting parents", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{LastListenTime: -1, WasParents: true, Category: pbrc.ReleaseMetadata_IN_COLLECTION}}}
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
	case "boxset":
		i, _ := strconv.Atoi(os.Args[2])
		up := &pbrc.UpdateRecordRequest{Reason: "Setting box", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{Sleeve: pbrc.ReleaseMetadata_BOX_SET}}}
		rec, err := registry.UpdateRecord(ctx, up)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	case "fixed":
		i, _ := strconv.Atoi(os.Args[2])
		up := &pbrc.UpdateRecordRequest{Reason: "Setting box", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{Sleeve: pbrc.ReleaseMetadata_FIXED}}}
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
		i2, _ := strconv.ParseInt(os.Args[3], 10, 64)
		up := &pbrc.UpdateRecordRequest{Reason: "cli-addsale", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{SaleId: int64(i2)}}}
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
		up := &pbrc.UpdateRecordRequest{Reason: "cli-parents", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{Category: pbrc.ReleaseMetadata_PARENTS}}}
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
	case "hard_reset":
		i, _ := strconv.Atoi(os.Args[2])
		up := &pbrc.UpdateRecordRequest{Reason: "cli-hard-teset", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{Category: pbrc.ReleaseMetadata_PURCHASED}}}
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
	case "no_keep":
		i, _ := strconv.Atoi(os.Args[2])
		up := &pbrc.UpdateRecordRequest{Reason: "rccli-keep", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{Keep: pbrc.ReleaseMetadata_NOT_KEEPER}}}
		rec, err := registry.UpdateRecord(ctx, up)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	case "no_digital":
		i, _ := strconv.Atoi(os.Args[2])
		up := &pbrc.UpdateRecordRequest{Reason: "rccli-keep", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{DigitalAvailability: pbrc.ReleaseMetadata_NO_DIGITAL}}}
		rec, err := registry.UpdateRecord(ctx, up)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	case "has_digital":
		i, _ := strconv.Atoi(os.Args[2])
		up := &pbrc.UpdateRecordRequest{Reason: "rccli-keep", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{DigitalAvailability: pbrc.ReleaseMetadata_DIGITAL_AVAILABLE}}}
		rec, err := registry.UpdateRecord(ctx, up)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Updated: %v", rec)
	case "digital_keep":
		i, _ := strconv.Atoi(os.Args[2])
		up := &pbrc.UpdateRecordRequest{Reason: "rccli-keep", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{Keep: pbrc.ReleaseMetadata_DIGITAL_KEEPER}}}
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

			DateArrived: -1}}}
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
	case "adjust_id":
		i, _ := strconv.Atoi(os.Args[2])
		i2, _ := strconv.Atoi(os.Args[3])
		up := &pbrc.UpdateRecordRequest{Reason: "cli-arrived", Update: &pbrc.Record{Release: &pbgd.Release{Id: int32(i2), InstanceId: int32(i)}}}
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
	case "old":
		i, _ := strconv.ParseInt(os.Args[2], 10, 32)
		up := &pbrc.UpdateRecordRequest{Reason: "cli-ready", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(i)}, Metadata: &pbrc.ReleaseMetadata{AccountingYear: 2022}}}
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
