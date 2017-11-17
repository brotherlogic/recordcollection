package main

import (
	"flag"
	"io/ioutil"
	"log"

	"google.golang.org/grpc"

	"github.com/brotherlogic/goserver"
	"github.com/brotherlogic/keystore/client"

	pbg "github.com/brotherlogic/goserver/proto"
	pb "github.com/brotherlogic/recordcollection/proto"
)

//Server main server type
type Server struct {
	*goserver.GoServer
	collection *pb.RecordCollection
}

const (
	// KEY The main collection
	KEY = "github.com/brotherlogic/recordcollection/collection"
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

// DoRegister does RPC registration
func (s *Server) DoRegister(server *grpc.Server) {
	pb.RegisterDiscogsServiceServer(server, s)
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
	return []*pbg.State{}
}

// Init builds out a server
func Init() *Server {
	return &Server{}
}

func main() {
	var quiet = flag.Bool("quiet", false, "Show all output")
	flag.Parse()

	//Turn off logging
	if *quiet {
		log.SetFlags(0)
		log.SetOutput(ioutil.Discard)
	}

	server := Init()
	server.Register = server
	server.PrepServer()

	server.GoServer.KSclient = *keystoreclient.GetClient(server.GetIP)
	server.RegisterServer("recordcollection", false)
	server.Log("Starting!")
	server.Serve()
}
