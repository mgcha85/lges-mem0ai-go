package vectorstore

import (
	"context"
	"fmt"
	"log"

	pb "github.com/qdrant/go-client/qdrant"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// QdrantStore implements VectorStore using Qdrant.
type QdrantStore struct {
	conn           *grpc.ClientConn
	pointsClient   pb.PointsClient
	collectClient  pb.CollectionsClient
	collectionName string
	vectorSize     uint64
}

// NewQdrantStore creates a new Qdrant vector store client.
func NewQdrantStore(host string, port int, collectionName string, vectorSize int) (*QdrantStore, error) {
	addr := fmt.Sprintf("%s:%d", host, port)
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to qdrant at %s: %w", addr, err)
	}

	qs := &QdrantStore{
		conn:           conn,
		pointsClient:   pb.NewPointsClient(conn),
		collectClient:  pb.NewCollectionsClient(conn),
		collectionName: collectionName,
		vectorSize:     uint64(vectorSize),
	}

	if err := qs.ensureCollection(); err != nil {
		conn.Close()
		return nil, err
	}

	return qs, nil
}

func (q *QdrantStore) ensureCollection() error {
	ctx := context.Background()
	_, err := q.collectClient.Get(ctx, &pb.GetCollectionInfoRequest{
		CollectionName: q.collectionName,
	})
	if err != nil {
		// Collection doesn't exist, create it
		_, err = q.collectClient.Create(ctx, &pb.CreateCollection{
			CollectionName: q.collectionName,
			VectorsConfig: &pb.VectorsConfig{
				Config: &pb.VectorsConfig_Params{
					Params: &pb.VectorParams{
						Size:     q.vectorSize,
						Distance: pb.Distance_Cosine,
					},
				},
			},
		})
		if err != nil {
			return fmt.Errorf("failed to create collection %s: %w", q.collectionName, err)
		}
		log.Printf("Created Qdrant collection: %s", q.collectionName)
	}
	return nil
}

// Insert adds vectors with their IDs and payloads to Qdrant.
func (q *QdrantStore) Insert(vectors [][]float32, ids []string, payloads []map[string]interface{}) error {
	var points []*pb.PointStruct
	for i := range vectors {
		payload := convertToQdrantPayload(payloads[i])
		points = append(points, &pb.PointStruct{
			Id: &pb.PointId{
				PointIdOptions: &pb.PointId_Uuid{Uuid: ids[i]},
			},
			Vectors: &pb.Vectors{
				VectorsOptions: &pb.Vectors_Vector{
					Vector: &pb.Vector{Data: vectors[i]},
				},
			},
			Payload: payload,
		})
	}

	_, err := q.pointsClient.Upsert(context.Background(), &pb.UpsertPoints{
		CollectionName: q.collectionName,
		Points:         points,
	})
	if err != nil {
		return fmt.Errorf("qdrant upsert error: %w", err)
	}
	return nil
}

// Search finds similar vectors in Qdrant.
func (q *QdrantStore) Search(query string, vector []float32, limit int, filters map[string]string) ([]SearchResult, error) {
	var filter *pb.Filter
	if len(filters) > 0 {
		filter = buildQdrantFilter(filters)
	}

	resp, err := q.pointsClient.Search(context.Background(), &pb.SearchPoints{
		CollectionName: q.collectionName,
		Vector:         vector,
		Limit:          uint64(limit),
		WithPayload:    &pb.WithPayloadSelector{SelectorOptions: &pb.WithPayloadSelector_Enable{Enable: true}},
		Filter:         filter,
	})
	if err != nil {
		return nil, fmt.Errorf("qdrant search error: %w", err)
	}

	var results []SearchResult
	for _, r := range resp.GetResult() {
		results = append(results, SearchResult{
			ID:      r.GetId().GetUuid(),
			Score:   r.GetScore(),
			Payload: convertFromQdrantPayload(r.GetPayload()),
		})
	}
	return results, nil
}

// Get retrieves a single record by ID.
func (q *QdrantStore) Get(id string) (*VectorRecord, error) {
	resp, err := q.pointsClient.Get(context.Background(), &pb.GetPoints{
		CollectionName: q.collectionName,
		Ids: []*pb.PointId{
			{PointIdOptions: &pb.PointId_Uuid{Uuid: id}},
		},
		WithPayload: &pb.WithPayloadSelector{SelectorOptions: &pb.WithPayloadSelector_Enable{Enable: true}},
	})
	if err != nil {
		return nil, fmt.Errorf("qdrant get error: %w", err)
	}

	if len(resp.GetResult()) == 0 {
		return nil, fmt.Errorf("memory not found: %s", id)
	}

	point := resp.GetResult()[0]
	return &VectorRecord{
		ID:      point.GetId().GetUuid(),
		Payload: convertFromQdrantPayload(point.GetPayload()),
	}, nil
}

