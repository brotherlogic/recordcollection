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
	qpb "github.com/brotherlogic/queue/proto"
	rapb "github.com/brotherlogic/recordadder/proto"
	pbrc "github.com/brotherlogic/recordcollection/proto"
	fopb "github.com/brotherlogic/recordfanout/proto"
	pbrs "github.com/brotherlogic/recordscores/proto"
	ro "github.com/brotherlogic/recordsorganiser/sales"
	google_protobuf "github.com/golang/protobuf/ptypes/any"

	//Needed to pull in gzip encoding init

	"google.golang.org/grpc/codes"
	_ "google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

func main() {
	ctx, cancel := utils.ManualContext("recordcollectioncli-"+os.Args[1], time.Hour)
	defer cancel()

	conn, err := utils.LFDialServer(ctx, "recordcollection")
	if err != nil {
		log.Fatalf("Cannot reach rc: %v", err)
	}
	defer conn.Close()

	registry := pbrc.NewRecordCollectionServiceClient(conn)

	switch os.Args[1] {
	case "cleaning":
		ctx, cancel := utils.ManualContext("recordcollectioncli-"+os.Args[1], time.Hour*24)
		defer cancel()

		fmt.Printf("Checking Listening Pile\n")
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_FolderId{int32(3282985)}})
		if err != nil {
			log.Fatalf("Bad request: %v", err)
		}
		fmt.Printf("FOUND %v\n", len(ids.GetInstanceIds()))
		for _, id := range ids.GetInstanceIds() {
			r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				log.Fatalf("Bad Request: %v", err)
			}

			fmt.Printf("%v -> %v\n", r.GetRecord().GetRelease().GetInstanceId(), r.GetRecord().GetRelease().GetTitle())
			if r.GetRecord().GetMetadata().GetBoxState() == pbrc.ReleaseMetadata_IN_THE_BOX {
				_, err := registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Reason: "movetobox", Update: &pbrc.Record{
					Release:  &pbgd.Release{InstanceId: id},
					Metadata: &pbrc.ReleaseMetadata{MoveFolder: 3282985},
				}})
				fmt.Printf("%v\n", err)
			}
		}
	case "label":
		lblnm, _ := strconv.Atoi(os.Args[2])
		ctx, cancel := utils.ManualContext("recordcollectioncli-"+os.Args[1], time.Hour*24)
		defer cancel()

		fmt.Printf("Checking Listening Pile\n")
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_FolderId{int32(812802)}})
		if err != nil {
			log.Fatalf("Bad request: %v", err)
		}
		for _, id := range ids.GetInstanceIds() {
			r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				log.Fatalf("Bad Request: %v", err)
			}

			for _, label := range r.GetRecord().GetRelease().GetLabels() {
				if label.GetId() == int32(lblnm) {
					fmt.Printf("Found %v\n", r.GetRecord().GetRelease().GetTitle())
					return
				}
			}
		}
		fmt.Printf("Checked %v records, no dice\n", len(ids.GetInstanceIds()))

		fmt.Printf("Checking Listening Box\n")
		ids, err = registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_FolderId{int32(673768)}})
		if err != nil {
			log.Fatalf("Bad request: %v", err)
		}
		for _, id := range ids.GetInstanceIds() {
			r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				log.Fatalf("Bad Request: %v", err)
			}

			for _, label := range r.GetRecord().GetRelease().GetLabels() {
				if label.GetId() == int32(lblnm) {
					fmt.Printf("Found %v\n", r.GetRecord().GetRelease().GetTitle())
					return
				}
			}
		}
		fmt.Printf("Checked %v records, no dice\n", len(ids.GetInstanceIds()))

		fmt.Printf("Checking Adding Pile\n")
		conn, err = utils.LFDialServer(ctx, "recordadder")
		if err != nil {
			log.Fatalf("Bad dial: %v", err)
		}
		client := rapb.NewAddRecordServiceClient(conn)
		res, err := client.ListQueue(ctx, &rapb.ListQueueRequest{})
		if err != nil {
			log.Fatalf("Bad request: %v", err)
		}
		for _, re := range res.GetRequests() {
			r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{ReleaseId: re.GetId()})
			if err != nil {
				log.Fatalf("Bad Request: %v", err)
			}

			for _, label := range r.GetRecord().GetRelease().GetLabels() {
				if label.GetId() == int32(lblnm) {
					fmt.Printf("Found %v\n", r.GetRecord().GetRelease().GetTitle())
					return
				}
			}
		}
		fmt.Printf("Checked %v records, no dice\n", len(res.GetRequests()))
	case "print_all":
		ctx, cancel := utils.ManualContext("recordcollectioncli-"+os.Args[1], time.Hour*24)
		defer cancel()

		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_All{true}})
		if err != nil {
			log.Fatalf("Bad query: %v", err)
		}

		fmt.Printf("Read %v records\n", len(ids.GetInstanceIds()))

		for _, id := range ids.GetInstanceIds() {
			rec, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				log.Fatalf("Bad read: %v", err)
			}
			//fmt.Printf("%v. [%v] %v\n", rec.GetRecord().GetMetadata().GetDateAdded(), rec.GetRecord().GetMetadata().GetInstanceId(), rec.GetRecord().GetRelease().GetTitle())

			if rec.GetRecord().GetRelease().GetFolderId() == 3380098 {
				conn, err := utils.LFDialServer(ctx, "queue")
				if err != nil {
					log.Fatalf("Unable to dial: %v", err)
				}
				defer conn.Close()

				client := qpb.NewQueueServiceClient(conn)
				update := &fopb.FanoutRequest{InstanceId: rec.GetRecord().GetRelease().GetInstanceId()}
				data, _ := proto.Marshal(update)
				res, err := client.AddQueueItem(ctx, &qpb.AddQueueItemRequest{
					QueueName: "record_fanout",
					RunTime:   int64(time.Now().Unix()),
					Payload:   &google_protobuf.Any{Value: data},
					Key:       fmt.Sprintf("%v", rec.GetRecord().GetRelease().GetInstanceId()),
				})
				fmt.Printf("%v and %v from %v\n", res, err, rec.GetRecord().GetRelease().GetInstanceId())
				//log.Fatalf("Aha")
			}
		}
	case "categories":
		ctx, cancel := utils.ManualContext("recordcollectioncli-"+os.Args[1], time.Hour*24)
		defer cancel()

		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_All{true}})
		if err != nil {
			log.Fatalf("Bad query: %v", err)
		}

		fmt.Printf("Read %v records\n", len(ids.GetInstanceIds()))

		categories := make(map[string]int)
		for _, id := range ids.GetInstanceIds() {
			rec, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				log.Fatalf("Bad read: %v", err)
			}
			if rec.Record.GetMetadata().GetGoalFolder() != 242018 &&
				rec.Record.GetMetadata().GetGoalFolder() != 1782105 &&
				rec.Record.GetMetadata().GetGoalFolder() != 2274270 &&
				rec.Record.GetMetadata().GetGoalFolder() != 1727264 &&
				rec.Record.GetMetadata().GetGoalFolder() != 1727264 &&
				rec.GetRecord().GetMetadata().GetCategory() != pbrc.ReleaseMetadata_LISTED_TO_SELL &&
				rec.GetRecord().GetMetadata().GetCategory() != pbrc.ReleaseMetadata_STALE_SALE &&
				rec.GetRecord().GetMetadata().GetCategory() != pbrc.ReleaseMetadata_SOLD_ARCHIVE &&
				rec.GetRecord().GetMetadata().GetCategory() != pbrc.ReleaseMetadata_PREPARE_TO_SELL &&
				rec.GetRecord().GetMetadata().GetCategory() != pbrc.ReleaseMetadata_SOLD_OFFLINE {
				categories[fmt.Sprintf("%v", rec.Record.GetMetadata().GetCategory())]++
			}
		}
		sum := 0
		for cat, count := range categories {
			fmt.Printf("%v - %v\n", count, cat)
			sum += count
		}
		fmt.Printf("Total %v\n", sum)
	case "get_all":
		ctx, cancel := utils.ManualContext("recordcollectioncli-"+os.Args[1], time.Hour*24)
		defer cancel()

		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_All{true}})
		if err != nil {
			log.Fatalf("Bad query: %v", err)
		}

		fmt.Printf("Read %v records\n", len(ids.GetInstanceIds()))

		for i, id := range ids.GetInstanceIds() {
			rec, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				log.Fatalf("Unable to read record: %v", err)
			}

			fmt.Printf("%v (%v) %v\n", id, i, rec.GetRecord().GetRelease().Title)
		}
	case "pre_valid":
		ctx, cancel := utils.ManualContext("recordcollectioncli-"+os.Args[1], time.Hour*24)
		defer cancel()

		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_All{true}})
		if err != nil {
			log.Fatalf("Bad query: %v", err)
		}

		fmt.Printf("Read %v records\n", len(ids.GetInstanceIds()))

		for i, id := range ids.GetInstanceIds() {
			rec, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				log.Fatalf("Unable to read record: %v", err)
			}

			if rec.GetRecord().GetMetadata().GetCategory() == pbrc.ReleaseMetadata_PRE_VALIDATE {
				fmt.Printf("%v. %v [%v] \n", i, rec.GetRecord().Release.GetTitle(), rec.GetRecord().GetRelease().GetInstanceId())
			}

		}
	case "pre_high":
		ctx, cancel := utils.ManualContext("recordcollectioncli-"+os.Args[1], time.Hour*24)
		defer cancel()

		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_All{true}})
		if err != nil {
			log.Fatalf("Bad query: %v", err)
		}

		fmt.Printf("Read %v records\n", len(ids.GetInstanceIds()))

		for i, id := range ids.GetInstanceIds() {
			rec, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				log.Fatalf("Unable to read record: %v", err)
			}

			if rec.GetRecord().GetRelease().GetFolderId() == 673768 {
				if rec.GetRecord().GetMetadata().GetCategory() == pbrc.ReleaseMetadata_STAGED {
					fmt.Printf("%v. %v [%v] -> %v\n", time.Since(time.Unix(rec.GetRecord().GetMetadata().GetLastListenTime(), 0)).Hours(), i, rec.GetRecord().Release.GetTitle(), rec.Record.GetMetadata().GetCategory())
				}
			}
		}
	case "soft":
		ctx, cancel := utils.ManualContext("recordcollectioncli-"+os.Args[1], time.Hour*24)
		defer cancel()

		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_Category{pbrc.ReleaseMetadata_PRE_SOFT_VALIDATE}})
		if err != nil {
			log.Fatalf("Bad query: %v", err)
		}

		fmt.Printf("Read %v records\n", len(ids.GetInstanceIds()))

		for i, id := range ids.GetInstanceIds() {
			rec, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				log.Fatalf("Unable to read record: %v", err)
			}

			fmt.Printf("%v. %v [%v] -> %v\n", time.Since(time.Unix(rec.GetRecord().GetMetadata().GetLastListenTime(), 0)).Hours(), i, rec.GetRecord().Release.GetTitle(), rec.Record.GetMetadata().GetCategory())
		}
	case "all_cds":
		ctx, cancel := utils.ManualContext("recordcollectioncli-"+os.Args[1], time.Hour*24)
		defer cancel()

		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_All{true}})
		if err != nil {
			log.Fatalf("Bad query: %v", err)
		}

		fmt.Printf("Read %v records\n", len(ids.GetInstanceIds()))

		for _, id := range ids.GetInstanceIds() {
			rec, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				log.Fatalf("Unable to read record: %v", err)
			}

			iscd := false
			for _, format := range rec.GetRecord().GetRelease().GetFormats() {
				if format.GetName() == "CD" || format.GetName() == "CDr" {
					iscd = true
				}
			}

			if iscd {
				fmt.Printf("%v - %v [%v]\n", rec.GetRecord().GetRelease().GetInstanceId(), rec.GetRecord().GetRelease().GetTitle(), rec.GetRecord().GetMetadata().GetCategory())
			}
		}
	case "align":
		ctx, cancel := utils.ManualContext("recordcollectioncli-"+os.Args[1], time.Hour*24)
		defer cancel()

		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_UpdateTime{0}})
		if err != nil {
			log.Fatalf("Bad query: %v", err)
		}

		fmt.Printf("Read %v records\n", len(ids.GetInstanceIds()))

		for _, id := range ids.GetInstanceIds() {
			rec, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})

			if err != nil {
				log.Fatalf("Boing: %v", err)
			}

			if rec.GetRecord().GetRelease().GetFolderId() == 812802 {
				_, err := registry.CommitRecord(ctx, &pbrc.CommitRecordRequest{InstanceId: id})
				fmt.Printf("%v -> %v\n", id, err)
			}
		}
	case "auditions":
		ctx, cancel := utils.ManualContext("recordcollectioncli-"+os.Args[1], time.Hour*24)
		defer cancel()

		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_UpdateTime{0}})
		if err != nil {
			log.Fatalf("Bad query: %v", err)
		}

		fmt.Printf("Read %v records\n", len(ids.GetInstanceIds()))

		for i, id := range ids.GetInstanceIds() {
			rec, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				if status.Convert(err).Code() == codes.OutOfRange {
					fmt.Printf("%v has been deleted\n", id)
				} else {
					log.Fatalf("Unable to read record: %v", err)
				}
			}

			if rec.GetRecord().GetMetadata().GetLastAudition() > 0 {
				fmt.Printf("%v. %v on %v\n", i, rec.GetRecord().GetRelease().GetTitle(), time.Unix(rec.GetRecord().GetMetadata().GetLastAudition(), 0))
			}
		}
	case "unlistened":
		ctx, cancel := utils.ManualContext("recordcollectioncli-"+os.Args[1], time.Hour*24)
		defer cancel()

		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_UpdateTime{0}})
		if err != nil {
			log.Fatalf("Bad query: %v", err)
		}

		fmt.Printf("Read %v records\n", len(ids.GetInstanceIds()))

		for i, id := range ids.GetInstanceIds() {
			rec, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				log.Fatalf("Unable to read record: %v", err)
			}

			if rec.GetRecord().GetMetadata().GetLastListenTime() == 0 {
				if rec.GetRecord().GetMetadata().GetCategory() != pbrc.ReleaseMetadata_SOLD_ARCHIVE &&
					rec.GetRecord().GetMetadata().GetCategory() != pbrc.ReleaseMetadata_PARENTS {
					fmt.Printf("%v. %v [%v] -> %v\n", i, id, rec.GetRecord().GetRelease().GetTitle(), rec.GetRecord().GetMetadata().GetCategory())
				}
			}
		}
	case "fix_stales":
		ctx, cancel := utils.ManualContext("recordcollectioncli-"+os.Args[1], time.Hour*24)
		defer cancel()

		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_UpdateTime{0}})
		if err != nil {
			log.Fatalf("Bad query: %v", err)
		}

		fmt.Printf("Read %v records\n", len(ids.GetInstanceIds()))

		for i, id := range ids.GetInstanceIds() {
			rec, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				log.Fatalf("Unable to read record: %v", err)
			}

			if rec.GetRecord().GetMetadata().GetCategory() == pbrc.ReleaseMetadata_STALE_SALE {
				fmt.Printf("%v. %v [%v] -> %v\n", i, id, rec.GetRecord().GetRelease().GetTitle(), rec.GetRecord().GetMetadata().GetCategory())
			}
		}
	case "pre_in":
		ctx, cancel := utils.ManualContext("recordcollectioncli-"+os.Args[1], time.Hour*24)
		defer cancel()

		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_UpdateTime{0}})
		if err != nil {
			log.Fatalf("Bad query: %v", err)
		}

		fmt.Printf("Read %v records\n", len(ids.GetInstanceIds()))

		for i, id := range ids.GetInstanceIds() {
			rec, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				log.Fatalf("Unable to read record: %v", err)
			}

			if rec.GetRecord().GetMetadata().GetCategory() == pbrc.ReleaseMetadata_PRE_IN_COLLECTION &&
				(rec.GetRecord().GetMetadata().GetBoxState() == pbrc.ReleaseMetadata_BOX_UNKNOWN || rec.GetRecord().GetMetadata().GetBoxState() == pbrc.ReleaseMetadata_OUT_OF_BOX) {
				fmt.Printf("%v. %v [%v] -> %v\n", i, id, rec.GetRecord().GetRelease().GetTitle(), rec.GetRecord().GetMetadata().GetCategory())
			}
		}
	case "folder":
		ctx, cancel := utils.ManualContext("recordcollectioncli-"+os.Args[1], time.Hour*24)
		defer cancel()

		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_All{true}})
		if err != nil {
			log.Fatalf("Bad query: %v", err)
		}

		fmt.Printf("Read %v records\n", len(ids.GetInstanceIds()))

		categories := make(map[string]int)
		for i, id := range ids.GetInstanceIds() {
			rec, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				log.Fatalf("Bad read: %v", err)
			}

			isTwelve := false
			is45 := false
			isCD := false
			isTape := false
			isDigital := false
			if rec.GetRecord().GetMetadata().GetGoalFolder() != 1782105 &&
				rec.GetRecord().GetMetadata().GetGoalFolder() != 3282985 &&
				rec.GetRecord().GetMetadata().GetGoalFolder() != 3291655 &&
				rec.GetRecord().GetMetadata().GetGoalFolder() != 2274270 &&
				rec.GetRecord().GetMetadata().GetGoalFolder() != 1727264 &&
				rec.GetRecord().GetMetadata().GetGoalFolder() != 3291970 &&
				rec.GetRecord().GetRelease().GetFolderId() != 1782105 &&
				rec.GetRecord().GetRelease().GetFolderId() != 3282985 &&
				rec.GetRecord().GetRelease().GetFolderId() != 3291655 &&
				rec.GetRecord().GetRelease().GetFolderId() != 2274270 &&
				rec.GetRecord().GetRelease().GetFolderId() != 1727264 &&
				rec.GetRecord().GetRelease().GetFolderId() != 1613206 &&
				rec.GetRecord().GetRelease().GetFolderId() != 1708299 &&
				rec.GetRecord().GetRelease().GetFolderId() != 3291970 &&
				rec.GetRecord().GetRelease().GetFolderId() != 488127 {

				for _, format := range rec.GetRecord().GetRelease().GetFormats() {
					if format.Name == "LP" || format.Name == "12\"" || format.Name == "10\"" {
						isTwelve = true
					}
					if format.Name == "7\"" {
						is45 = true
					}
					if format.Name == "Cassette" {
						isTape = true
					}
					if format.Name == "CD" || format.Name == "CDr" {
						isCD = true
					}
					for _, des := range format.GetDescriptions() {
						if des == "LP" || des == "12\"" || des == "10\"" {
							isTwelve = true
						}
						if des == "7\"" {
							is45 = true
						}
						if des == "Cassette" {
							isTape = true
						}
						if des == "CD" || des == "CDr" {
							isCD = true
						}
					}
				}
			} else {
				if rec.GetRecord().GetMetadata().GetGoalFolder() == 1782105 ||
					rec.GetRecord().GetMetadata().GetGoalFolder() == 2274270 ||
					rec.GetRecord().GetMetadata().GetNewBoxState() == pbrc.ReleaseMetadata_IN_DIGITAL_BOX {
					isDigital = true
				}
			}

			if !isTwelve && !is45 && !isCD && !isTape && !isDigital {
				//fmt.Printf("Skipping %v (%v) -> %v\n", rec.GetRecord().GetRelease().GetInstanceId(), rec.GetRecord().GetRelease().GetTitle(), rec.GetRecord().GetRelease().GetFormats())
			} else {

				ctx2, cancel2 := utils.ManualContext("script_set_box-"+os.Args[1], time.Minute)
				defer cancel2()
				conn2, err := utils.LFDialServer(ctx2, "recordcollection")
				if err != nil {
					log.Fatalf("Cannot reach rc: %v", err)
				}
				defer conn2.Close()

				lclient := pbrc.NewRecordCollectionServiceClient(conn2)
				if isTwelve {
					_, err = lclient.UpdateRecord(ctx2, &pbrc.UpdateRecordRequest{Reason: "Boxing",
						Update: &pbrc.Record{
							Release: &pbgd.Release{
								InstanceId: id,
							},
							Metadata: &pbrc.ReleaseMetadata{NewBoxState: pbrc.ReleaseMetadata_IN_THE_BOX, Dirty: true}}})
				} else if is45 {
					_, err = lclient.UpdateRecord(ctx2, &pbrc.UpdateRecordRequest{Reason: "Boxing",
						Update: &pbrc.Record{
							Release: &pbgd.Release{
								InstanceId: id,
							},
							Metadata: &pbrc.ReleaseMetadata{NewBoxState: pbrc.ReleaseMetadata_IN_45S_BOX, Dirty: true}}})

				} else if isCD {
					_, err = lclient.UpdateRecord(ctx2, &pbrc.UpdateRecordRequest{Reason: "Boxing",
						Update: &pbrc.Record{
							Release: &pbgd.Release{
								InstanceId: id,
							},
							Metadata: &pbrc.ReleaseMetadata{NewBoxState: pbrc.ReleaseMetadata_IN_CDS_BOX, Dirty: true}}})

				} else if isTape {
					_, err = lclient.UpdateRecord(ctx2, &pbrc.UpdateRecordRequest{Reason: "Boxing",
						Update: &pbrc.Record{
							Release: &pbgd.Release{
								InstanceId: id,
							},
							Metadata: &pbrc.ReleaseMetadata{NewBoxState: pbrc.ReleaseMetadata_IN_TAPE_BOX, Dirty: true}}})

				} else if isDigital {
					_, err = lclient.UpdateRecord(ctx2, &pbrc.UpdateRecordRequest{Reason: "Boxing",
						Update: &pbrc.Record{
							Release: &pbgd.Release{
								InstanceId: id,
							},
							Metadata: &pbrc.ReleaseMetadata{NewBoxState: pbrc.ReleaseMetadata_IN_DIGITAL_BOX, Dirty: true}}})

				}
				if err != nil {
					log.Printf("Yep: %v", err)
				}
				fmt.Printf("%v / %v. Adding %v to box\n", i, len(ids.GetInstanceIds()), id)
			}
		}
		sum := 0
		for cat, count := range categories {
			fmt.Printf("%v - %v\n", count, cat)
			sum += count
		}
		fmt.Printf("Total %v\n", sum)

	case "runscore":
		ctx, cancel := utils.ManualContext("recordcollectioncli-"+os.Args[1], time.Hour*24)
		defer cancel()

		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_All{true}})
		if err != nil {
			log.Fatalf("Bad query: %v", err)
		}

		fmt.Printf("Read %v records\n", len(ids.GetInstanceIds()))

		count := 0
		count2 := 0
		for i, id := range ids.GetInstanceIds() {
			rec, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				log.Fatalf("Bad read: %v", err)
			}
			if rec.GetRecord().GetMetadata().GetCategory() == pbrc.ReleaseMetadata_PRE_IN_COLLECTION {
				if rec.GetRecord().GetMetadata().GetBoxState() == pbrc.ReleaseMetadata_OUT_OF_BOX || rec.GetRecord().GetMetadata().GetBoxState() == pbrc.ReleaseMetadata_BOX_UNKNOWN {

					ctx2, cancel2 := utils.ManualContext("runscore-getscore", time.Minute)
					defer cancel2()
					conn2, err2 := utils.LFDialServer(ctx2, "recordscores")
					if err2 != nil {
						log.Fatalf("Dial error: %v", err2)
					}
					defer conn2.Close()
					client2 := pbrs.NewRecordScoreServiceClient(conn2)
					scores, err := client2.GetScore(ctx2, &pbrs.GetScoreRequest{InstanceId: id})
					if err != nil {
						log.Fatalf("Bad score get: %v", err)
					}
					bestScore := int32(0)
					bestTime := int64(0)
					for _, score := range scores.GetScores() {
						if score.ScoreTime > bestTime {
							bestTime = score.GetScoreTime()
							bestScore = score.GetRating()
						}
					}

					if bestTime > 0 {
						_, err := registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Reason: "recordcollection-cli_reset_score",
							Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: int32(id)},
								Metadata: &pbrc.ReleaseMetadata{SetRating: int32(bestScore)}}})
						if err != nil {
							log.Fatalf("Error: %v", err)
						}
						fmt.Printf("%v. %v => %v\n", i, rec.GetRecord().GetRelease().GetTitle(), bestScore)
					} else {
						count2++
						fmt.Printf("%v. %v [NO_SCORE]\n", i, rec.GetRecord().GetRelease().GetTitle())
					}

					count++
				}
			}
		}
		fmt.Printf("Found %v records needing a score (%v we have no data on)\n", count, count2)
	case "age":
		ctx, cancel := utils.ManualContext("recordcollectioncli-"+os.Args[1], time.Hour*24)
		defer cancel()

		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_All{All: true}})
		if err != nil {
			log.Fatalf("Bad query: %v", err)
		}

		fmt.Printf("Read %v records\n", len(ids.GetInstanceIds()))

		for i, id := range ids.GetInstanceIds() {
			rec, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				log.Fatalf("Bad read: %v", err)
			}
			fmt.Printf("%v %v. %v [%v] (%v - %v)\n", time.Since(time.Unix(rec.GetRecord().GetMetadata().GetLastListenTime(), 0)).Hours(), i, rec.GetRecord().GetRelease().GetTitle(),
				rec.GetRecord().GetRelease().GetInstanceId(),
				time.Since(time.Unix(rec.GetRecord().GetMetadata().GetLastListenTime(), 0)), rec.GetRecord().GetMetadata().GetCategory())
		}
	case "age_order":
		ctx, cancel := utils.ManualContext("recordcollectioncli-"+os.Args[1], time.Hour*24)
		defer cancel()

		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_All{true}})
		if err != nil {
			log.Fatalf("Bad query: %v", err)
		}

		fmt.Printf("Read %v records\n", len(ids.GetInstanceIds()))

		for i, id := range ids.GetInstanceIds() {
			rec, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				log.Fatalf("Bad read: %v", err)
			}
			fmt.Printf("%v %v. %v [%v] (%v)\n", rec.GetRecord().GetMetadata().GetDateAdded(), i, rec.GetRecord().GetRelease().GetInstanceId(), rec.GetRecord().GetRelease().GetTitle(),
				time.Since(time.Unix(rec.GetRecord().GetMetadata().GetDateAdded(), 0)))
		}

	case "no_width":
		ctx, cancel := utils.ManualContext("recordcollectioncli-"+os.Args[1], time.Hour*24)
		defer cancel()

		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_All{true}})
		if err != nil {
			log.Fatalf("Bad query: %v", err)
		}

		fmt.Printf("Read %v records\n", len(ids.GetInstanceIds()))

		for i, id := range ids.GetInstanceIds() {
			rec, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				log.Fatalf("Bad read: %v", err)
			}
			if rec.Record.GetMetadata().GetRecordWidth() == 0 {
				fmt.Printf("%v. %v - %v\n", i, rec.GetRecord().GetRelease().GetInstanceId(), rec.GetRecord().Release.GetFolderId())
			}
		}

	case "validated":
		ctx, cancel := utils.ManualContext("recordcollectioncli-"+os.Args[1], time.Hour*24)
		defer cancel()

		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_All{true}})
		if err != nil {
			log.Fatalf("Bad query: %v", err)
		}

		fmt.Printf("Read %v records\n", len(ids.GetInstanceIds()))

		count := 0
		for _, id := range ids.GetInstanceIds() {
			rec, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				log.Fatalf("Bad read: %v", err)
			}

			if rec.GetRecord().GetMetadata().GetLastValidate() > 0 {
				found := false
				for _, format := range rec.GetRecord().GetRelease().GetFormats() {
					if strings.Contains(format.Name, "10") {
						found = true
					}
					for _, des := range format.GetDescriptions() {
						if strings.Contains(des, "10") {
							found = true
						}
					}
				}

				if found {
					fmt.Printf("Found %v\n", rec.GetRecord().GetRelease().GetInstanceId())
				}
				count++
			}
		}
		fmt.Printf("%v / %v are validated - %v%%\n", count, len(ids.GetInstanceIds()), 100*count/len(ids.GetInstanceIds()))

	case "stats":
		ctx, cancel := utils.ManualContext("recordcollectioncli-"+os.Args[1], time.Hour*24)
		defer cancel()

		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_All{true}})
		if err != nil {
			log.Fatalf("Bad query: %v", err)
		}

		fmt.Printf("Read %v records\n", len(ids.GetInstanceIds()))

		maxProfit := int32(0)
		maxProfR := int32(0)
		minProfit := int32(20000)
		minProfitR := int32(0)
		avgProfit := int32(0)
		count := int32(0)
		for _, id := range ids.GetInstanceIds() {
			rec, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				log.Fatalf("Bad: %v", err)
			}
			if rec.GetRecord().GetMetadata().GetCategory() == pbrc.ReleaseMetadata_SOLD_ARCHIVE && rec.GetRecord().GetMetadata().GetCost() > 1 {
				profit := rec.GetRecord().GetMetadata().GetSoldPrice() - rec.GetRecord().GetMetadata().GetCost()
				if profit > maxProfit {
					maxProfit = profit
					maxProfR = rec.GetRecord().GetRelease().GetInstanceId()
				}
				if profit < minProfit {
					minProfit = profit
					minProfitR = rec.GetRecord().GetRelease().GetInstanceId()
				}
				avgProfit += profit
				count++
			}
		}
		fmt.Printf("Max: %v (%v)\n", maxProfit, maxProfR)
		fmt.Printf("Min: %v (%v)\n", minProfit, minProfitR)
		fmt.Printf("Avg: %v\n", avgProfit/count)

	case "sales":
		ctx, cancel := utils.ManualContext("recordcollectioncli-"+os.Args[1], time.Hour*24)
		defer cancel()

		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_All{true}})
		if err != nil {
			log.Fatalf("Bad query: %v", err)
		}

		fmt.Printf("Read %v records\n", len(ids.GetInstanceIds()))
		ctx2, cancel2 := utils.ManualContext("recordcollectioncli-"+os.Args[1], time.Hour*24)
		conn2, err := utils.LFDialServer(ctx2, "recordscores")
		if err != nil {
			log.Fatalf("Cannot reach rc: %v", err)
		}

		registry2 := pbrs.NewRecordScoreServiceClient(conn2)

		tSales := int32(0)
		for i, id := range ids.GetInstanceIds() {
			rec, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				log.Fatalf("Err %v", err)
			}

			fmt.Printf("Processing: (%v/%v) %v - %v\n", i, len(ids.GetInstanceIds()), id, rec.GetRecord().GetMetadata().GetCategory())

			if time.Now().Sub(time.Unix(rec.GetRecord().GetMetadata().LastStockCheck, 0)) > 24*time.Hour {
				if rec.GetRecord().GetMetadata().GetCategory() == pbrc.ReleaseMetadata_LISTED_TO_SELL || rec.GetRecord().GetMetadata().GetCategory() == pbrc.ReleaseMetadata_SOLD_ARCHIVE {
					for i := 0; i < 10; i++ {
						up := &pbrc.UpdateRecordRequest{Reason: "org-stock", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: rec.GetRecord().GetRelease().InstanceId}, Metadata: &pbrc.ReleaseMetadata{LastStockCheck: time.Now().Unix()}}}
						rec, err := registry.UpdateRecord(ctx, up)
						if err == nil {
							break
						}
						if err != nil && status.Convert(err).Code() != codes.ResourceExhausted {
							log.Fatalf("Error: %v", err)
						}

						time.Sleep(time.Second * 90)
						fmt.Printf("Retrying in 90: %v", rec)
					}
				}
			}

			if rec.GetRecord().GetMetadata().GetCategory() == pbrc.ReleaseMetadata_SOLD_ARCHIVE {
				valid := false
				scores, err := registry2.GetScore(ctx, &pbrs.GetScoreRequest{InstanceId: id})
				if err != nil {
					log.Fatalf("Err: %v", err)
				}

				for _, score := range scores.GetScores() {
					fmt.Printf(" %v - %v\n", score.GetCategory(), time.Unix(score.GetScoreTime(), 0))
					if score.GetCategory() == pbrc.ReleaseMetadata_SOLD_ARCHIVE && time.Unix(score.GetScoreTime(), 0).Year() == 2021 {
						fmt.Printf("%v. %v - %v\n", i, rec.GetRecord().GetRelease().GetTitle(), rec.Record.GetMetadata().GetSalePrice())
						valid = true
					}
				}

				if valid {
					tSales += rec.Record.GetMetadata().GetSalePrice()
				}
			}
		}
		cancel2()
		conn2.Close()
		fmt.Printf("SALES: %v\n", tSales)

	case "retrospective":
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_UpdateTime{0}})
		if err != nil {
			log.Fatalf("Bad query: %v", err)
		}

		fmt.Printf("Processing %v records\n", len(ids.GetInstanceIds()))
		for _, id := range ids.GetInstanceIds() {
			r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				//Pass
			}

			t := time.Unix(r.GetRecord().GetMetadata().GetDateAdded(), 0)
			if t.Year() == time.Now().Year()-1 {
				cat := r.GetRecord().GetMetadata().GetCategory()
				gf := r.GetRecord().GetMetadata().GetGoalFolder()
				if cat != pbrc.ReleaseMetadata_PARENTS &&
					cat != pbrc.ReleaseMetadata_GOOGLE_PLAY &&
					cat != pbrc.ReleaseMetadata_SOLD_ARCHIVE && gf != 565206 && gf != 1782105 && gf != 242018 {
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

			fmt.Printf("Highest [%v] = %v (%v)\n", *folder, rec.GetRelease().GetTitle(), rec.GetMetadata().GetCurrentSalePrice())
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
			for i := 0; i < len(records); i++ {
				fmt.Printf("%v. {%v} [%v]: %v - %v\n", i, ro.GetScore(records[i]), records[i].GetRelease().GetInstanceId(), records[i].GetRelease().GetArtists()[0].GetName(), records[i].GetRelease().GetTitle())
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
			fmt.Printf("Queried: %v records\n", len(ids.GetInstanceIds()))
			counts := make(map[pbrc.ReleaseMetadata_Category]int)
			for _, id := range ids.GetInstanceIds() {
				fmt.Printf("%v\n", id)
				registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Reason: "cold", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: id}}})
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
	case "finder":
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_All{true}})

		if err == nil {
			count := 0
			without := 0
			for _, id := range ids.GetInstanceIds() {
				//fmt.Printf("Getting record: %v\n", id)
				rec, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
				if err != nil {
					log.Fatalf("Bad get: %v", err)
				}
				if rec.GetRecord().GetRelease().GetFolderId() == 1613206 {
					if rec.GetRecord().GetMetadata().GetSaleId() == 0 {
						fmt.Printf("%v - %v [%v]\n", id, rec.GetRecord().Release.GetTitle(), rec.GetRecord().GetMetadata().GetGoalFolder())
						/*registry = pbrc.NewRecordCollectionServiceClient(conn)
						fmt.Printf("%v - %v (%v)\n", id, rec.GetRecord().GetRelease().GetTitle(), rec.GetRecord().GetMetadata().GetSoldPrice())
						_, err = registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Reason: "ale-update", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: id}}})
						if err != nil {
							log.Fatalf("Bad update:%v", err)
						}*/
					} else {
						count++
						if rec.GetRecord().GetMetadata().GetSoldDate() == 0 {
							conn, err := utils.LFDialServer(ctx, "recordcollection")
							if err != nil {
								log.Fatalf("Cannot reach rc: %v", err)
							}
							defer conn.Close()

							registry = pbrc.NewRecordCollectionServiceClient(conn)
							fmt.Printf("%v - %v (%v)\n", id, rec.GetRecord().GetRelease().GetTitle(), rec.GetRecord().GetMetadata().GetSoldPrice())
							_, err = registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Reason: "ale-update", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: id}}})
							if err != nil {
								log.Fatalf("Bad update:%v", err)
							}
							without++
						}
					}
				}
			}
			fmt.Printf("Found %v/%v legit\n", without, count)
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
	case "first_in":
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_All{true}})
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

			if time.Since(time.Unix(r.GetRecord().GetMetadata().GetDateAdded(), 0)) > time.Hour*24*30*3 {
				fmt.Printf("%v - %v [%v]\n", r.GetRecord().GetMetadata().GetDateAdded(), r.GetRecord().GetRelease().GetInstanceId(), r.GetRecord().GetRelease().GetTitle())
			}

		}

		fmt.Printf("%v, %v\n", time.Unix(lowest, 0), rec)
	case "needs_stock":
		ids, err := registry.QueryRecords(ctx, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_FolderId{int32(812802)}})
		if err != nil {
			log.Fatalf("Bad query: %v", err)
		}

		fmt.Printf("Processing %v records\n", len(ids.GetInstanceIds()))
		for _, id := range ids.GetInstanceIds() {
			r, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: id})
			if err != nil {
				log.Printf("Error: %v", err)
			} else {
				if time.Now().Sub(time.Unix(r.GetRecord().GetMetadata().GetLastStockCheck(), 0)) > time.Hour*6 {
					_, err := registry.UpdateRecord(ctx, &pbrc.UpdateRecordRequest{Reason: "stock push", Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: id}, Metadata: &pbrc.ReleaseMetadata{LastStockCheck: time.Now().Unix()}}})
					if err != nil {
						log.Printf("Cannot update: %v\n", err)
					} else {
						fmt.Printf("[%v] is too old (%v)\n", r.GetRecord().GetRelease().GetInstanceId(), r.GetRecord().GetRelease().GetTitle())
					}
					time.Sleep(time.Second)
				}
			}
		}
		fmt.Printf("Done\n")
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
