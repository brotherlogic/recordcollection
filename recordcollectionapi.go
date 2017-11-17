package main

import (
	"golang.org/x/net/context"

	pb "github.com/brotherlogic/recordcollection/proto"
)

// GetRecords gets a bunch of records
func (s *Server) GetRecords(ctx context.Context, request *pb.GetRecordsRequest) (*pb.GetRecordsResponse, error) {
	return nil, nil
}
