package repoindex

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strings"
	"time"
)

const embeddingModel = "text-embedding-3-small" // 1536-dim, $0.02/MTok

// EmbedOptions controls embedding generation.
type EmbedOptions struct {
	RepoName string
	APIKey   string // OpenAI key; falls back to OPENAI_API_KEY env var
	Force    bool
}

// EmbedRepo generates and stores vector embeddings for all indexed entities
// in the given repo. Skips entities that already have an embedding unless Force.
func EmbedRepo(db *DB, opts EmbedOptions) error {
	repoID := slugID(opts.RepoName)

	apiKey := opts.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if apiKey == "" {
		return fmt.Errorf("OPENAI_API_KEY not set — needed for embeddings")
	}

	snap, err := GetSnapshot(db, opts.RepoName)
	if err != nil {
		return err
	}

	type entity struct {
		entityType string
		entityID   string
		text       string
	}

	var entities []entity

	for _, h := range snap.Handlers {
		entities = append(entities, entity{"handler", h.Name, handlerText(h)})
	}
	for _, m := range snap.Models {
		entities = append(entities, entity{"model", m.Name, modelText(m)})
	}
	for _, a := range snap.ExternalAPIs {
		entities = append(entities, entity{"api", a.Name, apiText(a)})
	}
	for _, e := range snap.Events {
		entities = append(entities, entity{"event", e.Name, eventText(e)})
	}

	skipped := 0
	for _, ent := range entities {
		id := slugID(repoID + "-emb-" + ent.entityType + "-" + ent.entityID)

		if !opts.Force {
			var exists int
			db.sql.QueryRow(`SELECT COUNT(1) FROM embeddings WHERE id=?`, id).Scan(&exists)
			if exists > 0 {
				skipped++
				continue
			}
		}

		vec, err := fetchEmbedding(apiKey, ent.text)
		if err != nil {
			fmt.Fprintf(os.Stderr, "embed %s/%s: %v\n", ent.entityType, ent.entityID, err)
			continue
		}

		blob := float32SliceToBlob(vec)
		db.sql.Exec(`
			INSERT INTO embeddings(id,entity_type,entity_id,repo_id,model,vector,created_at)
			VALUES(?,?,?,?,?,?,?)
			ON CONFLICT(id) DO UPDATE SET vector=excluded.vector, created_at=excluded.created_at`,
			id, ent.entityType, ent.entityID, repoID, embeddingModel, blob, time.Now().Format(time.RFC3339))

		fmt.Printf("  embedded %s/%s\n", ent.entityType, ent.entityID)
	}

	if skipped > 0 {
		fmt.Printf("skipped %d already-embedded entities (use --force to re-embed)\n", skipped)
	}
	return nil
}

// SimilarResult is a single result from a similarity search.
type SimilarResult struct {
	EntityType string  `json:"entity_type"`
	EntityID   string  `json:"entity_id"`
	RepoName   string  `json:"repo_name"`
	Score      float64 `json:"score"` // cosine similarity 0–1
}

