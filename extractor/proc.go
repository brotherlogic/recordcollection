package main

import (
	"bufio"
	"log"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/brotherlogic/goserver/utils"
	"google.golang.org/grpc"

	pbgd "github.com/brotherlogic/godiscogs"
	pbrc "github.com/brotherlogic/recordcollection/proto"

	//Needed to pull in gzip encoding init
	_ "google.golang.org/grpc/encoding/gzip"
)

func main() {
	f := os.Args[1]

	set := make(map[int32]int64)

	file, err := os.Open(f)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	idMatcher, err := regexp.Compile("coll.id=\"([\\d].*?)\"")
	if err != nil {
		log.Fatal(err)
	}

	dateMatcher, err := regexp.Compile("span.title=\"(.*?)\"")
	if err != nil {
		log.Fatal(err)
	}

	scanner := bufio.NewScanner(file)
	id := int32(0)
	datestr := ""
	for scanner.Scan() {
		str := idMatcher.FindStringSubmatch(scanner.Text())
		if len(str) > 0 {
			val, err := strconv.Atoi(str[1])
			if err != nil {
				log.Fatalf("Errr %v", err)
			}
			id = int32(val)
		}

		dstr := dateMatcher.FindStringSubmatch(scanner.Text())
		if len(dstr) > 0 {
			datestr = dstr[1]
		}

		if id > 0 && len(datestr) > 0 {
			t, err := time.Parse("02-Jan-06 03:04 PM", datestr)
			if err != nil {
				log.Fatalf("Errrror %v", err)
			}
			set[id] = t.Unix()
			id = 0
			datestr = ""
		}
	}

	host, port, err := utils.Resolve("recordcollection")

	if err != nil {
		log.Fatalf("Unable to locate recordcollection server")
	}

	conn, _ := grpc.Dial(host+":"+strconv.Itoa(int(port)), grpc.WithInsecure())
	defer conn.Close()

	registry := pbrc.NewRecordCollectionServiceClient(conn)

	ctx, cancel := utils.BuildContext("recordcollectioncli-"+os.Args[1], "recordcollection")
	defer cancel()

	for id, date := range set {
		update := &pbrc.UpdateRecordRequest{Update: &pbrc.Record{Release: &pbgd.Release{InstanceId: id}, Metadata: &pbrc.ReleaseMetadata{DateAdded: date}}}

		_, err := registry.UpdateRecord(ctx, update)
		if err != nil {
			log.Fatalf("Error %v", err)
		}
	}
}
