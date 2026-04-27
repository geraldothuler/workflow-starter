package repoindex

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// ParseArchSummary reads ~/Cobliteam/architecture/docs/*/repoName/summary.md
// and extracts deployment units, Kafka topics with enrichment, DD service name, and primary hostname.
func ParseArchSummary(repoName, cobliteamDir string) (*ArchSummary, error) {
	archBase := filepath.Join(cobliteamDir, "architecture", "docs")
	categories := []string{"apis", "jobs", "gateways", "frontend", "apps"}

	var summaryPath string
	for _, cat := range categories {
		p := filepath.Join(archBase, cat, repoName, "summary.md")
		if _, err := os.Stat(p); err == nil {
			summaryPath = p
			break
		}
	}
	if summaryPath == "" {
		return nil, fmt.Errorf("no architecture summary found for %q", repoName)
	}

	data, err := os.ReadFile(summaryPath)
	if err != nil {
		return nil, err
	}

	return parseArchContent(repoName, string(data)), nil
}

// ImportArchSummary persists an ArchSummary into the DB, replacing previous data for the repo.
//
// DuckDB's ART (primary-key) index retains deleted keys within a transaction, causing a spurious
// duplicate-key error when DELETE + INSERT of the same id happen in the same tx.
// Workaround: commit the DELETE first, then INSERT in a separate transaction.
func ImportArchSummary(db *DB, summary *ArchSummary) error {
	repoID := slugID(summary.RepoName)

	// ── Tx 1: update scalar fields + delete enrichment rows ──────────────────
	tx1, err := db.sql.Begin()
	if err != nil {
		return fmt.Errorf("begin tx1: %w", err)
	}
	if _, err := tx1.Exec(`UPDATE repos SET dd_service_name=?, primary_hostname=? WHERE id=?`,
		summary.DDServiceName, summary.PrimaryHostname, repoID); err != nil {
		tx1.Rollback() //nolint:errcheck
		return fmt.Errorf("update repos: %w", err)
	}
	if _, err := tx1.Exec(`DELETE FROM deployment_units WHERE repo_id=?`, repoID); err != nil {
		tx1.Rollback() //nolint:errcheck
		return fmt.Errorf("delete deployment_units: %w", err)
	}
	if _, err := tx1.Exec(`DELETE FROM topic_enrichments WHERE repo_id=?`, repoID); err != nil {
		tx1.Rollback() //nolint:errcheck
		return fmt.Errorf("delete topic_enrichments: %w", err)
	}
	if err := tx1.Commit(); err != nil {
		return fmt.Errorf("commit tx1: %w", err)
	}

	// ── Tx 2: upsert enrichment rows ─────────────────────────────────────────
	// Use INSERT … ON CONFLICT DO UPDATE SET to avoid ART-index phantom issues
	// when the same key was deleted in Tx1 and reinserted here.
	tx2, err := db.sql.Begin()
	if err != nil {
		return fmt.Errorf("begin tx2: %w", err)
	}
	defer tx2.Rollback() //nolint:errcheck

	for _, u := range summary.Units {
		dep := 0
		if u.Deprecated {
			dep = 1
		}
		id := slugID(fmt.Sprintf("%s-%s", repoID, u.Name))
		if _, err := tx2.Exec(`
			INSERT INTO deployment_units(id,repo_id,name,description,namespace,replicas_min,replicas_max,consumer_group,deprecated)
			VALUES(?,?,?,?,?,?,?,?,?)
			ON CONFLICT (id) DO UPDATE SET
				name=excluded.name,
				description=excluded.description,
				namespace=excluded.namespace,
				replicas_min=excluded.replicas_min,
				replicas_max=excluded.replicas_max,
				consumer_group=excluded.consumer_group,
				deprecated=excluded.deprecated`,
			id, repoID, u.Name, u.Description, u.Namespace, u.ReplicasMin, u.ReplicasMax, u.ConsumerGroup, dep); err != nil {
			return fmt.Errorf("upsert deployment_unit %q: %w", u.Name, err)
		}
	}

	for i, t := range summary.Topics {
		id := slugID(fmt.Sprintf("%s-%s-%s-%d-%d", repoID, t.DeploymentUnit, t.Topic, i, time.Now().UnixNano()))
		if _, err := tx2.Exec(`
			INSERT INTO topic_enrichments(id,repo_id,deployment_unit,topic,direction,serialization,consumer_group,key_description)
			VALUES(?,?,?,?,?,?,?,?)
			ON CONFLICT (id) DO UPDATE SET
				deployment_unit=excluded.deployment_unit,
				topic=excluded.topic,
				direction=excluded.direction,
				serialization=excluded.serialization,
				consumer_group=excluded.consumer_group,
				key_description=excluded.key_description`,
			id, repoID, t.DeploymentUnit, t.Topic, t.Direction, t.Serialization, t.ConsumerGroup, t.KeyDescription); err != nil {
			return fmt.Errorf("upsert topic_enrichment %q: %w", t.Topic, err)
		}
	}

	return tx2.Commit()
}

