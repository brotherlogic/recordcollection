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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pbd "github.com/brotherlogic/godiscogs"
	pbg "github.com/brotherlogic/goserver/proto"
	pbks "github.com/brotherlogic/keystore/proto"
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
	moveRecord(ctx context.Context, record *pb.Record, oldFolder, newFolder int32) error
}

type prodMoveRecorder struct {
	dial func(server string) (*grpc.ClientConn, error)
}

func (p *prodMoveRecorder) moveRecord(ctx context.Context, record *pb.Record, oldFolder, newFolder int32) error {
	conn, err := p.dial("recordmover")
	if err != nil {
		return err
	}
	defer conn.Close()

	rmclient := pbrm.NewMoveServiceClient(conn)
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
	instanceToFolderMutex *sync.Mutex
	mismatches            int
	recordCache           map[int32]*pb.Record
	recordCacheMutex      *sync.Mutex
	TimeoutLoad           bool
	collectionMutex       *sync.Mutex
	longest               time.Duration
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

	// Create the instance to recahe map
	if s.collection.InstanceToRecache == nil {
		s.collection.InstanceToRecache = make(map[int32]int64)
	}

	if s.collection.InstanceToLastSalePriceUpdate == nil {
		s.collection.InstanceToLastSalePriceUpdate = make(map[int32]int64)
	}

	return nil
}

func (s *Server) saveRecordCollection(ctx context.Context) error {
	s.saves++
	s.collectionMutex.Lock()
	defer s.collectionMutex.Unlock()
	return s.KSclient.Save(ctx, KEY, s.collection)
}

func (s *Server) deleteRecord(ctx context.Context, i int32) error {
	if !s.SkipLog {
		conn, err := s.DialMaster("keystore")
		if err != nil {
			return err
		}
		defer conn.Close()

		client := pbks.NewKeyStoreServiceClient(conn)
		_, err = client.Delete(ctx, &pbks.DeleteRequest{Key: fmt.Sprintf("%v%v", SAVEKEY, i)})
		return err
	}
	return nil
}

func (s *Server) saveRecord(ctx context.Context, r *pb.Record) error {
	s.saveMutex.Lock()
	defer s.saveMutex.Unlock()

	if r.GetMetadata().GoalFolder == 0 {
		s.RaiseIssue(ctx, "Save Error", fmt.Sprintf("Trying to save a record without a goal folder: %v", r), false)
		return fmt.Errorf("No goal folder")
	}

	r.GetMetadata().SaveIteration = s.collection.CollectionNumber
	err := s.KSclient.Save(ctx, fmt.Sprintf("%v%v", SAVEKEY, r.GetRelease().InstanceId), r)

	s.collectionMutex.Lock()
	save := false
	if s.collection.InstanceToFolder[r.GetRelease().InstanceId] != r.GetRelease().FolderId {
		s.collection.InstanceToFolder[r.GetRelease().InstanceId] = r.GetRelease().FolderId
		save = true
	}

	if s.collection.InstanceToCategory[r.GetRelease().InstanceId] != r.GetMetadata().Category {
		s.collection.InstanceToCategory[r.GetRelease().InstanceId] = r.GetMetadata().Category
		save = true
	}

	if s.collection.InstanceToUpdate[r.GetRelease().InstanceId] != r.GetMetadata().LastUpdateTime {
		s.collection.InstanceToUpdate[r.GetRelease().InstanceId] = r.GetMetadata().LastUpdateTime
		save = true
	}

	if s.collection.InstanceToMaster[r.GetRelease().InstanceId] != r.GetRelease().MasterId {
		s.collection.InstanceToMaster[r.GetRelease().InstanceId] = r.GetRelease().MasterId
		save = true
	}

	if s.collection.InstanceToId[r.GetRelease().InstanceId] != r.GetRelease().Id {
		s.collection.InstanceToId[r.GetRelease().InstanceId] = r.GetRelease().Id
		save = true
	}

	if s.collection.InstanceToLastSalePriceUpdate[r.GetRelease().InstanceId] != r.GetMetadata().GetSalePriceUpdate() {
		s.collection.InstanceToLastSalePriceUpdate[r.GetRelease().InstanceId] = r.GetMetadata().GetSalePriceUpdate()
		save = true
	}

	if r.GetMetadata().LastCache == 0 || r.GetMetadata().LastCache == 1 {
		s.collection.InstanceToRecache[r.GetRelease().InstanceId] = time.Now().Unix()
	} else {
		s.collection.InstanceToRecache[r.GetRelease().InstanceId] = time.Unix(r.GetMetadata().LastCache, 0).Add(time.Hour * 24 * 7 * 2).Unix()
	}
	s.collectionMutex.Unlock()

	if save {
		s.saveRecordCollection(ctx)
	}

	s.recordCacheMutex.Lock()
	defer s.recordCacheMutex.Unlock()
	s.recordCache[r.GetRelease().InstanceId] = r

	return err
}

