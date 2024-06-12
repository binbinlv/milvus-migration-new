package milvus2x

import (
	"context"
	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
	"github.com/zilliztech/milvus-migration/core/common"
	"github.com/zilliztech/milvus-migration/core/config"
	"github.com/zilliztech/milvus-migration/core/type/milvus2xtype"
	"github.com/zilliztech/milvus-migration/internal/log"
	"go.uber.org/zap"
	"io"
	"strconv"
	"time"
)

type Milvus23VerClient struct {
	_milvus   client.Client
	_iterator *client.QueryIterator
}

func (milvus23 *Milvus23VerClient) Close() error {
	if milvus23._milvus != nil {
		return milvus23._milvus.Close()
	}
	return nil
}

func NewMilvus23VerCli(Milvus2xConfig *config.Milvus2xConfig) (Milvus2xVersClient, error) {
	verCli, err := _createMilvus23VerClient(Milvus2xConfig)
	if err != nil {
		return nil, err
	}
	return verCli, nil
}

func (milvus23 *Milvus23VerClient) InitIterator(ctx context.Context, collCfg *milvus2xtype.CollectionCfg, batchSize int) error {

	count, err := milvus23.Count(ctx, collCfg)
	if err != nil {
		return err
	}
	collCfg.Rows = count

	log.Info("start iterator milvus collection", zap.String("collection", collCfg.Collection),
		zap.Int("BatchSize", batchSize))
	fieldNames := make([]string, 0, len(collCfg.Fields))
	for _, fieldCfg := range collCfg.Fields {
		fieldNames = append(fieldNames, fieldCfg.Name)
	}
	iteratorParam := client.NewQueryIteratorOption(collCfg.Collection).WithBatchSize(batchSize).WithExpr(common.EMPTY).WithOutputFields(fieldNames...)
	iterator, err := milvus23._milvus.QueryIterator(ctx, iteratorParam)
	if err != nil {
		return err
	}
	milvus23._iterator = iterator
	return nil
}

func (milvus23 *Milvus23VerClient) IterateNext(ctx context.Context) (*Milvus2xData, error) {

	var start time.Time
	if common.DEBUG {
		start = time.Now()
	}
	rs, err := milvus23._iterator.Next(ctx)
	if err != nil {
		if err == io.EOF {
			log.Info("milvus no data, iterator reach EOF")
			return &Milvus2xData{IsEmpty: true}, nil
		}
		return nil, err
	}
	columns := make([]entity.Column, 0, len(rs))
	for _, col := range rs {
		columns = append(columns, col)
		if common.DEBUG {
			log.Info("[Milvus2x] iterateNext data ======>", zap.String("colName", col.Name()), zap.Any("colLen", col.Len()))
		}
	}
	if common.DEBUG {
		log.Info("[Milvus2x] iterateNext data ======>", zap.Float64("Cost", time.Since(start).Seconds()))
	}
	return &Milvus2xData{Columns: columns, IsEmpty: false}, nil
}

func (milvus23 *Milvus23VerClient) Count(ctx context.Context, collCfg *milvus2xtype.CollectionCfg) (int64, error) {
	stats, err := milvus23._milvus.GetCollectionStatistics(ctx, collCfg.Collection)
	log.Info("[Milvus2x] GetCollectionStatistics ===>",
		zap.String("collection", collCfg.Collection), zap.Any("stats", stats))
	if err != nil {
		return 0, err
	}
	count := stats["row_count"]
	return strconv.ParseInt(count, 10, 64)
}

func (milvus23 *Milvus23VerClient) DescCollection(ctx context.Context, collectionName string) (*entity.Collection, error) {
	collEntity, err := milvus23._milvus.DescribeCollection(ctx, collectionName)
	if err != nil {
		return nil, err
	}
	return collEntity, nil
}

func _createMilvus23VerClient(cfg *config.Milvus2xConfig) (*Milvus23VerClient, error) {

	log.Info("[Milvus23x] begin to new milvus client", zap.String("endPoint", cfg.Endpoint))

	var milvus client.Client
	var err error
	ctx := context.Background()

	if cfg.UserName == "" {
		log.Info("[Milvus23x] find username is empty, will use NewDefaultGrpcClient() to new client")
		milvus, err = client.NewDefaultGrpcClient(ctx, cfg.Endpoint)
	} else {
		log.Info("[Milvus23x] find username not empty, will use NewDefaultGrpcClientWithURI() to new client")
		milvus, err = client.NewDefaultGrpcClientWithURI(ctx, cfg.Endpoint, cfg.UserName, cfg.Password)
	}
	if err != nil {
		log.Error("[Milvus23x] new milvus client error", zap.Error(err))
		return nil, err
	}

	log.Info("[Milvus23x] begin to test connect",
		zap.String("endpoint", cfg.Endpoint),
		zap.String("username", cfg.UserName))
	_, err = milvus.HasCollection(ctx, "test")
	if err != nil {
		return nil, err
	}
	return &Milvus23VerClient{_milvus: milvus}, nil
}
