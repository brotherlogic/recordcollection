package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/brotherlogic/godiscogs"
	pbd "github.com/brotherlogic/godiscogs"
	"github.com/brotherlogic/goserver"
	pbg "github.com/brotherlogic/goserver/proto"
	"github.com/brotherlogic/goserver/utils"
	"github.com/brotherlogic/keystore/client"
	pb "github.com/brotherlogic/recordcollection/proto"
	pbrm "github.com/brotherlogic/recordmover/proto"
	pbrp "github.com/brotherlogic/recordprocess/proto"
	pbro "github.com/brotherlogic/recordsorganiser/proto"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

type quotaChecker interface {
	hasQuota(ctx context.Context, folder int32) (*pbro.QuotaResponse, error)
}

type moveRecorder interface {
	moveRecord(record *pb.Record, oldFolder, newFolder int32) error
}

type prodMoveRecorder struct{}

func (p *prodMoveRecorder) moveRecord(record *pb.Record, oldFolder, newFolder int32) error {
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
		InstanceId: record.GetRelease().InstanceId,
		FromFolder: oldFolder,
		ToFolder:   newFolder,
		Record:     record,
	}})

	return err
}

type prodQuotaChecker struct{}

func (p *prodQuotaChecker) hasQuota(ctx context.Context, folder int32) (*pbro.QuotaResponse, error) {
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
	SellRecord(releaseID int, price float32, state string) int
	GetSalePrice(releaseID int) float32
	RemoveFromWantlist(releaseID int)
	AddToWantlist(releaseID int)
	UpdateSalePrice(saleID int, releaseID int, condition string, price float32) error
	GetCurrentSalePrice(saleID int) float32
	GetCurrentSaleState(saleID int) godiscogs.SaleState
}

type scorer interface {
	GetScore(instanceID int32) (float32, error)
}

type prodScorer struct{}

func (p *prodScorer) GetScore(instanceID int32) (float32, error) {
	ip, port, err := utils.Resolve("recordprocess")
	if err != nil {
		return -1, err
	}

	conn, err := grpc.Dial(ip+":"+strconv.Itoa(int(port)), grpc.WithInsecure())
	if err != nil {
		return -1, err
	}
	defer conn.Close()

	client := pbrp.NewScoreServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

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
	lastWantUpdate int32
	wantCheck      string
	lastWantText   string
	scorer         scorer
	saleMap        map[int32]*pb.Record
	lastSalePush   time.Time
	lastSyncLength time.Duration
	salesPushes    int64
}

const (
	// KEY The main collection
	KEY = "/github.com/brotherlogic/recordcollection/collection"

	//TOKEN for discogs
	TOKEN = "/github.com/brotherlogic/recordcollection/token"
)

func (s *Server) saveLoop(ctx context.Context) {
	if s.saveNeeded {
		s.saveNeeded = false
		s.saveRecordCollection(ctx)
	}
}

func (s *Server) readRecordCollection(ctx context.Context) error {
	collection := &pb.RecordCollection{}
	data, _, err := s.KSclient.Read(ctx, KEY, collection)

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
	}

	// Fill the sale map
	for _, r := range s.collection.GetRecords() {
		if r.GetMetadata().SaleId > 0 {
			s.saleMap[r.GetMetadata().SaleId] = r
		}
	}

	return nil
}

