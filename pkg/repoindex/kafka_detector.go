package repoindex

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// reKafkaListener matches @KafkaListener(topics = ["topic1", "topic2"]) in Kotlin.
var reKafkaListener = regexp.MustCompile(`@KafkaListener\s*\([^)]*topics\s*=\s*\[([^\]]+)\]`)

// reTopicStringLiteral matches "kebab-case-topic" or "snake_case_topic" literals.
// Used to extract topic names from enum constructors and kafkaTemplate.send calls.
var reTopicStringLiteral = regexp.MustCompile(`"([a-z][a-z0-9_-]{2,})"`)

// reEnumTopicConstructor matches enum entries whose name contains TOPIC or EVENT
// and that have a string literal argument, e.g. FUEL_TRANSACTION_EVENTS_TOPIC("fuel-transaction-events").
var reEnumTopicConstructor = regexp.MustCompile(`(?i)\b([A-Z][A-Z0-9_]*(?:TOPIC|EVENT)[A-Z0-9_]*)\s*\(\s*"([a-z][a-z0-9_-]+)"`)

// reKafkaSend matches kafkaTemplate.send("topic-name", ...).
var reKafkaSend = regexp.MustCompile(`kafkaTemplate\.send\s*\(\s*"([a-z][a-z0-9_-]+)"`)

// reFlinkTopicConfig matches Flink TopicConfig sealed class objects:
//
//	object DevicePathTopic : TopicConfig<DevicePathPB>("device-path", ...)
//	object FilteredVideoEventTopic : SomeTopicConfig<T>("filtered-video-event", ...)
//
// These are kafka-consumer topics (Flink Kafka source).
var reFlinkTopicConfig = regexp.MustCompile(`:\s*\w*[Tt]opic\w*Config\s*<[^>]+>\s*\(\s*"([a-z][a-z0-9_-]+)"`)

// reFlinkKafkaSinkTopic matches KafkaSink/KafkaProducer literal topic strings in Flink jobs:
//
//	KafkaSink.builder<T>().setRecordSerializer(KafkaRecordSerializationSchema.builder().setTopic("webhook-events")...)
//	FlinkKafkaProducer("webhook-events", ...)
var reFlinkKafkaSinkTopic = regexp.MustCompile(`(?:setTopic|FlinkKafkaProducer)\s*\(\s*"([a-z][a-z0-9_-]+)"`)

// DetectKafkaTopics scans Kotlin source files in repoPath deterministically and
// returns a slice of Events with concrete Kafka topic names. This is called after
// LLM extraction to supplement or correct LLM-extracted events.
//
// Strategy:
//  1. @KafkaListener(topics = ["..."]) → kafka-consumer
//  2. Enum entry with TOPIC/EVENT in name and string constructor arg → kafka-producer
//  3. kafkaTemplate.send("topic", ...) → kafka-producer
func DetectKafkaTopics(repoPath string) []Event {
	seen := map[string]bool{}
	var events []Event

	filepath.Walk(repoPath, func(path string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return nil
		}
		// Skip generated and test directories
		if isTestPath(path) {
			return nil
		}
		base := fi.Name()
		dir := strings.ToLower(filepath.Base(filepath.Dir(path)))
		if dir == "target" || dir == ".gradle" || dir == "node_modules" {
			return filepath.SkipDir
		}
		if base == "target" || base == ".gradle" {
			return filepath.SkipDir
		}
		if filepath.Ext(path) != ".kt" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		content := string(data)

		// 1. @KafkaListener(topics = ["topic-name", ...])
		for _, match := range reKafkaListener.FindAllStringSubmatch(content, -1) {
			topicsStr := match[1] // e.g. "\"maintenance-events\", \"other-topic\""
			for _, lit := range reTopicStringLiteral.FindAllStringSubmatch(topicsStr, -1) {
				topic := lit[1]
				if isConcreteTopic(topic) && !seen["consumer:"+topic] {
					seen["consumer:"+topic] = true
					events = append(events, Event{
						Name:      topic,
						EventType: "kafka-consumer",
						BusName:   topic,
					})
				}
			}
		}

		// 2. Enum entries: FUEL_TRANSACTION_EVENTS_TOPIC("fuel-transaction-events")
		for _, match := range reEnumTopicConstructor.FindAllStringSubmatch(content, -1) {
			topic := match[2]
			if isConcreteTopic(topic) && !seen["producer:"+topic] {
				seen["producer:"+topic] = true
				events = append(events, Event{
					Name:      topic,
					EventType: "kafka-producer",
					BusName:   topic,
				})
			}
		}

		// 3. kafkaTemplate.send("topic-name", ...)
		for _, match := range reKafkaSend.FindAllStringSubmatch(content, -1) {
			topic := match[1]
			if isConcreteTopic(topic) && !seen["producer:"+topic] {
				seen["producer:"+topic] = true
				events = append(events, Event{
					Name:      topic,
					EventType: "kafka-producer",
					BusName:   topic,
				})
			}
		}

		// 4. Flink TopicConfig sealed class objects (kafka-consumer)
		//    object DevicePathTopic : TopicConfig<DevicePathPB>("device-path", ...)
		for _, match := range reFlinkTopicConfig.FindAllStringSubmatch(content, -1) {
			topic := match[1]
			if isConcreteTopic(topic) && !seen["consumer:"+topic] {
				seen["consumer:"+topic] = true
				events = append(events, Event{
					Name:      topic,
					EventType: "kafka-consumer",
					BusName:   topic,
				})
			}
		}

		// 5. Flink KafkaSink / FlinkKafkaProducer literal topics (kafka-producer)
		//    setTopic("webhook-events") or FlinkKafkaProducer("webhook-events", ...)
		for _, match := range reFlinkKafkaSinkTopic.FindAllStringSubmatch(content, -1) {
			topic := match[1]
			if isConcreteTopic(topic) && !seen["producer:"+topic] {
				seen["producer:"+topic] = true
				events = append(events, Event{
					Name:      topic,
					EventType: "kafka-producer",
					BusName:   topic,
				})
			}
		}

		return nil
	})

	return events
}

// mergeDetectedTopics adds deterministically detected Kafka events to a repo,
// skipping topics already present in the DB (by bus_name + event_type).
// Called from IndexRepo after all LLM layers complete.
func mergeDetectedTopics(db *DB, repoID string, detected []Event) {
	if len(detected) == 0 {
		return
	}

	// Load existing bus_names to avoid duplicates
	rows, err := db.sql.Query(
		`SELECT bus_name, event_type FROM events WHERE repo_id=?`, repoID)
	if err != nil {
		return
	}
	existing := map[string]bool{}
	for rows.Next() {
		var bn, et string
		rows.Scan(&bn, &et)
		existing[et+":"+bn] = true
	}
	rows.Close()

	for _, e := range detected {
		key := e.EventType + ":" + e.BusName
		if existing[key] {
			continue
		}
		id := slugID(repoID + "-event-" + e.EventType + "-" + e.BusName)
		db.sql.Exec(
			`INSERT INTO events(id,repo_id,name,event_type,detail_type,bus_name,description)
			 VALUES(?,?,?,?,?,?,?)
			 ON CONFLICT(id) DO UPDATE SET event_type=excluded.event_type, bus_name=excluded.bus_name`,
			id, repoID, e.Name, e.EventType, "", e.BusName, "")
	}
}