func (s *Server) loadRecord(ctx context.Context, id int32) (*pb.Record, error) {
	if s.TimeoutLoad {
		return nil, status.Error(codes.DeadlineExceeded, "Force DE")
	}
	s.recordCacheMutex.Lock()
	defer s.recordCacheMutex.Unlock()
	if val, ok := s.recordCache[id]; ok {
		return val, nil
	}

	record := &pb.Record{}
	data, _, err := s.KSclient.Read(ctx, fmt.Sprintf("%v%v", SAVEKEY, id), record)

	if err != nil {
		return nil, err
	}

	if proto.Size(data) == 0 {
		return nil, fmt.Errorf("Error on read for %v", id)
	}

	recordToReturn := data.(*pb.Record)
	s.recordCache[recordToReturn.GetRelease().InstanceId] = recordToReturn

	s.collectionMutex.Lock()
	if recordToReturn.GetMetadata().LastCache == 0 {
		s.collection.InstanceToRecache[recordToReturn.GetRelease().InstanceId] = time.Now().Unix()
	} else {
		s.collection.InstanceToRecache[recordToReturn.GetRelease().InstanceId] = time.Unix(recordToReturn.GetMetadata().LastCache, 0).Add(time.Hour * 24 * 7 * 2).Unix()
	}

	if recordToReturn.GetMetadata().GetDirty() {
		s.collection.NeedsPush = append(s.collection.NeedsPush, recordToReturn.GetRelease().GetInstanceId())
	}
	s.collectionMutex.Unlock()

	return recordToReturn, nil
}

func (s *Server) getRecord(ctx context.Context, id int32) (*pb.Record, error) {
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
	s.recordCacheMutex.Lock()
	defer s.recordCacheMutex.Unlock()
	s.collectionMutex.Lock()
	defer s.collectionMutex.Unlock()

	count := 0
	if s.collection != nil {
		for _, push := range s.collection.NeedsPush {
			if push == 404128726 {
				count++
			}
		}
	}

	dcount := time.Now().Unix()
	ecount := 0
	for _, val := range s.collection.GetInstanceToLastSalePriceUpdate() {
		if val < dcount {
			dcount = val
		}
		if val > 1 {
			ecount++
		}
	}

	return []*pbg.State{
		&pbg.State{Key: "price_min", TimeValue: int64(dcount)},
		&pbg.State{Key: "price_update", Value: int64(len(s.collection.GetInstanceToLastSalePriceUpdate()) - ecount)},
		&pbg.State{Key: "longest", TimeDuration: s.longest.Nanoseconds()},
		&pbg.State{Key: "needs_push_sales", Text: fmt.Sprintf("%v", s.collection.GetSaleUpdates())},
		&pbg.State{Key: "needs_push", Text: fmt.Sprintf("%v", s.collection.GetNeedsPush())},
		&pbg.State{Key: "recache_size", Value: int64(len(s.collection.GetInstanceToRecache()))},
		&pbg.State{Key: "cache_size", Value: int64(len(s.recordCache))},
		&pbg.State{Key: "to_sell", Value: int64(len(s.collection.GetSaleUpdates()))},
		&pbg.State{Key: "master_size", Value: int64(len(s.collection.GetInstanceToMaster()))},
		&pbg.State{Key: "collection_size", Value: int64(proto.Size(s.collection))},
		&pbg.State{Key: "categories", Value: int64(len(s.collection.GetInstanceToCategory()))},
		&pbg.State{Key: "folder_map", Value: int64(len(s.collection.GetInstanceToFolder()))},
		&pbg.State{Key: "update_map", Value: int64(len(s.collection.GetInstanceToUpdate()))},
		&pbg.State{Key: "records", Value: int64(len(s.collection.GetInstances()))},
		&pbg.State{Key: "iteration", Value: s.collection.GetCollectionNumber()},
		&pbg.State{Key: "mismatches", Value: int64(s.mismatches)},
	}
}