// List returns all records matching filters.
func (q *QdrantStore) List(filters map[string]string, limit int) ([]VectorRecord, error) {
	var filter *pb.Filter
	if len(filters) > 0 {
		filter = buildQdrantFilter(filters)
	}

	resp, err := q.pointsClient.Scroll(context.Background(), &pb.ScrollPoints{
		CollectionName: q.collectionName,
		Filter:         filter,
		Limit:          ptrUint32(uint32(limit)),
		WithPayload:    &pb.WithPayloadSelector{SelectorOptions: &pb.WithPayloadSelector_Enable{Enable: true}},
	})
	if err != nil {
		return nil, fmt.Errorf("qdrant scroll error: %w", err)
	}

	var records []VectorRecord
	for _, point := range resp.GetResult() {
		records = append(records, VectorRecord{
			ID:      point.GetId().GetUuid(),
			Payload: convertFromQdrantPayload(point.GetPayload()),
		})
	}
	return records, nil
}

// Update updates a vector record in Qdrant.
func (q *QdrantStore) Update(id string, vector []float32, payload map[string]interface{}) error {
	pointID := &pb.PointId{PointIdOptions: &pb.PointId_Uuid{Uuid: id}}

	// Update payload
	if payload != nil {
		_, err := q.pointsClient.SetPayload(context.Background(), &pb.SetPayloadPoints{
			CollectionName: q.collectionName,
			Payload:        convertToQdrantPayload(payload),
			PointsSelector: &pb.PointsSelector{
				PointsSelectorOneOf: &pb.PointsSelector_Points{
					Points: &pb.PointsIdsList{Ids: []*pb.PointId{pointID}},
				},
			},
		})
		if err != nil {
			return fmt.Errorf("qdrant set payload error: %w", err)
		}
	}

	// Update vector if provided
	if vector != nil {
		_, err := q.pointsClient.UpdateVectors(context.Background(), &pb.UpdatePointVectors{
			CollectionName: q.collectionName,
			Points: []*pb.PointVectors{
				{
					Id: pointID,
					Vectors: &pb.Vectors{
						VectorsOptions: &pb.Vectors_Vector{
							Vector: &pb.Vector{Data: vector},
						},
					},
				},
			},
		})
		if err != nil {
			return fmt.Errorf("qdrant update vectors error: %w", err)
		}
	}

	return nil
}

// Delete removes a record by ID.
func (q *QdrantStore) Delete(id string) error {
	_, err := q.pointsClient.Delete(context.Background(), &pb.DeletePoints{
		CollectionName: q.collectionName,
		Points: &pb.PointsSelector{
			PointsSelectorOneOf: &pb.PointsSelector_Points{
				Points: &pb.PointsIdsList{
					Ids: []*pb.PointId{
						{PointIdOptions: &pb.PointId_Uuid{Uuid: id}},
					},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("qdrant delete error: %w", err)
	}
	return nil
}

// Reset deletes and recreates the collection.
func (q *QdrantStore) Reset() error {
	ctx := context.Background()
	_, err := q.collectClient.Delete(ctx, &pb.DeleteCollection{
		CollectionName: q.collectionName,
	})
	if err != nil {
		return fmt.Errorf("qdrant delete collection error: %w", err)
	}

	return q.ensureCollection()
}

// Close closes the gRPC connection.
func (q *QdrantStore) Close() error {
	return q.conn.Close()
}

// --- Helper functions ---

func buildQdrantFilter(filters map[string]string) *pb.Filter {
	var conditions []*pb.Condition
	for key, value := range filters {
		conditions = append(conditions, &pb.Condition{
			ConditionOneOf: &pb.Condition_Field{
				Field: &pb.FieldCondition{
					Key: key,
					Match: &pb.Match{
						MatchValue: &pb.Match_Keyword{Keyword: value},
					},
				},
			},
		})
	}
	return &pb.Filter{Must: conditions}
}

func convertToQdrantPayload(data map[string]interface{}) map[string]*pb.Value {
	payload := make(map[string]*pb.Value)
	for k, v := range data {
		switch val := v.(type) {
		case string:
			payload[k] = &pb.Value{Kind: &pb.Value_StringValue{StringValue: val}}
		case float64:
			payload[k] = &pb.Value{Kind: &pb.Value_DoubleValue{DoubleValue: val}}
		case int:
			payload[k] = &pb.Value{Kind: &pb.Value_IntegerValue{IntegerValue: int64(val)}}
		case int64:
			payload[k] = &pb.Value{Kind: &pb.Value_IntegerValue{IntegerValue: val}}
		case bool:
			payload[k] = &pb.Value{Kind: &pb.Value_BoolValue{BoolValue: val}}
		default:
			payload[k] = &pb.Value{Kind: &pb.Value_StringValue{StringValue: fmt.Sprintf("%v", val)}}
		}
	}
	return payload
}

func convertFromQdrantPayload(payload map[string]*pb.Value) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range payload {
		switch val := v.GetKind().(type) {
		case *pb.Value_StringValue:
			result[k] = val.StringValue
		case *pb.Value_DoubleValue:
			result[k] = val.DoubleValue
		case *pb.Value_IntegerValue:
			result[k] = val.IntegerValue
		case *pb.Value_BoolValue:
			result[k] = val.BoolValue
		default:
			result[k] = fmt.Sprintf("%v", v)
		}
	}
	return result
}

func ptrUint32(v uint32) *uint32 {
	return &v
}