// SimilarEntities finds the N most similar entities to the given entity across all indexed repos.
func SimilarEntities(db *DB, repoName, entityType, entityID string, topN int) ([]SimilarResult, error) {
	repoID := slugID(repoName)
	targetID := slugID(repoID + "-emb-" + entityType + "-" + entityID)

	var targetBlob []byte
	err := db.sql.QueryRow(`SELECT vector FROM embeddings WHERE id=?`, targetID).Scan(&targetBlob)
	if err != nil {
		return nil, fmt.Errorf("no embedding for %s/%s in repo %q — run: wtb repo embed %s", entityType, entityID, repoName, repoName)
	}
	target := blobToFloat32Slice(targetBlob)

	rows, err := db.sql.Query(`
		SELECT e.entity_type, e.entity_id, r.name, e.vector
		FROM embeddings e
		JOIN repos r ON r.id = e.repo_id
		WHERE NOT (e.entity_type=? AND e.entity_id=? AND e.repo_id=?)`,
		entityType, entityID, repoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SimilarResult
	for rows.Next() {
		var et, eid, rname string
		var blob []byte
		rows.Scan(&et, &eid, &rname, &blob)
		vec := blobToFloat32Slice(blob)
		score := cosineSimilarity(target, vec)
		results = append(results, SimilarResult{et, eid, rname, score})
	}

	// Sort descending by score.
	sortBySimilarity(results)
	if topN > 0 && len(results) > topN {
		results = results[:topN]
	}
	return results, nil
}

// --- OpenAI embedding API ---

func fetchEmbedding(apiKey, text string) ([]float32, error) {
	type request struct {
		Input string `json:"input"`
		Model string `json:"model"`
	}
	type response struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	body, _ := json.Marshal(request{Input: text, Model: embeddingModel})
	req, _ := http.NewRequest("POST", "https://api.openai.com/v1/embeddings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	var result response
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	if result.Error != nil {
		return nil, fmt.Errorf("openai: %s", result.Error.Message)
	}
	if len(result.Data) == 0 {
		return nil, fmt.Errorf("empty embedding response")
	}
	return result.Data[0].Embedding, nil
}

// --- entity text serialization ---

func handlerText(h Handler) string {
	parts := []string{
		fmt.Sprintf("handler name: %s", h.Name),
		fmt.Sprintf("trigger: %s", h.TriggerType),
		fmt.Sprintf("trigger detail: %s", h.TriggerDetail),
		fmt.Sprintf("timeout: %ds", h.Timeout),
	}
	if h.Description != "" {
		parts = append(parts, fmt.Sprintf("description: %s", h.Description))
	}
	return strings.Join(parts, ". ")
}

func modelText(m Model) string {
	var fields []string
	for _, f := range m.Fields {
		fields = append(fields, f.Name+":"+f.Type)
	}
	var assocs []string
	for _, a := range m.Associations {
		assocs = append(assocs, a.AssocType+"->"+a.TargetModel)
	}
	parts := []string{
		fmt.Sprintf("model name: %s", m.Name),
		fmt.Sprintf("table: %s", m.TableName),
		fmt.Sprintf("dialect: %s", m.Dialect),
	}
	if len(fields) > 0 {
		parts = append(parts, fmt.Sprintf("fields: %s", strings.Join(fields, ", ")))
	}
	if len(assocs) > 0 {
		parts = append(parts, fmt.Sprintf("associations: %s", strings.Join(assocs, ", ")))
	}
	return strings.Join(parts, ". ")
}

func apiText(a ExternalAPI) string {
	parts := []string{
		fmt.Sprintf("api name: %s", a.Name),
		fmt.Sprintf("url: %s", a.URL),
		fmt.Sprintf("method: %s", a.Method),
		fmt.Sprintf("auth: %s", a.AuthType),
	}
	if a.Description != "" {
		parts = append(parts, fmt.Sprintf("description: %s", a.Description))
	}
	return strings.Join(parts, ". ")
}

func eventText(e Event) string {
	return fmt.Sprintf("event name: %s. type: %s. detail-type: %s. bus: %s. description: %s",
		e.Name, e.EventType, e.DetailType, e.BusName, e.Description)
}

// --- vector math ---

func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		fa, fb := float64(a[i]), float64(b[i])
		dot += fa * fb
		normA += fa * fa
		normB += fb * fb
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

func float32SliceToBlob(v []float32) []byte {
	buf := make([]byte, len(v)*4)
	for i, f := range v {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	return buf
}

func blobToFloat32Slice(b []byte) []float32 {
	v := make([]float32, len(b)/4)
	for i := range v {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return v
}

func sortBySimilarity(results []SimilarResult) {
	// Simple insertion sort — dataset is small (~200 entities max).
	for i := 1; i < len(results); i++ {
		for j := i; j > 0 && results[j].Score > results[j-1].Score; j-- {
			results[j], results[j-1] = results[j-1], results[j]
		}
	}
}
