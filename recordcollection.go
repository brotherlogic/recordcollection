package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/brotherlogic/godiscogs"
	"github.com/brotherlogic/goserver"
	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	pbd "github.com/brotherlogic/godiscogs"
	pbg "github.com/brotherlogic/goserver/proto"
	pb "github.com/brotherlogic/recordcollection/proto"
	pbrm "github.com/brotherlogic/recordmover/proto"
	pbrp "github.com/brotherlogic/recordprocess/proto"
	pbro "github.com/brotherlogic/recordsorganiser/proto"

	_ "net/http/pprof"
)

type quotaChecker interface {
	hasQuota(ctx context.Context, folder int32) (*pbro.QuotaResponse, error)
}

type moveRecorder interface {
	moveRecord(record *pb.Record, oldFolder, newFolder int32) error
}

type prodMoveRecorder struct {
	dial func(server string) (*grpc.ClientConn, error)
}

func (p *prodMoveRecorder) moveRecord(record *pb.Record, oldFolder, newFolder int32) error {
	conn, err := p.dial("recordmover")
	if err != nil {
		return err
	}
	defer conn.Close()

	rmclient := pbrm.NewMoveServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	_, err = rmclient.RecordMove(ctx, &pbrm.MoveRequest{Move: &pbrm.RecordMove{
		InstanceId: record.GetRelease().InstanceId,
		FromFolder: oldFolder,
		ToFolder:   newFolder,
		Record:     record,
	}})

	return err
}

type prodQuotaChecker struct {
	dial func(server string) (*grpc.ClientConn, error)
}

func (p *prodQuotaChecker) hasQuota(ctx context.Context, folder int32) (*pbro.QuotaResponse, error) {
	conn, err := p.dial("recordsorganiser")
	if err != nil {
		return &pbro.QuotaResponse{}, err
	}
	defer conn.Close()

	client := pbro.NewOrganiserServiceClient(conn)
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
	SellRecord(releaseID int, price float32, state string, condition, sleeve string) int
	GetSalePrice(releaseID int) (float32, error)
	RemoveFromWantlist(releaseID int)
	AddToWantlist(releaseID int)
	UpdateSalePrice(saleID int, releaseID int, condition, sleeve string, price float32) error
	GetCurrentSalePrice(saleID int) float32
	GetCurrentSaleState(saleID int) godiscogs.SaleState
	RemoveFromSale(saleID int, releaseID int) error
}

type scorer interface {
	GetScore(ctx context.Context, instanceID int32) (float32, error)
}

type prodScorer struct {
	dial func(server string) (*grpc.ClientConn, error)
}

func (p *prodScorer) GetScore(ctx context.Context, instanceID int32) (float32, error) {
	conn, err := p.dial("recordprocess")
	if err != nil {
		return -1, err
	}
	defer conn.Close()

	client := pbrp.NewScoreServiceClient(conn)
	res, err := client.GetScore(ctx, &pbrp.GetScoreRequest{InstanceId: instanceID})
	if err != nil {
		return -1, err
	}

	score := float32(0)
	for _, sc := range res.Scores {
		score += float32(sc.Rating)
	}

	return score / float32(len(res.Scores)), nil
}

//Server main server type
type Server struct {
	*goserver.GoServer
	collection            *pb.RecordCollection
	retr                  saver
	lastSyncTime          time.Time
	lastPushTime          time.Time
	lastPushLength        time.Duration
	lastPushDone          int
	lastPushSize          int
	cacheWait             time.Duration
	pushMutex             *sync.Mutex
	pushMap               map[int32]*pb.Record
	pushWait              time.Duration
	saveNeeded            bool
	quota                 quotaChecker
	mover                 moveRecorder
	nextPush              *pb.Record
	lastWantUpdate        int32
	wantCheck             string
	lastWantText          string
	scorer                scorer
	saleMap               map[int32]*pb.Record
	lastSalePush          time.Time
	lastSyncLength        time.Duration
	salesPushes           int64
	soldAdjust            int64
	wantUpdate            string
	saves                 int64
	saveMutex             *sync.Mutex
	biggest               int64
	lastSale              int64
	disableSales          bool
	recordCache           map[int32]*pb.Record
	instanceToFolderMutex *sync.Mutex
	allrecords            []*pb.Record
}

func (s *Server) findBiggest(ctx context.Context) error {
	biggestb := 0
	for _, r := range s.getRecords(ctx, "findBiggest") {
		data, _ := proto.Marshal(r)
		if len(data) > biggestb {
			s.biggest = int64(r.GetRelease().Id)
			biggestb = len(data)
		}
	}
	return nil
}

