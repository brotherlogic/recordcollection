package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/brotherlogic/goserver/utils"
	"github.com/brotherlogic/keystore/client"

	pbgd "github.com/brotherlogic/discovery/proto"
	pbd "github.com/brotherlogic/godiscogs"
	pbrc "github.com/brotherlogic/recordcollection/proto"

	//Needed to pull in gzip encoding init
	"google.golang.org/grpc"
	_ "google.golang.org/grpc/encoding/gzip"
)

func doDial(entry *pbgd.RegistryEntry) (*grpc.ClientConn, error) {
	return grpc.Dial(entry.Ip+":"+strconv.Itoa(int(entry.Port)), grpc.WithInsecure())
}

func dialMaster(server string) (*grpc.ClientConn, error) {
	ip, port, err := utils.Resolve(server, "rc-timing")
	if err != nil {
		return nil, err
	}

	return doDial(&pbgd.RegistryEntry{Ip: ip, Port: port})
}

func testRead() {
	t := time.Now()
	client := *keystoreclient.GetClient(dialMaster)
	rc := &pbrc.RecordCollection{}
	_, b, c := client.Read(context.Background(), "/github.com/brotherlogic/recordcollection/collection", rc)
	fmt.Printf("TOOK %v -> %v,%v\n", time.Now().Sub(t), b.GetReadTime(), c)
}

func testReadCollection() {
	conn, err := dialMaster("recordcollection")
	defer conn.Close()
	if err != nil {
		log.Fatalf("Unable to dial : %v", err)
	}

	client := pbrc.NewRecordCollectionServiceClient(conn)
	t := time.Now()
	recs, err := client.GetRecords(context.Background(), &pbrc.GetRecordsRequest{Filter: &pbrc.Record{Release: &pbd.Release{}}}, grpc.MaxCallRecvMsgSize(1024*1024*1024))
	if err != nil {
		log.Fatalf("Error getting records: %v", err)
	}

	fmt.Printf("Got %v records in %v\n", len(recs.GetRecords()), time.Now().Sub(t))
}

func testReadSubset() {
	conn, err := dialMaster("recordcollection")
	defer conn.Close()
	if err != nil {
		log.Fatalf("Unable to dial : %v", err)
	}

	client := pbrc.NewRecordCollectionServiceClient(conn)
	t := time.Now()
	recs, err := client.GetRecords(context.Background(), &pbrc.GetRecordsRequest{Filter: &pbrc.Record{Metadata: &pbrc.ReleaseMetadata{Category: pbrc.ReleaseMetadata_FRESHMAN}}}, grpc.UseCompressor("gzip"), grpc.MaxCallRecvMsgSize(1024*1024*1024))
	if err != nil {
		log.Fatalf("Error getting records: %v", err)
	}

	count := 0
	for _, rc := range recs.GetRecords() {
		if rc.GetRelease().Rating == 0 && rc.GetRelease().FolderId != 488127 {
			fmt.Printf("Found %v -> %v\n", rc.GetRelease().Title, rc.GetMetadata().GetCategory())
		}
	}

	fmt.Printf("Got %v (%v) records in %v -> %v\n", len(recs.GetRecords()), count, time.Now().Sub(t), recs.GetInternalProcessingTime())
}

func testReadSubsetStripped() {
	conn, err := dialMaster("recordcollection")
	defer conn.Close()
	if err != nil {
		log.Fatalf("Unable to dial : %v", err)
	}

	client := pbrc.NewRecordCollectionServiceClient(conn)
	t := time.Now()
	recs, err := client.GetRecords(context.Background(), &pbrc.GetRecordsRequest{Strip: true, Filter: &pbrc.Record{Release: &pbd.Release{FolderId: 242017}, Metadata: &pbrc.ReleaseMetadata{}}}, grpc.UseCompressor("gzip"), grpc.MaxCallRecvMsgSize(1024*1024*1024))
	if err != nil {
		log.Fatalf("Error getting records: %v", err)
	}

	count := 0
	for _, rc := range recs.GetRecords() {
		fmt.Printf("%v\n", rc.GetRelease().Title)
		if len(rc.GetRelease().GetFormats()) > 10 {
			count++
		}
	}

	fmt.Printf("Got %v (%v) records in %v -> %v\n", len(recs.GetRecords()), count, time.Now().Sub(t), recs.GetInternalProcessingTime())
}

func main() {
	testReadCollection()
}
