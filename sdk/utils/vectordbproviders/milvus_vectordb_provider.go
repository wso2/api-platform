package vectordbproviders

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/goccy/go-json"
	"github.com/google/uuid"
	"github.com/milvus-io/milvus/client/v2/entity"
	"github.com/milvus-io/milvus/client/v2/index"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
	"github.com/milvus-io/milvus/pkg/v2/common"
)

// MilvusVectorDBProvider implements the VectorDBProvider interface for Milvus
type MilvusVectorDBProvider struct {
	milvusURL      string
	dimension      int
	ttl            int
	client         *milvusclient.Client
	collectionName string
}

// Init initializes the Milvus vector DB provider with configuration
func (m *MilvusVectorDBProvider) Init(config VectorDBProviderConfig) error {
	err := ValidateVectorStoreConfigProps(config)
	if err != nil {
		return err
	}
	embeddingDimension := config.EmbeddingDimension
	m.milvusURL = config.DBHost + ":" + strconv.Itoa(config.DBPort)
	m.collectionName = fmt.Sprintf("%s_%s", VectorIndexPrefix, embeddingDimension)
	m.dimension, _ = strconv.Atoi(embeddingDimension)

	m.ttl = DefaultTTL
	if config.TTL != "" {
		parsedTTL, err := strconv.Atoi(config.TTL)
		if err != nil {
			return fmt.Errorf("invalid TTL value: %v", err)
		}
		m.ttl = parsedTTL
	}

	m.client, err = milvusclient.New(context.Background(), &milvusclient.ClientConfig{
		Address: m.milvusURL,
	})
	if err != nil {
		return fmt.Errorf("failed to connect to MilvusDB: %v", err)
	}

	return nil
}

// GetType returns the type of the provider
func (m *MilvusVectorDBProvider) GetType() string {
	return "MILVUS"
}

// CreateIndex creates an index for Milvus
func (m *MilvusVectorDBProvider) CreateIndex() error {
	// Check if collection exists
	exists, err := m.client.HasCollection(context.Background(), milvusclient.NewHasCollectionOption(m.collectionName))
	if err != nil {
		return fmt.Errorf("failed to check if collection '%s' exists: %v", m.collectionName, err)
	}
	if exists {
		return nil
	}

	schema := entity.NewSchema().WithDynamicFieldEnabled(true).
		WithField(entity.NewField().
			WithName("id").
			WithDataType(entity.FieldTypeVarChar).
			WithIsPrimaryKey(true).
			WithIsAutoID(false).
			WithMaxLength(36),
		).
		WithField(entity.NewField().
			WithName("created_at").
			WithDataType(entity.FieldTypeInt64).
			WithDim(4),
		).
		WithField(entity.NewField().
			WithName("api_id").
			WithDataType(entity.FieldTypeVarChar).
			WithMaxLength(36),
		).
		WithField(entity.NewField().
			WithName(embeddingField).
			WithDataType(entity.FieldTypeFloatVector).
			WithDim(int64(m.dimension)),
		).
		WithField(entity.NewField().
			WithName(responseField).
			WithDataType(entity.FieldTypeVarChar).
			WithMaxLength(65535).
			WithNullable(false),
		)

	// Define HNSW Index Parameter
	hnswIndex := index.NewHNSWIndex(
		entity.L2, // MetricType: L2, IP, or COSINE
		64,        // M: Maximum number of neighbors per node
		100,       // efConstruction: Number of candidates during construction
	)

	// Create the Index Option
	// This option links the index configuration to a specific field and gives the index a name.
	indexOptions := []milvusclient.CreateIndexOption{milvusclient.NewCreateIndexOption(m.collectionName, embeddingField, hnswIndex)}

	err = m.client.CreateCollection(context.Background(), milvusclient.NewCreateCollectionOption(m.collectionName, schema).
		WithIndexOptions(indexOptions...).
		WithProperty(common.CollectionTTLConfigKey, m.ttl))
	if err != nil {
		return fmt.Errorf("failed to create collection: %w", err)
	}
	fmt.Printf("Collection successfully created with the given parameters")
	return nil
}

