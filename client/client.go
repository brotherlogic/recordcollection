package recordcollection_client

import (
	"context"

	pbgs "github.com/brotherlogic/goserver"
	pb "github.com/brotherlogic/recordcollection/proto"
)

type RecordCollectionClient struct {
	Gs     *pbgs.GoServer
	getMap map[int32]*pb.Record
	Test   bool
}

func (c *RecordCollectionClient) AddRecord(r *pb.Record) {
	if c.getMap == nil {
		c.getMap = make(map[int32]*pb.Record)
	}
	c.getMap[r.GetRelease().GetInstanceId()] = r
}

func (c *RecordCollectionClient) GetRecord(ctx context.Context, req *pb.GetRecordRequest) (*pb.GetRecordResponse, error) {
	if c.Test {
		return &pb.GetRecordResponse{Record: c.getMap[req.GetInstanceId()]}, nil
	}

	conn, err := c.Gs.FDialServer(ctx, "recordcollection")
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	client := pb.NewRecordCollectionServiceClient(conn)
	return client.GetRecord(ctx, req)
}

func (c *RecordCollectionClient) QueryRecords(ctx context.Context, req *pb.QueryRecordsRequest) (*pb.QueryRecordsResponse, error) {
	if c.Test {
		var keys []int32
		for k, r := range c.getMap {
			if req.GetReleaseId() > 0 && r.GetRelease().GetId() == req.GetReleaseId() {
				keys = append(keys, k)
			}
		}
		return &pb.QueryRecordsResponse{InstanceIds: keys}, nil
	}
	conn, err := c.Gs.FDialServer(ctx, "recordcollection")
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	client := pb.NewRecordCollectionServiceClient(conn)
	return client.QueryRecords(ctx, req)
}

func (c *RecordCollectionClient) GetPrice(ctx context.Context, req *pb.GetPriceRequest) (*pb.GetPriceResponse, error) {
	if c.Test {
		return &pb.GetPriceResponse{Price: 23.45}, nil
	}
	conn, err := c.Gs.FDialServer(ctx, "recordcollection")
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	client := pb.NewRecordCollectionServiceClient(conn)
	return client.GetPrice(ctx, req)
}