// Init builds out a server
func Init() *Server {
	s := &Server{
		GoServer:              &goserver.GoServer{},
		lastSyncTime:          time.Now(),
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
		instanceToFolderMutex: &sync.Mutex{},
		recordCache:           make(map[int32]*pb.Record),
		recordCacheMutex:      &sync.Mutex{},
		collectionMutex:       &sync.Mutex{},
	}
	s.scorer = &prodScorer{s.DialMaster}
	s.quota = &prodQuotaChecker{s.DialMaster}
	s.mover = &prodMoveRecorder{s.DialMaster}
	return s
}

func (s *Server) runRecache(ctx context.Context) error {
	s.collectionMutex.Lock()
	for id, key := range s.collection.InstanceToRecache {
		if time.Unix(key, 0).Before(time.Now()) {
			s.collectionMutex.Unlock()
			r, err := s.loadRecord(ctx, id)
			if err != nil {
				return err
			}
			err = s.recache(ctx, r)
			if err != nil {
				return err
			}
			return s.saveRecord(ctx, r)
		}
	}
	s.collectionMutex.Unlock()
	return nil
}

func (s *Server) updateSalePrice(ctx context.Context) error {
	for id, val := range s.collection.InstanceToLastSalePriceUpdate {
		if time.Now().Sub(time.Unix(val, 0)) > time.Hour*24*30 {
			r, err := s.loadRecord(ctx, id)
			if err != nil {
				return err
			}
			price, err := s.retr.GetSalePrice(int(r.GetRelease().Id))
			if err != nil {
				return err
			}
			s.Log(fmt.Sprintf("Retrieved %v, %v -> %v", price, err, r.GetRelease().Id))
			r.GetMetadata().CurrentSalePrice = int32(price * 100)
			r.GetMetadata().SalePriceUpdate = time.Now().Unix()
			s.saveRecord(ctx, r)
			return nil
		}
	}

	if len(s.collection.InstanceToLastSalePriceUpdate) != len(s.collection.InstanceToFolder) {
		for id := range s.collection.InstanceToFolder {
			if _, ok := s.collection.InstanceToLastSalePriceUpdate[id]; !ok {
				s.collection.InstanceToLastSalePriceUpdate[id] = 1
			}
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

	err := server.RegisterServerV2("recordcollection", false, false)
	if err != nil {
		return
	}

	// This enables pprof
	go http.ListenAndServe(":8089", nil)

	server.RegisterRepeatingTask(server.runSync, "run_sync", time.Hour)
	server.RegisterRepeatingTask(server.runSyncWants, "run_sync_wants", time.Hour)
	server.RegisterRepeatingTask(server.pushWants, "push_wants", time.Minute)
	server.RegisterRepeatingTask(server.runPush, "run_push", time.Minute)
	server.RegisterRepeatingTask(server.runRecache, "run_recache", time.Minute)
	server.RegisterRepeatingTask(server.pushSales, "push_sales", time.Minute)
	server.RegisterRepeatingTask(server.updateSalePrice, "update_sale_price", time.Second*5)

	server.disableSales = false

	server.MemCap = 400000000
	server.Serve()
}
