package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"time"

	"github.com/brotherlogic/godiscogs"
	"github.com/brotherlogic/goserver"
	"github.com/golang/protobuf/proto"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pbd "github.com/brotherlogic/godiscogs"
	pbg "github.com/brotherlogic/goserver/proto"
	"github.com/brotherlogic/goserver/utils"
	pbks "github.com/brotherlogic/keystore/proto"
	pb "github.com/brotherlogic/recordcollection/proto"
	pbrm "github.com/brotherlogic/recordmover/proto"
	pbrp "github.com/brotherlogic/recordprocess/proto"
	pbro "github.com/brotherlogic/recordsorganiser/proto"

	_ "net/http/pprof"
)

var (
	stateCount = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "recordcollection_recordstate",
		Help: "The state of records in the collection",
	}, []string{"state"})
	folderCount = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "recordcollection_recordfolder",
		Help: "The size of each folder",
	}, []string{"folder"})
	updateIn = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "recordcollection_update_in",
		Help: "Last update time",
	}, []string{"status"})
)

type quotaChecker interface {
	hasQuota(ctx context.Context, folder int32) (*pbro.QuotaResponse, error)
}

type moveRecorder interface {
	moveRecord(ctx context.Context, record *pb.Record, oldFolder, newFolder int32) error
}

type prodMoveRecorder struct {
	dial func(ctx context.Context, server string) (*grpc.ClientConn, error)
}

func (p *prodMoveRecorder) moveRecord(ctx context.Context, record *pb.Record, oldFolder, newFolder int32) error {
	conn, err := p.dial(ctx, "recordmover")
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
	dial func(ctx context.Context, server string) (*grpc.ClientConn, error)
}

func (p *prodQuotaChecker) hasQuota(ctx context.Context, folder int32) (*pbro.QuotaResponse, error) {
	conn, err := p.dial(ctx, "recordsorganiser")
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
	MoveToFolder(folderID, releaseID, instanceID, newFolderID int) (string, error)
	DeleteInstance(folderID, releaseID, instanceID int) string
	SellRecord(releaseID int, price float32, state string, condition, sleeve string) int
	GetSalePrice(releaseID int) (float32, error)
	RemoveFromWantlist(releaseID int)
	AddToWantlist(releaseID int) error
	UpdateSalePrice(saleID int, releaseID int, condition, sleeve string, price float32) error
	GetCurrentSalePrice(saleID int) float32
	GetCurrentSaleState(saleID int) godiscogs.SaleState
	RemoveFromSale(saleID int, releaseID int) error
	ExpireSale(saleID int, releaseID int, price float32) error
	GetInventory() ([]*godiscogs.ForSale, error)
	GetInstanceInfo(ID int32) (map[int32]*godiscogs.InstanceInfo, error)
}

type scorer interface {
	GetScore(ctx context.Context, instanceID int32) (float32, error)
}
type prodScorer struct {
	dial func(ctx context.Context, server string) (*grpc.ClientConn, error)
}

func (p *prodScorer) GetScore(ctx context.Context, instanceID int32) (float32, error) {
	conn, err := p.dial(ctx, "recordprocess")
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
	retr          saver
	scorer        scorer
	quota         quotaChecker
	mover         moveRecorder
	TimeoutLoad   bool
	disableSales  bool
	updateFanout  chan int32
	fanoutServers []string
	repeatCount   map[int32]int
	repeatError   map[int32]error
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

var (
	sizes = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "recordcollection_sizes",
		Help: "The state of records in the collection",
	}, []string{"map"})

	wants = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "recordcollection_wants",
		Help: "The state of records in the collection",
	}, []string{"active"})
)

