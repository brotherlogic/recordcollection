package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/brotherlogic/goserver/utils"
	"github.com/brotherlogic/keystore/client"

	pbd "github.com/brotherlogic/godiscogs"
	pbrc "github.com/brotherlogic/recordcollection/proto"

	//Needed to pull in gzip encoding init
	"google.golang.org/grpc"
	_ "google.golang.org/grpc/encoding/gzip"
)

func getIP(server string) (string, int) {
	t := time.Now()
	h, p, _ := utils.Resolve(server)
	fmt.Printf("GOT %v\n", time.Now().Sub(t))
	return h, int(p)
}

func testRead() {
	t := time.Now()
	client := *keystoreclient.GetClient(getIP)
	rc := &pbrc.RecordCollection{}
	_, b, c := client.Read("/github.com/brotherlogic/recordcollection/collection", rc)
	fmt.Printf("TOOK %v -> %v,%v\n", time.Now().Sub(t), b.GetReadTime(), c)
}

func testReadCollection() {
	host, port := getIP("recordcollection")
	conn, err := grpc.Dial(host+":"+strconv.Itoa(port), grpc.WithInsecure())
	defer conn.Close()
	if err != nil {
		log.Fatalf("Unable to dial : %v", err)
	}

	client := pbrc.NewRecordCollectionServiceClient(conn)
	t := time.Now()
	recs, err := client.GetRecords(context.Background(), &pbrc.GetRecordsRequest{Filter: &pbrc.Record{}})
	if err != nil {
		log.Fatalf("Error getting records: %v", err)
	}

	fmt.Printf("Got %v records in %v\n", len(recs.GetRecords()), time.Now().Sub(t))
}

func testReadSubset() {
	host, port := getIP("recordcollection")
	conn, err := grpc.Dial(host+":"+strconv.Itoa(port), grpc.WithInsecure())
	defer conn.Close()
	if err != nil {
		log.Fatalf("Unable to dial : %v", err)
	}

	client := pbrc.NewRecordCollectionServiceClient(conn)
	t := time.Now()
	recs, err := client.GetRecords(context.Background(), &pbrc.GetRecordsRequest{Force: true, Filter: &pbrc.Record{Release: &pbd.Release{FolderId: 812802}}})
	if err != nil {
		log.Fatalf("Error getting records: %v", err)
	}

	fmt.Printf("Got %v records in %v\n", len(recs.GetRecords()), time.Now().Sub(t))
}

func main() {
	testReadSubset()
}
