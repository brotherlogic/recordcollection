package main

import (
	"context"
	"testing"

	"github.com/brotherlogic/goserver"
	"github.com/brotherlogic/keystore/client"

	pb "github.com/brotherlogic/recordcollection/proto"
)

func InitTestServer() *Server {
	s := &Server{GoServer: &goserver.GoServer{}, collection: &pb.RecordCollection{}}
	s.retr = &testSyncer{}
	s.GoServer.KSclient = *keystoreclient.GetTestClient(".testing/")
	return s
}

func TestGetRecords(t *testing.T) {
	s := InitTestServer()
	_, err := s.GetRecords(context.Background(), nil)

	if err != nil {
		t.Errorf("Error in getting records: %v", err)
	}
}