func (s *Server) readRecordCollection(ctx context.Context) (*pb.RecordCollection, error) {
	collection := &pb.RecordCollection{}
	data, _, err := s.KSclient.Read(ctx, KEY, collection)

	if err != nil {
		return nil, err
	}

	collection = data.(*pb.RecordCollection)

	// Create the instance to recahe map
	if collection.InstanceToRecache == nil {
		collection.InstanceToRecache = make(map[int32]int64)
	}

	if collection.InstanceToLastSalePriceUpdate == nil {
		collection.InstanceToLastSalePriceUpdate = make(map[int32]int64)
	}

	if collection.InstanceToFolder == nil {
		log.Fatalf("Unable to get the folder: %v", collection)
	}

	if collection.InstanceToId == nil {
		collection.InstanceToId = make(map[int32]int32)
	}

	if collection.InstanceToUpdate == nil {
		collection.InstanceToUpdate = make(map[int32]int64)
	}

	if collection.InstanceToUpdateIn == nil {
		collection.InstanceToUpdateIn = make(map[int32]int64)
	}

	if collection.InstanceToCategory == nil {
		collection.InstanceToCategory = make(map[int32]pb.ReleaseMetadata_Category)
	}

	if collection.InstanceToMaster == nil {
		log.Fatalf("Unable to get the master: %v", collection)
	}

	if collection.GetOldestRecord() == 0 {
		collection.OldestRecord = time.Now().Unix()
	}

	s.updateMetrics(collection)

	count := 0
	for _, w := range collection.GetNewWants() {
		if w.GetMetadata().GetActive() {
			count++
		}
	}
	wants.With(prometheus.Labels{"active": "true"}).Set(float64(count))
	wants.With(prometheus.Labels{"active": "false"}).Set(float64(len(collection.GetNewWants()) - count))

	return collection, nil
}

func (s *Server) updateMetrics(collection *pb.RecordCollection) {
	sizes.With(prometheus.Labels{"map": "master"}).Set(float64(len(collection.GetInstanceToMaster())))
	sizes.With(prometheus.Labels{"map": "update"}).Set(float64(len(collection.GetInstanceToUpdate())))
	sizes.With(prometheus.Labels{"map": "category"}).Set(float64(len(collection.GetInstanceToCategory())))
	sizes.With(prometheus.Labels{"map": "folder"}).Set(float64(len(collection.GetInstanceToFolder())))
	sizes.With(prometheus.Labels{"map": "updatein"}).Set(float64(len(collection.GetInstanceToUpdateIn())))

	minT := time.Now().Unix()
	maxT := int64(0)
	for iid, up := range collection.GetInstanceToUpdateIn() {
		value := collection.GetInstanceToUpdate()[iid] - up
		if value < minT {
			minT = value
		}
		if value > maxT {
			maxT = value
		}
	}
	updateIn.With(prometheus.Labels{"status": "max"}).Set(float64(maxT))
	updateIn.With(prometheus.Labels{"status": "min"}).Set(float64(minT))
}

func (s *Server) saveRecordCollection(ctx context.Context, collection *pb.RecordCollection) error {
	s.updateMetrics(collection)
	return s.KSclient.Save(ctx, KEY, collection)
}

