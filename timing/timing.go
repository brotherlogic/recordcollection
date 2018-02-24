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
	conn, err := grpc.Dial(host+":"+strconv.Itoa(port), grpc.WithInsecure(), grpc.WithDefaultCallOptions(grpc.UseCompressor("gzip")))
	defer conn.Close()
	if err != nil {
		log.Fatalf("Unable to dial : %v", err)
	}

	client := pbrc.NewRecordCollectionServiceClient(conn)
	t := time.Now()
	recs, err := client.GetRecords(context.Background(), &pbrc.GetRecordsRequest{Filter: &pbrc.Record{}}, grpc.MaxCallRecvMsgSize(1024*1024*1024))
	if err != nil {
		log.Fatalf("Error getting records: %v", err)
	}

	for _, r := range recs.GetRecords() {
		if r.GetMetadata().GetDateAdded() == 0 {
			r.GetMetadata().DateAdded = time.Now().Unix()
			_, err2 := client.UpdateRecord(context.Background(), &pbrc.UpdateRecordRequest{Update: r})
			if err2 != nil {
				log.Fatalf("ERRRR: %v", err2)
			}
		}
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
	recs, err := client.GetRecords(context.Background(), &pbrc.GetRecordsRequest{Filter: &pbrc.Record{Release: &pbd.Release{FolderId: 268147}}}, grpc.UseCompressor("gzip"), grpc.MaxCallRecvMsgSize(1024*1024*1024))
	if err != nil {
		log.Fatalf("Error getting records: %v", err)
	}

	count := 0
	for _, rc := range recs.GetRecords() {
		if len(rc.GetRelease().GetFormats()) > 10 {
			count++
		}
	}

	fmt.Printf("Got %v (%v) records in %v -> %v\n", len(recs.GetRecords()), count, time.Now().Sub(t), recs.GetInternalProcessingTime())
}

func testReadSubsetStripped() {
	host, port := getIP("recordcollection")
	conn, err := grpc.Dial(host+":"+strconv.Itoa(port), grpc.WithInsecure())
	defer conn.Close()
	if err != nil {
		log.Fatalf("Unable to dial : %v", err)
	}

	client := pbrc.NewRecordCollectionServiceClient(conn)
	t := time.Now()
	recs, err := client.GetRecords(context.Background(), &pbrc.GetRecordsRequest{Strip: true, Filter: &pbrc.Record{Release: &pbd.Release{FolderId: 268147}}}, grpc.UseCompressor("gzip"), grpc.MaxCallRecvMsgSize(1024*1024*1024))
	if err != nil {
		log.Fatalf("Error getting records: %v", err)
	}

	count := 0
	for _, rc := range recs.GetRecords() {
		if len(rc.GetRelease().GetFormats()) > 10 {
			count++
		}
	}

	fmt.Printf("Got %v (%v) records in %v -> %v\n", len(recs.GetRecords()), count, time.Now().Sub(t), recs.GetInternalProcessingTime())
}

func main() {
	testReadSubset()
	testReadSubsetStripped()
}