const (
	// KEY The main collection
	KEY = "/github.com/brotherlogic/recordcollection/collection"

	// SAVEKEY individual saves
	SAVEKEY = "/github.com/brotherlogic/recordcollection/records/"

	//TOKEN for discogs
	TOKEN = "/github.com/brotherlogic/recordcollection/token"

	//RECORDS for all the records
	RECORDS = "/github.com/brotherlogic/recordcollection/allrecords"
)

func (s *Server) readRecordCollection(ctx context.Context) error {
	collection := &pb.RecordCollection{}
	data, _, err := s.KSclient.Read(ctx, KEY, collection)

	if err != nil {
		return err
	}

	s.collection = data.(*pb.RecordCollection)

	//Fill the push map
	for _, r := range s.getRecords(ctx, "fillpushmap") {

		//Copy over the instance id if needed
		if r.GetMetadata().InstanceId == 0 {
			r.GetMetadata().InstanceId = r.GetRelease().InstanceId
		}

		// Stop repeated fields from blowing up
		if len(r.GetRelease().GetFormats()) > 100 {
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

		if len(r.GetRelease().GetTracklist()) == 0 {
			r.GetMetadata().LastCache = 1
		}

		if r.GetMetadata().Keep == pb.ReleaseMetadata_KEEP_UNKNOWN {
			r.GetMetadata().Keep = pb.ReleaseMetadata_NOT_KEEPER
		}

		if r.GetMetadata().Keep == pb.ReleaseMetadata_KEEPER {
			r.GetMetadata().NeedsStockCheck = false
		}
	}

	// Fill the sale map
	for _, r := range s.getRecords(ctx, "fillsalemap") {
		if r.GetMetadata().SaleId > 0 {
			s.saleMap[r.GetMetadata().SaleId] = r
		}
	}

	// Fill the update map
	if s.collection.InstanceToUpdate == nil {
		s.collection.InstanceToUpdate = make(map[int32]int64)
	}
	if s.collection.InstanceToCategory == nil {
		s.collection.InstanceToCategory = make(map[int32]pb.ReleaseMetadata_Category)
	}
	if s.collection.InstanceToFolder == nil {
		s.collection.InstanceToFolder = make(map[int32]int32)
	}
	for _, r := range s.getRecords(ctx, "fillinstancemap") {
		s.collection.InstanceToUpdate[r.GetRelease().InstanceId] = r.GetMetadata().NextUpdateTime
		s.collection.InstanceToCategory[r.GetRelease().InstanceId] = r.GetMetadata().Category
		s.collection.InstanceToFolder[r.GetRelease().InstanceId] = r.GetRelease().FolderId
	}

	return nil
}

func (s *Server) getRecords(ctx context.Context, caller string) []*pb.Record {
	if len(s.allrecords) == 0 {
		data, _, err := s.KSclient.Read(ctx, RECORDS, &pb.AllRecords{})

		if err != nil {
			return nil
		}

		s.allrecords = (data.(*pb.AllRecords)).GetRecords()
	}
	return s.allrecords
}

func (s *Server) saveRecordCollection(ctx context.Context) {
	s.saveMutex.Lock()
	defer s.saveMutex.Unlock()
	s.saves++
	s.KSclient.Save(ctx, KEY, s.collection)
	if len(s.allrecords) > 0 {
		s.KSclient.Save(ctx, RECORDS, &pb.AllRecords{Records: s.allrecords})
	}
}

func (s *Server) saveRecord(ctx context.Context, r *pb.Record) error {
	r.GetMetadata().SaveIteration = s.collection.CollectionNumber
	return s.KSclient.Save(ctx, fmt.Sprintf("%v%v", SAVEKEY, r.GetRelease().InstanceId), r)
}

func (s *Server) loadRecord(ctx context.Context, id int32) (*pb.Record, error) {
	record := &pb.Record{}
	data, _, err := s.KSclient.Read(ctx, fmt.Sprintf("%v%v", SAVEKEY, id), record)

	if err != nil {
		return nil, err
	}

	if proto.Size(data) == 0 {
		return nil, fmt.Errorf("Error on read for %v", id)
	}

	recordToReturn := data.(*pb.Record)
	s.recordCache[id] = recordToReturn
	return recordToReturn, nil
}

func (s *Server) getRecord(ctx context.Context, id int32) (*pb.Record, error) {
	if val, ok := s.recordCache[id]; ok {
		return val, nil
	}
	return s.loadRecord(ctx, id)
}

// DoRegister does RPC registration
func (s *Server) DoRegister(server *grpc.Server) {
	pb.RegisterRecordCollectionServiceServer(server, s)
}

// ReportHealth alerts if we're not healthy
func (s *Server) ReportHealth() bool {
	return true
}

// Shutdown the server
func (s *Server) Shutdown(ctx context.Context) error {
	s.saveRecordCollection(ctx)
	return nil
}

// Mote promotes/demotes this server
func (s *Server) Mote(ctx context.Context, master bool) error {
	if master {
		tType := &pb.Token{}
		tResp, _, err := s.KSclient.Read(context.Background(), TOKEN, tType)
		if err != nil {
			return err
		}

		if len(tResp.(*pb.Token).Token) == 0 {
			return fmt.Errorf("Empty token: %v", tResp)
		}

		s.retr = pbd.NewDiscogsRetriever(tResp.(*pb.Token).Token, s.Log)

		err = s.readRecordCollection(ctx)

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
	s.instanceToFolderMutex.Lock()
	defer s.instanceToFolderMutex.Unlock()
	return []*pbg.State{
		&pbg.State{Key: "all_records", Value: int64(len(s.allrecords))},
		&pbg.State{Key: "categories", Value: int64(len(s.collection.GetInstanceToCategory()))},
		&pbg.State{Key: "cache_size", Value: int64(len(s.recordCache))},
		&pbg.State{Key: "folder_map", Value: int64(len(s.collection.InstanceToFolder))},
		&pbg.State{Key: "update_map", Value: int64(len(s.collection.InstanceToUpdate))},
		&pbg.State{Key: "records", Value: int64(len(s.collection.Instances))},
		&pbg.State{Key: "iteration", Value: s.collection.CollectionNumber},
	}
}

// Init builds out a server
func Init() *Server {
	s := &Server{
		GoServer:              &goserver.GoServer{},
		lastSyncTime:          time.Now(),
		pushMap:               make(map[int32]*pb.Record),
		pushWait:              time.Minute,
		pushMutex:             &sync.Mutex{},
		lastPushTime:          time.Now(),
		lastPushSize:          0,
		lastPushLength:        0,
		quota:                 &prodQuotaChecker{},
		mover:                 &prodMoveRecorder{},
		lastWantText:          "",
		scorer:                &prodScorer{},
		saleMap:               make(map[int32]*pb.Record),
		lastSalePush:          time.Now(),
		wantUpdate:            "unknown",
		saveMutex:             &sync.Mutex{},
		recordCache:           make(map[int32]*pb.Record),
		instanceToFolderMutex: &sync.Mutex{},
	}
	s.scorer = &prodScorer{s.DialMaster}
	s.quota = &prodQuotaChecker{s.DialMaster}
	s.mover = &prodMoveRecorder{s.DialMaster}
	return s
}

func (s *Server) cacheLoop(ctx context.Context) error {
	for _, r := range s.getRecords(ctx, "cacheloop") {
		if r.GetMetadata().LastCache <= 1 {
			s.cacheRecord(ctx, r)
			return nil
		}
	}
	return nil
}

func (s *Server) updateSalePrice(ctx context.Context) error {
	for _, r := range s.getRecords(ctx, "updatesaleprice") {
		if r.GetMetadata().CurrentSalePrice == 0 || time.Now().Sub(time.Unix(r.GetMetadata().SalePriceUpdate, 0)) > time.Hour*24*30 {
			price, err := s.retr.GetSalePrice(int(r.GetRelease().Id))
			s.Log(fmt.Sprintf("Retrieved %v, %v", price, err))
			r.GetMetadata().CurrentSalePrice = int32(price * 100)
			r.GetMetadata().SalePriceUpdate = time.Now().Unix()
			s.Log(fmt.Sprintf("Updating %v", r.GetRelease().Id))
			return nil
		}
	}

	return nil
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

	server.PrepServer()

	if len(*token) > 0 {
		server.KSclient.Save(context.Background(), TOKEN, &pb.Token{Token: *token})
		log.Fatalf("Written TOKEN")
	}

	server.Register = server
	server.SendTrace = true

	t := time.Now()
	err := server.RegisterServer("recordcollection", false)
	if err != nil {
		log.Fatalf("Unable to register (%v) at %v", err, time.Now().Sub(t))
	}

	// This enables pprof
	go http.ListenAndServe(":8089", nil)

	server.RegisterRepeatingTask(server.runSync, "run_sync", time.Hour)
	server.RegisterRepeatingTask(server.runSyncWants, "run_sync_wants", time.Hour)
	server.RegisterRepeatingTask(server.pushWants, "push_wants", time.Minute)
	server.RegisterRepeatingTask(server.runPush, "run_push", time.Minute)
	server.RegisterRepeatingTask(server.syncIssue, "sync_issue", time.Hour)
	server.RegisterRepeatingTask(server.pushSales, "push_sales", time.Minute)
	server.RegisterRepeatingTask(server.cacheLoop, "cache_loop", time.Minute)
	server.RegisterRepeatingTask(server.updateSalePrice, "update_sale_price", time.Minute*5)

	//server.disableSales = true

	server.MemCap = 400000000
	server.Serve()
}