func (s *Server) deleteRecord(ctx context.Context, i int32) error {
	if !s.SkipLog {
		conn, err := s.FDialServer(ctx, "keystore")
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
	if r.GetMetadata().GoalFolder == 0 {
		s.RaiseIssue("Save Error", fmt.Sprintf("Trying to save a record without a goal folder: %v", r))
		return fmt.Errorf("No goal folder")
	}

	err := s.KSclient.Save(ctx, fmt.Sprintf("%v%v", SAVEKEY, r.GetRelease().InstanceId), r)

	collection, err := s.readRecordCollection(ctx)
	if err != nil {
		return err
	}
	save := false
	if collection.GetInstanceToFolder()[r.GetRelease().InstanceId] != r.GetRelease().FolderId {
		collection.GetInstanceToFolder()[r.GetRelease().InstanceId] = r.GetRelease().FolderId
		save = true
	}

	if collection.InstanceToCategory[r.GetRelease().InstanceId] != r.GetMetadata().Category {
		collection.InstanceToCategory[r.GetRelease().InstanceId] = r.GetMetadata().Category
		save = true
	}

	if collection.InstanceToUpdate[r.GetRelease().InstanceId] != r.GetMetadata().LastUpdateTime {
		collection.InstanceToUpdate[r.GetRelease().InstanceId] = r.GetMetadata().LastUpdateTime
		save = true
	}

	if collection.InstanceToMaster[r.GetRelease().InstanceId] != r.GetRelease().MasterId {
		collection.InstanceToMaster[r.GetRelease().InstanceId] = r.GetRelease().MasterId
		save = true
	}

	if collection.InstanceToId[r.GetRelease().InstanceId] != r.GetRelease().Id {
		collection.InstanceToId[r.GetRelease().InstanceId] = r.GetRelease().Id
		save = true
	}

	if collection.InstanceToLastSalePriceUpdate[r.GetRelease().InstanceId] != r.GetMetadata().GetSalePriceUpdate() {
		collection.InstanceToLastSalePriceUpdate[r.GetRelease().InstanceId] = r.GetMetadata().GetSalePriceUpdate()
		save = true
	}

	if collection.GetInstanceToUpdateIn()[r.GetRelease().InstanceId] != r.GetMetadata().GetLastUpdateIn() {
		collection.InstanceToUpdateIn[r.GetRelease().InstanceId] = r.GetMetadata().GetLastUpdateIn()
		save = true
	}

	if r.GetMetadata().SaleDirty {
		collection.SaleUpdates = append(collection.SaleUpdates, r.GetRelease().GetInstanceId())
	}

	if r.GetMetadata().LastCache == 0 || r.GetMetadata().LastCache == 1 {
		collection.InstanceToRecache[r.GetRelease().InstanceId] = time.Now().Unix()
	} else {
		collection.InstanceToRecache[r.GetRelease().InstanceId] = time.Unix(r.GetMetadata().LastCache, 0).Add(time.Hour * 24 * 7 * 2).Unix()
	}

	if r.GetMetadata().GetDirty() {
		collection.NeedsPush = append(collection.NeedsPush, r.GetRelease().InstanceId)
	}

	if save {
		s.saveRecordCollection(ctx, collection)
	}

	counts := make(map[int32]int)
	for _, folder := range collection.GetInstanceToFolder() {
		counts[folder]++
	}
	for folder, count := range counts {
		folderCount.With(prometheus.Labels{"folder": fmt.Sprintf("%v", folder)}).Set(float64(count))
	}

	return err
}

var (
	cacheSize = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "recordcollection_cachesize",
		Help: "The size of the record cache",
	})
)