func (s *Server) saveRecordCollection(ctx context.Context) {
	s.KSclient.Save(ctx, KEY, s.collection)
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
func (s *Server) Mote(ctx context.Context, master bool) error {
	if master {
		tType := &pb.Token{}
		tResp, _, err := s.KSclient.Read(context.Background(), TOKEN, tType)
		if err != nil {
			return err
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
	unknownCount := 0
	for _, r := range s.collection.GetRecords() {
		if r.GetMetadata().SaleId > 0 && r.GetMetadata().SalePrice == 0 {
			unknownCount++
		}
	}

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

	count := 0
	for _, w := range s.collection.GetNewWants() {
		if w.GetMetadata().Active {
			count++
		}
	}

	twelves := 0
	scoredTwelves := 0
	for _, w := range s.collection.GetRecords() {
		if w.GetRelease().FolderId == 242017 {
			twelves++
			if w.GetRelease().Rating > 0 {
				scoredTwelves++
			}
		} else if w.GetRelease().FolderId == 812802 && w.GetMetadata().GoalFolder == 242017 && (w.GetMetadata().Category != pb.ReleaseMetadata_UNLISTENED && w.GetMetadata().Category != pb.ReleaseMetadata_STAGED && w.GetMetadata().Category != pb.ReleaseMetadata_PRE_FRESHMAN && w.GetMetadata().Category != pb.ReleaseMetadata_STAGED_TO_SELL) {
			twelves++
		}
	}

	noInstanceCount := 0
	for _, r := range s.collection.GetRecords() {
		if r.GetRelease().InstanceId == 0 {
			noInstanceCount++
		}
	}

	oldSyncCount := 0
	noScoreCount := 0
	oldestSync := time.Now().Unix()
	for _, r := range s.collection.GetRecords() {
		if time.Now().Sub(time.Unix(r.GetMetadata().LastSyncTime, 0)) > time.Hour*24*7 {
			oldSyncCount++

		}
		if r.GetMetadata().OverallScore == 0 {
			noScoreCount++
		}

		if r.GetMetadata().LastSyncTime < oldestSync {
			oldestSync = r.GetMetadata().LastSyncTime
		}
	}

	diffCount := 0
	badFolder := make(map[string]bool)
	for _, r := range s.collection.GetRecords() {
		if r.GetRelease().FolderId != r.GetMetadata().GoalFolder {
			if r.GetRelease().FolderId != 673768 && r.GetRelease().FolderId != 812802 {
				diffCount++
				badFolder[fmt.Sprintf("%v", r.GetMetadata().Category)] = true
			}
		}
	}

	return []*pbg.State{
		&pbg.State{Key: "core", Value: int64((stateCount * 100) / max(1, len(s.collection.GetRecords())))},
		&pbg.State{Key: "last_sync_time", TimeValue: s.lastSyncTime.Unix()},
		&pbg.State{Key: "oldest_sync", TimeValue: oldestSync},
		&pbg.State{Key: "sync_size", Value: int64(len(s.cacheMap))},
		&pbg.State{Key: "to_push", Value: int64(len(s.pushMap))},
		&pbg.State{Key: "sizington", Text: fmt.Sprintf("%v and %v", len(s.collection.GetRecords()), len(s.collection.GetWants()))},
		&pbg.State{Key: "push_state", Text: fmt.Sprintf("Started %v [%v / %v]; took %v", s.lastPushTime, s.lastPushSize, s.lastPushDone, s.lastPushLength)},
		&pbg.State{Key: "next_push", Text: tText},
		&pbg.State{Key: "want_check", Text: s.wantCheck},
		&pbg.State{Key: "last_want", Value: int64(s.lastWantUpdate)},
		&pbg.State{Key: "last_want_text", Text: s.lastWantText},
		&pbg.State{Key: "want_count", Value: int64(count)},
		&pbg.State{Key: "all_twelves", Value: int64(twelves)},
		&pbg.State{Key: "scoredTwelves", Value: int64(scoredTwelves)},
		&pbg.State{Key: "old_sync", Value: int64(oldSyncCount)},
		&pbg.State{Key: "no_instance", Value: int64(noInstanceCount)},
		&pbg.State{Key: "no_score", Value: int64(noScoreCount)},
		&pbg.State{Key: "sale_map", Value: int64(len(s.saleMap))},
		&pbg.State{Key: "unknow_sale_prices", Value: int64(unknownCount)},
		&pbg.State{Key: "last_sale_push", TimeValue: s.lastSalePush.Unix()},
		&pbg.State{Key: "last_sync_length", Text: fmt.Sprintf("%v", s.lastSyncLength)},
		&pbg.State{Key: "bad_folder", Value: int64(diffCount)},
		&pbg.State{Key: "bad_folder_category", Text: fmt.Sprintf("%v", badFolder)},
		&pbg.State{Key: "sales_pushes", Value: s.salesPushes},
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
		lastWantText:   "",
		scorer:         &prodScorer{},
		saleMap:        make(map[int32]*pb.Record),
		lastSalePush:   time.Now(),
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
		server.KSclient.Save(context.Background(), TOKEN, &pb.Token{Token: *token})
		log.Fatalf("Written TOKEN")
	}

	server.Register = server

	server.RegisterServer("recordcollection", false)
	server.RegisterRepeatingTask(server.runSync, "run_sync", time.Hour)
	server.RegisterRepeatingTask(server.runSyncWants, "run_sync_wants", time.Hour)
	server.RegisterRepeatingTask(server.pushWants, "push_wants", time.Minute)
	server.RegisterRepeatingTask(server.runRecache, "run_recache", time.Minute)
	server.RegisterRepeatingTask(server.runPush, "run_push", time.Minute)
	server.RegisterRepeatingTask(server.saveLoop, "save_loop", time.Minute)
	server.RegisterRepeatingTask(server.syncIssue, "sync_issue", time.Hour)
	server.RegisterRepeatingTask(server.pushSales, "push_sales", time.Minute)

	server.Serve()
}