// Store stores the embeddings and associated response in Milvus
func (m *MilvusVectorDBProvider) Store(embeddings []float32, response CacheResponse, filter map[string]interface{}) error {
	id := uuid.New().String()
	ctx := filter["ctx"].(context.Context)
	responseBytes, err := SerializeObject(response)
	if err != nil {
		return err
	}
	responseString := string(responseBytes)

	// Construct a row to insert
	dbRow := map[string]interface{}{
		"id":           id,
		"created_at":   time.Now().Unix(),
		"api_id":       filter["api_id"].(string),
		embeddingField: embeddings,
		responseField:  responseString,
	}

	_, err = m.client.Insert(ctx, milvusclient.NewRowBasedInsertOption(m.collectionName, dbRow))
	if err != nil {
		return fmt.Errorf("failed to insert data into Milvus: %w", err)
	}

	return nil
}

// Retrieve retrieves the most similar embedding from Milvus
func (m *MilvusVectorDBProvider) Retrieve(embeddings []float32, filter map[string]interface{}) (CacheResponse, error) {
	ctx, ok := filter["ctx"].(context.Context)
	if !ok {
		return CacheResponse{}, fmt.Errorf("missing or invalid context in filter")
	}
	loadTask, err := m.client.LoadCollection(ctx, milvusclient.NewLoadCollectionOption(m.collectionName))
	if err != nil {
		return CacheResponse{}, fmt.Errorf("failed to load collection: %w", err)
	}
	err = loadTask.Await(ctx)
	if err != nil {
		return CacheResponse{}, fmt.Errorf("error in fetching the collection: %w", err)
	}

	resultSets, err := m.client.Search(ctx, milvusclient.NewSearchOption(
		m.collectionName, // collectionName
		1,                // limit
		[]entity.Vector{entity.FloatVector(embeddings)},
	).WithConsistencyLevel(entity.ClStrong).
		WithANNSField(embeddingField).
		WithFilter("api_id == '"+filter["api_id"].(string)+"'").
		WithFilter("created_at >= "+strconv.FormatInt(time.Now().Unix()-int64(m.ttl), 10)).
		WithOutputFields(responseField))

	if err != nil {
		return CacheResponse{}, fmt.Errorf("failed to search in Milvus: %w", err)
	}

	// Defensive check to avoid index out of range
	if len(resultSets) == 0 {
		return CacheResponse{}, nil
	}
	// Grab the first result set (we only asked for one vector batch)
	rs := resultSets[0]
	if rs.ResultCount == 0 {
		return CacheResponse{}, nil
	}

	// Raw Milvus distance → similarity score
	simScore := float64(rs.Scores[0])
	response := rs.GetColumn("response").FieldData().GetScalars()

	// Check for threshold and comapre with similarity score
	thrRaw, ok := filter["threshold"].(string)
	if !ok {
		return CacheResponse{}, fmt.Errorf("missing threshold")
	}
	thr, err := strconv.ParseFloat(thrRaw, 64)
	if err != nil || thr == 0 {
		return CacheResponse{}, fmt.Errorf("bad threshold value found: %w", err)
	}

	if simScore > thr {
		return CacheResponse{}, nil
	}
	var resp CacheResponse
	stringArray := response.GetStringData()
	if len(stringArray.Data) == 0 {
		return CacheResponse{}, fmt.Errorf("no response data found")
	}
	responseBytes := []byte(stringArray.Data[0])
	err = json.Unmarshal(responseBytes, &resp)
	if err != nil {
		return CacheResponse{}, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return resp, nil
}

// Close closes the Milvus client connection
func (m *MilvusVectorDBProvider) Close() error {
	if m.client != nil {
		return m.client.Close(context.Background())
	}
	return nil
}
