package cache

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	IndexName           = "idx:semantic_cache"
	KeyPrefix           = "cache:"
	VectorDim           = 384
	SimilarityThreshold = 0.92
	DefaultTTL          = 24 * time.Hour
)

type SourceDoc struct {
	ID       string            `json:"id"`
	Content  string            `json:"content"`
	Score    float32           `json:"score"`
	Metadata map[string]string `json:"metadata"`
}

type CachedResponse struct {
	Answer    string      `json:"answer"`
	Sources   []SourceDoc `json:"sources"`
	QueryText string      `json:"query_text"`
	CreatedAt time.Time   `json:"created_at"`
}

type SemanticCache struct {
	client *redis.Client
}

func NewSemanticCache(client *redis.Client) *SemanticCache {
	sc := &SemanticCache{client: client}
	_ = sc.ensureIndex(context.Background())
	return sc
}

func (sc *SemanticCache) ensureIndex(ctx context.Context) error {
	_, err := sc.client.Do(ctx, "FT.INFO", IndexName).Result()
	if err == nil {
		return nil
	}

	cmd := sc.client.Do(ctx,
		"FT.CREATE", IndexName,
		"ON", "HASH",
		"PREFIX", "1", KeyPrefix,
		"SCHEMA",
		"query_text", "TEXT",
		"vector", "VECTOR", "HNSW", "6",
		"TYPE", "FLOAT32",
		"DIM", VectorDim,
		"DISTANCE_METRIC", "COSINE",
	)

	return cmd.Err()
}

func (sc *SemanticCache) Get(ctx context.Context, vector []float32) (*CachedResponse, bool, error) {
	vecBytes := float32ToBytes(vector)

	query := "*=>[KNN 1 @vector $vec AS score]"
	cmd := sc.client.Do(ctx,
		"FT.SEARCH", IndexName, query,
		"PARAMS", "2", "vec", vecBytes,
		"SORTBY", "score", "ASC",
		"RETURN", "2", "payload", "score",
		"DIALECT", "2",
	)

	res, err := cmd.Result()
	if err != nil {
		return nil, false, nil
	}

	results, ok := res.(map[interface{}]interface{})
	if !ok {
		return nil, false, nil
	}

	totalResults, _ := results["total_results"].(int64)
	if totalResults == 0 {
		return nil, false, nil // Miss
	}

	docs, ok := results["results"].([]interface{})
	if !ok || len(docs) == 0 {
		return nil, false, nil
	}

	firstDoc := docs[0].(map[interface{}]interface{})
	extraAttributes := firstDoc["extra_attributes"].(map[interface{}]interface{})

	scoreStr, _ := extraAttributes["score"].(string)
	var distance float64
	fmt.Sscanf(scoreStr, "%f", &distance)

	similarity := 1.0 - distance
	if similarity < SimilarityThreshold {
		return nil, false, nil
	}

	payloadStr, _ := extraAttributes["payload"].(string)
	var cached CachedResponse
	if err := json.Unmarshal([]byte(payloadStr), &cached); err != nil {
		return nil, false, err
	}

	return &cached, true, nil
}

func (sc *SemanticCache) Set(ctx context.Context, queryText string, vector []float32, resp *CachedResponse) error {
	hash := sha256.Sum256([]byte(queryText))
	key := KeyPrefix + hex.EncodeToString(hash[:16])

	resp.QueryText = queryText
	resp.CreatedAt = time.Now()

	payloadBytes, err := json.Marshal(resp)
	if err != nil {
		return err
	}

	vecBytes := float32ToBytes(vector)

	pipe := sc.client.Pipeline()
	pipe.HSet(ctx, key, map[string]interface{}{
		"query_text": queryText,
		"vector":     vecBytes,
		"payload":    string(payloadBytes),
	})
	pipe.Expire(ctx, key, DefaultTTL)

	_, err = pipe.Exec(ctx)
	return err
}

func float32ToBytes(vec []float32) []byte {
	buf := make([]byte, len(vec)*4)
	for i, v := range vec {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
	}
	return buf
}
