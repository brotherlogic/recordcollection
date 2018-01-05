package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"time"

	"google.golang.org/grpc"

	"github.com/brotherlogic/godiscogs"
	"github.com/brotherlogic/goserver"
	"github.com/brotherlogic/keystore/client"

	pbd "github.com/brotherlogic/godiscogs"
	pbg "github.com/brotherlogic/goserver/proto"
	pb "github.com/brotherlogic/recordcollection/proto"
)

type saver interface {
	GetCollection() []*godiscogs.Release
	GetWantlist() ([]*godiscogs.Release, error)
	GetRelease(id int32) (*godiscogs.Release, error)
	AddToFolder(folderID int32, releaseID int32) (int, error)
	SetRating(releaseID int, rating int) error
	MoveToFolder(folderID, releaseID, instanceID, newFolderID int)
}

//Server main server type
type Server struct {
	*goserver.GoServer
	collection   *pb.RecordCollection
	retr         saver
	lastSyncTime time.Time
	cacheMap     map[int32]*pb.Record
	cacheWait    time.Duration
	pushMap      map[int32]*pb.Record
	pushWait     time.Duration
}

const (
	// KEY The main collection
	KEY = "/github.com/brotherlogic/recordcollection/collection"

	//TOKEN for discogs
	TOKEN = "/github.com/brotherlogic/recordcollection/token"
)

func (s *Server) readRecordCollection() error {
	collection := &pb.RecordCollection{}
	data, _, err := s.KSclient.Read(KEY, collection)

	if err != nil {
		return err
	}

	s.collection = data.(*pb.RecordCollection)
	return nil
}

func (s *Server) saveRecordCollection() {
	s.KSclient.Save(KEY, s.collection)
}

// DoRegister does RPC registration
func (s *Server) DoRegister(server *grpc.Server) {
	pb.RegisterRecordCollectionServiceServer(server, s)
}

// ReportHealth alerts if we're not healthy
func (s *Server) ReportHealth() bool {
	return true
}

// Mote promotes/demotes this server
func (s *Server) Mote(master bool) error {
	if master {
		err := s.readRecordCollection()
		return err
	}

	return nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// GetState gets the state of the server
func (s *Server) GetState() []*pbg.State {
	stateCount := 0
	for _, r := range s.collection.GetRecords() {
		if r.GetMetadata().GetCategory() != pb.ReleaseMetadata_UNKNOWN {
			stateCount++
		}
	}

	s.Log(fmt.Sprintf("Count = %v leads to %v", stateCount, int64((stateCount*100)/max(1, len(s.collection.GetRecords())))))

	return []*pbg.State{&pbg.State{Key: "core", Value: int64((stateCount * 100) / max(1, len(s.collection.GetRecords())))}, &pbg.State{Key: "last_sync_time", TimeValue: s.lastSyncTime.Unix()}, &pbg.State{Key: "sync_size", Value: int64(len(s.cacheMap))}}
}

// Init builds out a server
func Init() *Server {
	return &Server{GoServer: &goserver.GoServer{}, lastSyncTime: time.Now(), cacheMap: make(map[int32]*pb.Record), cacheWait: time.Minute, pushMap: make(map[int32]*pb.Record), pushWait: time.Minute}
}

func main() {
	var quiet = flag.Bool("quiet", true, "Show all output")
	var token = flag.String("token", "", "Discogs token")
	flag.Parse()

	//Turn off logging
	if *quiet {
		log.SetFlags(0)
		log.SetOutput(ioutil.Discard)
	}
	server := Init()
	server.GoServer.KSclient = *keystoreclient.GetClient(server.GetIP)
	server.PrepServer()

	if len(*token) > 0 {
		server.KSclient.Save(TOKEN, &pb.Token{Token: *token})
		log.Fatalf("Written TOKEN")
	}
	tType := &pb.Token{}
	tResp, _, err := server.KSclient.Read(TOKEN, tType)

	if err != nil {
		log.Fatalf("Error reading token: %v", err)
	}

	server.Log(fmt.Sprintf("Starting record collection with %v", tResp))
	server.retr = pbd.NewDiscogsRetriever(tResp.(*pb.Token).Token)
	server.Register = server

	server.RegisterServer("recordcollection", false)
	server.RegisterRepeatingTask(server.runSync, time.Hour)
	server.RegisterRepeatingTask(server.runRecache, time.Hour)
	server.RegisterRepeatingTask(server.runPush, time.Hour)
	server.Log("Starting!")
	server.Serve()
}