// parseArchContent parses the markdown content of a summary.md file.
//
// Supports five summary formats:
//
//	Format A — H2 named unit (fusca, trigger-action-api):
//	  ## Deployment: fusca-api
//
//	Format B — H2 section + H3 units (atlas-api, total-costs-of-ownership, odometer):
//	  ## Deployment Units
//	  ### atlas-api          (or ### 1. odometer-api with numeric prefix)
//
//	Format C — list item (alexstrasza-*):
//	  - Deployment name: device-stops
//
//	Format D — list item with bold markers (sherlock-driver):
//	  - **Deployment name**: device-stops
//
//	Format E — bare H2 kebab-case (osm-enhanced-api, pringles, trigger-action-sql):
//	  ## osm-enhanced-public-api     (all-lowercase, at least one hyphen — not a section heading)
func parseArchContent(repoName, content string) *ArchSummary {
	summary := &ArchSummary{RepoName: repoName}
	lines := strings.Split(content, "\n")

	var currentUnit *DeploymentUnit
	inReadsFrom := false
	inWritesTo := false
	inDeploymentUnitsSection := false // true when inside a "## Deployment Units" block

	// Format A: ## Deployment: <name>  (colon required — prevents false match on "## Deployment Units")
	// Format C: - Deployment name: <name>  or  - **Deployment name**: <name>  (with/without bold markers)
	reDeployment := regexp.MustCompile(`(?i)(?:^##\s+Deployment:\s+|^-\s+\*{0,2}Deployment\s+name\*{0,2}:\s*)(.+)`)
	// Format B section header: ## Deployment Units  (no colon)
	reDeploymentUnitsSection := regexp.MustCompile(`(?i)^##\s+Deployment\s+Units\s*$`)
	// Format B unit: ### <name>  or  ### 1. <name>  (strip numeric prefix)
	reH3Unit := regexp.MustCompile(`^###\s+(?:\d+\.\s+)?(.+)`)
	// Format E: ## <kebab-case-name>  — bare H2 that IS the unit name (all-lowercase, at least one hyphen)
	// Matches "## osm-enhanced-public-api" but NOT "## Metadata" or "## Kafka Configuration"
	reH2KebabUnit := regexp.MustCompile(`^##\s+([a-z][a-z0-9]*(?:-[a-z0-9]+)+)\s*$`)
	reKamonService := regexp.MustCompile(`(?i)service name[:\s]+` + "`" + `([^` + "`" + `]+)` + "`")
	reReplicas := regexp.MustCompile(`(?i)\*\*Replicas[:\s*]+(\d+)(?:-(\d+))?`)
	reNamespace := regexp.MustCompile(`(?i)\*\*Namespace[:\s*]+([^\s*]+)`)
	reConsumerGroup := regexp.MustCompile(`(?i)\*\*Consumer Group[:\s*]+([^\s*]+)`)
	reKafkaLine := regexp.MustCompile(`(?i)kafka topic[:\s]+` + "`" + `?([a-zA-Z0-9_\-]+)` + "`" + `?`)
	reKafkaParens := regexp.MustCompile(`(?i)\(([^)]+)\)`)
	reHostname := regexp.MustCompile(`([a-zA-Z0-9\-]+\.[a-zA-Z0-9\-]+\.cobli\.co)`)
	reDeprecatedClean := regexp.MustCompile(`(?i)\s*\[?DEPRECATED\]?`)
	reParenSuffix := regexp.MustCompile(`\s*\([^)]*\)\s*$`) // strip trailing "(CronJob)", "(Job)", etc.

	flushUnit := func() {
		if currentUnit != nil {
			summary.Units = append(summary.Units, *currentUnit)
			currentUnit = nil
		}
	}

	newUnit := func(name string, deprecated bool) {
		flushUnit()
		name = reDeprecatedClean.ReplaceAllString(name, "")
		name = reParenSuffix.ReplaceAllString(name, "")
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		currentUnit = &DeploymentUnit{Name: name, Deprecated: deprecated}
		inReadsFrom = false
		inWritesTo = false
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Format B: section header "## Deployment Units"
		if reDeploymentUnitsSection.MatchString(trimmed) {
			inDeploymentUnitsSection = true
			flushUnit()
			continue
		}

		// Any other H2 heading exits the deployment-units section
		if strings.HasPrefix(trimmed, "## ") && !reDeploymentUnitsSection.MatchString(trimmed) {
			if inDeploymentUnitsSection {
				inDeploymentUnitsSection = false
			}
		}

		// Format A: ## Deployment: <name>  /  Format C: - Deployment name: <name>
		if m := reDeployment.FindStringSubmatch(trimmed); m != nil {
			deprecated := strings.Contains(strings.ToUpper(m[1]), "DEPRECATED") ||
				strings.Contains(strings.ToUpper(trimmed), "[DEPRECATED]")
			newUnit(m[1], deprecated)
			continue
		}

		// Format E: ## <kebab-case>  — bare H2 that IS the unit name (only when not matched above)
		if m := reH2KebabUnit.FindStringSubmatch(trimmed); m != nil {
			deprecated := strings.Contains(strings.ToUpper(trimmed), "DEPRECATED")
			newUnit(m[1], deprecated)
			continue
		}

		// Format B: ### <name>  (only when inside Deployment Units section)
		if inDeploymentUnitsSection {
			if m := reH3Unit.FindStringSubmatch(trimmed); m != nil {
				name := strings.TrimSpace(m[1])
				deprecated := strings.Contains(strings.ToUpper(name), "DEPRECATED") ||
					strings.Contains(strings.ToUpper(trimmed), "[DEPRECATED]")
				newUnit(name, deprecated)
				continue
			}
		}

		// Detect section boundaries
		if strings.Contains(trimmed, "<reads_from>") {
			inReadsFrom = true
			inWritesTo = false
			continue
		}
		if strings.Contains(trimmed, "</reads_from>") {
			inReadsFrom = false
			continue
		}
		if strings.Contains(trimmed, "<writes_to>") {
			inWritesTo = true
			inReadsFrom = false
			continue
		}
		if strings.Contains(trimmed, "</writes_to>") {
			inWritesTo = false
			continue
		}

		// Parse unit metadata
		if currentUnit != nil {
			if m := reReplicas.FindStringSubmatch(trimmed); m != nil {
				fmt.Sscanf(m[1], "%d", &currentUnit.ReplicasMin)
				if m[2] != "" {
					fmt.Sscanf(m[2], "%d", &currentUnit.ReplicasMax)
				} else {
					currentUnit.ReplicasMax = currentUnit.ReplicasMin
				}
			}
			if m := reNamespace.FindStringSubmatch(trimmed); m != nil {
				currentUnit.Namespace = strings.Trim(m[1], "*_ ")
			}
			if m := reConsumerGroup.FindStringSubmatch(trimmed); m != nil {
				currentUnit.ConsumerGroup = strings.Trim(m[1], "*_ ")
			}
			if strings.HasPrefix(trimmed, "- **Description:**") || strings.HasPrefix(trimmed, "- **Description") {
				desc := strings.TrimPrefix(trimmed, "- **Description:**")
				desc = regexp.MustCompile(`\*\*Description[^:]*:\*\*\s*`).ReplaceAllString(desc, "")
				currentUnit.Description = strings.TrimSpace(desc)
			}
			// Also check for [DEPRECATED] in description/standalone line
			if !currentUnit.Deprecated && (strings.Contains(strings.ToUpper(trimmed), "[DEPRECATED]") || strings.Contains(strings.ToUpper(trimmed), "DEPRECATED]")) {
				currentUnit.Deprecated = true
			}
		}

		// Extract hostname (first unique one becomes primary)
		if hm := reHostname.FindString(trimmed); hm != "" && summary.PrimaryHostname == "" {
			summary.PrimaryHostname = hm
		}

		// Parse Kafka lines in reads_from / writes_to
		if (inReadsFrom || inWritesTo) && currentUnit != nil {
			direction := "consumes"
			if inWritesTo {
				direction = "produces"
			}

			// Check for Kafka topic lines
			if km := reKafkaLine.FindStringSubmatch(trimmed); km != nil {
				topicName := km[1]
				te := TopicEnrichment{
					RepoID:         slugID(repoName),
					DeploymentUnit: currentUnit.Name,
					Topic:          topicName,
					Direction:      direction,
				}

				// Extract serialization and consumer group from parentheses
				if pm := reKafkaParens.FindAllStringSubmatch(trimmed, -1); pm != nil {
					for _, p := range pm {
						info := strings.ToLower(p[1])
						if strings.Contains(info, "protobuf") || strings.Contains(info, "proto") {
							te.Serialization = "protobuf"
						} else if strings.Contains(info, "json") {
							te.Serialization = "json"
						} else if strings.Contains(info, "avro") {
							te.Serialization = "avro"
						}
						if strings.Contains(info, "consumer group:") {
							parts := strings.SplitN(info, "consumer group:", 2)
							if len(parts) == 2 {
								te.ConsumerGroup = strings.TrimSpace(parts[1])
							}
						}
						if strings.Contains(info, "key:") {
							parts := strings.SplitN(p[1], "key:", 2)
							if len(parts) == 2 {
								te.KeyDescription = strings.TrimSpace(parts[1])
							}
						}
					}
				}
				// Fallback: consumer group from the reads_from line itself
				if te.ConsumerGroup == "" && inReadsFrom && currentUnit.ConsumerGroup != "" {
					te.ConsumerGroup = currentUnit.ConsumerGroup
				}

				summary.Topics = append(summary.Topics, te)
			}

			// DD service name from writes_to Datadog line
			if inWritesTo {
				if m := reKamonService.FindStringSubmatch(trimmed); m != nil && summary.DDServiceName == "" {
					summary.DDServiceName = strings.TrimSpace(m[1])
				}
			}
		}

		// Also catch DD service name outside of writes_to (some files put it elsewhere)
		if summary.DDServiceName == "" {
			if m := reKamonService.FindStringSubmatch(trimmed); m != nil {
				summary.DDServiceName = strings.TrimSpace(m[1])
			}
		}
	}
	flushUnit()

	return summary
}
