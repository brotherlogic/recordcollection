package main

import (
	"flag"
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
}

//Server main server type
type Server struct {
	*goserver.GoServer
	collection   *pb.RecordCollection
	retr         saver
	lastSyncTime time.Time
	cacheMap     map[int32]*pb.Record
	cacheWait    time.Duration
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
		return s.readRecordCollection()
	}

	return nil
}

// GetState gets the state of the server
func (s *Server) GetState() []*pbg.State {
	return []*pbg.State{&pbg.State{Key: "last_sync_time", TimeValue: s.lastSyncTime.Unix()}, &pbg.State{Key: "sync_size", Value: int64(len(s.cacheMap))}}
}

// Init builds out a server
func Init() *Server {
	return &Server{GoServer: &goserver.GoServer{}, lastSyncTime: time.Now(), cacheMap: make(map[int32]*pb.Record), cacheWait: time.Minute}
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

	server.retr = pbd.NewDiscogsRetriever(tResp.(*pb.Token).Token)
	server.Register = server

	server.RegisterServer("recordcollection", false)
	server.RegisterRepeatingTask(server.runSync, time.Hour)
	server.RegisterRepeatingTask(server.runRecache, time.Hour)
	server.Log("Starting!")
	server.Serve()
}
