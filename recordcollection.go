package main

import (
	"context"
	"flag"
	"fmt"
"log"
	"io/ioutil"
	"strconv"
	"sync"
	"time"

	"google.golang.org/grpc"

	"github.com/brotherlogic/godiscogs"
	"github.com/brotherlogic/goserver"
	"github.com/brotherlogic/keystore/client"

	pbd "github.com/brotherlogic/godiscogs"
	pbg "github.com/brotherlogic/goserver/proto"
	"github.com/brotherlogic/goserver/utils"
	pb "github.com/brotherlogic/recordcollection/proto"
	pbrm "github.com/brotherlogic/recordmover/proto"
	pbro "github.com/brotherlogic/recordsorganiser/proto"
)

type quotaChecker interface {
	hasQuota(folder int32) (*pbro.QuotaResponse, error)
}

type moveRecorder interface {
	moveRecord(InstanceID, oldFolder, newFolder int32) error
}

type prodMoveRecorder struct{}

func (p *prodMoveRecorder) moveRecord(InstanceID, oldFolder, newFolder int32) error {
	ip, port, err := utils.Resolve("recordmover")
	if err != nil {
		return err
	}

	conn, err := grpc.Dial(ip+":"+strconv.Itoa(int(port)), grpc.WithInsecure())
	if err != nil {
		return err
	}
	defer conn.Close()

	rmclient := pbrm.NewMoveServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	_, err = rmclient.RecordMove(ctx, &pbrm.MoveRequest{Move: &pbrm.RecordMove{
		InstanceId: InstanceID,
		FromFolder: oldFolder,
		ToFolder:   newFolder,
	}})

	return err
}

type prodQuotaChecker struct{}

func (p *prodQuotaChecker) hasQuota(folder int32) (*pbro.QuotaResponse, error) {
	ip, port, err := utils.Resolve("recordsorganiser")
	if err != nil {
		return &pbro.QuotaResponse{}, err
	}

	conn, err := grpc.Dial(ip+":"+strconv.Itoa(int(port)), grpc.WithInsecure())
	if err != nil {
		return &pbro.QuotaResponse{}, err
	}
	defer conn.Close()

	client := pbro.NewOrganiserServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	return client.GetQuota(ctx, &pbro.QuotaRequest{FolderId: folder})
}

type saver interface {
	GetCollection() []*godiscogs.Release
	GetWantlist() ([]*godiscogs.Release, error)
	GetRelease(id int32) (*godiscogs.Release, error)
	AddToFolder(folderID int32, releaseID int32) (int, error)
	SetRating(releaseID int, rating int) error
	MoveToFolder(folderID, releaseID, instanceID, newFolderID int) string
	DeleteInstance(folderID, releaseID, instanceID int) string
	SellRecord(releaseID int, price float32, state string)
	GetSalePrice(releaseID int) float32
}

//Server main server type
type Server struct {
	*goserver.GoServer
	collection     *pb.RecordCollection
	retr           saver
	lastSyncTime   time.Time
	lastPushTime   time.Time
	lastPushLength time.Duration
	lastPushDone   int
	lastPushSize   int
	cacheMutex     *sync.Mutex
	cacheMap       map[int32]*pb.Record
	cacheWait      time.Duration
	pushMutex      *sync.Mutex
	pushMap        map[int32]*pb.Record
	pushWait       time.Duration
	saveNeeded     bool
	quota          quotaChecker
	mover          moveRecorder
	nextPush       *pb.Record
}

const (
	// KEY The main collection
	KEY = "/github.com/brotherlogic/recordcollection/collection"

	//TOKEN for discogs
	TOKEN = "/github.com/brotherlogic/recordcollection/token"
)

func (s *Server) saveLoop() {
	if s.saveNeeded {
		s.saveNeeded = false
		s.saveRecordCollection()
	}
}

func (s *Server) readRecordCollection() error {
	collection := &pb.RecordCollection{}
	data, _, err := s.KSclient.Read(KEY, collection)

	if err != nil {
		return err
	}

	s.collection = data.(*pb.RecordCollection)

	//Fill the push map
	for _, r := range s.collection.GetRecords() {
		if r.GetMetadata() == nil {
			r.Metadata = &pb.ReleaseMetadata{}
		}

		// Stop repeated fields from blowing up
		if len(r.GetRelease().GetFormats()) > 10 {
			r.GetRelease().Images = []*pbd.Image{}
			r.GetRelease().Artists = []*pbd.Artist{}
			r.GetRelease().Formats = []*pbd.Format{}
			r.GetRelease().Labels = []*pbd.Label{}
			r.GetMetadata().LastCache = 1
		}

		if r.GetMetadata().GetMoveFolder() > 0 || r.GetMetadata().GetSetRating() > 0 {
			r.GetMetadata().Dirty = true
		}

		if r.GetMetadata().Dirty {
			s.pushMutex.Lock()
			s.pushMap[r.GetRelease().InstanceId] = r
			s.pushMutex.Unlock()
		}
	}

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

	tText := "No Next Record"
	if s.nextPush != nil {
	   tText = s.nextPush.GetRelease().Title
	   }

	return []*pbg.State{
		&pbg.State{Key: "core", Value: int64((stateCount * 100) / max(1, len(s.collection.GetRecords())))},
		&pbg.State{Key: "last_sync_time", TimeValue: s.lastSyncTime.Unix()},
		&pbg.State{Key: "sync_size", Value: int64(len(s.cacheMap))},
		&pbg.State{Key: "to_push", Value: int64(len(s.pushMap))},
		&pbg.State{Key: "sizington", Text: fmt.Sprintf("%v and %v", len(s.collection.GetRecords()), len(s.collection.GetWants()))},
		&pbg.State{Key: "push_state", Text: fmt.Sprintf("Started %v [%v / %v]; took %v", s.lastPushTime, s.lastPushSize, s.lastPushDone, s.lastPushLength)},
		&pbg.State{Key: "next_push", Text: tText},
	}
}

// Init builds out a server
func Init() *Server {
	return &Server{
		GoServer:       &goserver.GoServer{},
		lastSyncTime:   time.Now(),
		cacheMap:       make(map[int32]*pb.Record),
		cacheWait:      time.Second,
		cacheMutex:     &sync.Mutex{},
		pushMap:        make(map[int32]*pb.Record),
		pushWait:       time.Minute,
		pushMutex:      &sync.Mutex{},
		lastPushTime:   time.Now(),
		lastPushSize:   0,
		lastPushLength: 0,
		quota:          &prodQuotaChecker{},
		mover:          &prodMoveRecorder{},
	}
}

func main() {
	var quiet = flag.Bool("quiet", false, "Show all output")
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
		return
	}

	server.retr = pbd.NewDiscogsRetriever(tResp.(*pb.Token).Token, server.Log)
	server.Register = server

	server.RegisterServer("recordcollection", false)
	server.RegisterRepeatingTask(server.runSync, time.Hour)
	server.RegisterRepeatingTask(server.runRecache, time.Minute)
	server.RegisterRepeatingTask(server.runPush, time.Minute)
	server.RegisterRepeatingTask(server.saveLoop, time.Second)
	server.Serve()
}