func (s *Server) loadRecord(ctx context.Context, id int32, validate bool) (*pb.Record, error) {
	if s.TimeoutLoad {
		return nil, status.Error(codes.DeadlineExceeded, "Force DE")
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

	// Let's make sure this is in the folder map
	if validate {
		collection, err := s.readRecordCollection(ctx)

		if err == nil {
			if collection.GetInstanceToFolder()[recordToReturn.GetRelease().GetInstanceId()] != recordToReturn.GetRelease().GetFolderId() {
				s.saveRecord(ctx, recordToReturn)
			}
		}

	}

	return recordToReturn, nil
}

func (s *Server) loadUpdates(ctx context.Context, id int32) (*pb.Updates, error) {
	data, _, err := s.KSclient.Read(ctx, fmt.Sprintf("%v%v.updates", SAVEKEY, id), &pb.Updates{})

	if err != nil {
		return nil, err
	}

	if proto.Size(data) == 0 {
		return nil, fmt.Errorf("Error on read for %v", id)
	}

	updates := data.(*pb.Updates)

	return updates, nil
}

func (s *Server) saveUpdates(ctx context.Context, id int32, updates *pb.Updates) error {
	return s.KSclient.Save(ctx, fmt.Sprintf("%v%v.updates", SAVEKEY, id), updates)
}

func (s *Server) getRecord(ctx context.Context, id int32) (*pb.Record, error) {
	r, err := s.loadRecord(ctx, id, false)

	if err != nil {
		return nil, err
	}

	collection, err := s.readRecordCollection(ctx)

	if err != nil {
		return nil, err
	}

	if len(r.GetRelease().GetLabels()) == 0 {
		r.GetMetadata().LastCache = 1
	}

	return r, s.saveRecordCollection(ctx, collection)

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
	return nil
}

// Mote promotes/demotes this server
func (s *Server) Mote(ctx context.Context, master bool) error {
	return nil
}

// GetState gets the state of the server
func (s *Server) GetState() []*pbg.State {
	return []*pbg.State{}
}

// Init builds out a server
func Init() *Server {
	s := &Server{
		GoServer:     &goserver.GoServer{},
		updateFanout: make(chan int32, 100),
		fanoutServers: []string{
			"cdprocessor",
			"recordbudget",
			"recordmatcher",
			"recordsorganiser",
			"recordmover",
			"recordscores",
			"recordprocess",
			"recordprinter",
			"recordsales",
			"recordwants",
			"digitalwantlist",
			"recordstats",
			"recordalerting"},
		repeatCount: make(map[int32]int),
		repeatError: make(map[int32]error),
	}
	s.scorer = &prodScorer{s.FDialServer}
	s.quota = &prodQuotaChecker{s.FDialServer}
	s.mover = &prodMoveRecorder{s.FDialServer}
	return s
}

func (s *Server) updateSalePrice(ctx context.Context) error {
	collection, err := s.readRecordCollection(ctx)
	if err != nil {
		return err
	}

	for id, val := range collection.GetInstanceToLastSalePriceUpdate() {
		if time.Now().Sub(time.Unix(val, 0)) > time.Hour*24*2 {
			r, err := s.loadRecord(ctx, id, false)
			if err != nil {
				return err
			}
			price, err := s.retr.GetSalePrice(int(r.GetRelease().Id))
			if err != nil {
				s.Log(fmt.Sprintf("Sale price error for %v -> %v", r.GetRelease().Id, err))
				return err
			}
			s.Log(fmt.Sprintf("Sale Price Retrieved %v, %v -> %v", price, err, r.GetRelease().Id))
			r.GetMetadata().CurrentSalePrice = int32(price * 100)
			r.GetMetadata().SalePriceUpdate = time.Now().Unix()
			s.saveRecord(ctx, r)
			return nil
		}
	}

	if len(collection.InstanceToLastSalePriceUpdate) != len(collection.InstanceToFolder) {
		for id := range collection.InstanceToFolder {
			if _, ok := collection.InstanceToLastSalePriceUpdate[id]; !ok {
				collection.InstanceToLastSalePriceUpdate[id] = 1
			}
		}
	}

	return s.saveRecordCollection(ctx, collection)
}

func (s *Server) updateRecordSalePrice(ctx context.Context, r *pb.Record) error {
	price, err := s.retr.GetSalePrice(int(r.GetRelease().Id))
	if err != nil {
		s.Log(fmt.Sprintf("Sale price error for %v -> %v", r.GetRelease().Id, err))
		return err
	}
	s.Log(fmt.Sprintf("Sale Price Retrieved %v, %v -> %v", price, err, r.GetRelease().Id))
	r.GetMetadata().CurrentSalePrice = int32(price * 100)
	r.GetMetadata().SalePriceUpdate = time.Now().Unix()
	return s.saveRecord(ctx, r)
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

	err := server.RegisterServerV2("recordcollection", false, true)
	if err != nil {
		return
	}

	server.disableSales = false

	tType := &pb.Token{}
	ctx, cancel := utils.ManualContext("rci", "rci", time.Minute, false)
	tResp, _, err := server.KSclient.Read(ctx, TOKEN, tType)
	if err != nil {
		log.Fatalf("Unable to read discogs token")
	}

	if len(tResp.(*pb.Token).Token) == 0 {
		log.Fatalf("Read an empty token: %v", tResp)

	}
	server.retr = pbd.NewDiscogsRetriever(tResp.(*pb.Token).Token, server.Log)

	go server.runUpdateFanout()

	// Seed the fanout queue with some records that need an update
	stop, err := server.Elect()
	if err != nil {
		log.Fatalf("Bad election: %v", err)
	}

	collection, err := server.readRecordCollection(ctx)
	if err != nil {
		log.Falatf("Unable to read collection: %v", err)
	}
	for id, _ := range collection.GetInstanceToUpdate() {
		if (collection.GetInstanceToUpdateIn()[id] == 0 || collection.GetInstanceToUpdate()[id]-collection.GetInstanceToUpdateIn()[id] < 0) && len(server.updateFanout) < 50 {
			server.UpdateRecord(ctx, &pb.UpdateRecordRequest{Reason: "UpdateSeed", Update: &pb.Record{Release: &pbd.Release{InstanceId: id}}})
		}
	}
	cancel()
	stop()

	server.Serve()
}
