package repoindex

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// GetSnapshot returns the full indexed state of a repo.
func GetSnapshot(db *DB, repoName string) (*RepoSnapshot, error) {
	repoID := slugID(repoName)

	snap := &RepoSnapshot{}

	// Repo — COALESCE handles NULL for columns added via ALTER TABLE
	row := db.sql.QueryRow(`SELECT id,name,path,lang,framework,COALESCE(owner,''),COALESCE(last_indexed_at,''),COALESCE(secondary_lang,''),COALESCE(secondary_framework,''),COALESCE(ci_platform,'') FROM repos WHERE id=?`, repoID)
	if err := row.Scan(&snap.Repo.ID, &snap.Repo.Name, &snap.Repo.Path, &snap.Repo.Lang, &snap.Repo.Framework, &snap.Repo.Owner, &snap.Repo.LastIndexedAt, &snap.Repo.SecondaryLang, &snap.Repo.SecondaryFramework, &snap.Repo.CIPlatform); err != nil {
		return nil, fmt.Errorf("repo %q not found (run: wtb repo index %s)", repoName, repoName)
	}

	// Handlers
	rows, _ := db.sql.Query(`SELECT id,repo_id,name,handler_file,trigger_type,trigger_detail,timeout,max_retry,concurrency,vpc,description FROM handlers WHERE repo_id=? ORDER BY name`, repoID)
	defer rows.Close()
	for rows.Next() {
		var h Handler
		var vpc int
		rows.Scan(&h.ID, &h.RepoID, &h.Name, &h.HandlerFile, &h.TriggerType, &h.TriggerDetail, &h.Timeout, &h.MaxRetry, &h.Concurrency, &vpc, &h.Description)
		h.VPC = vpc == 1
		snap.Handlers = append(snap.Handlers, h)
	}

	// Events
	rows2, _ := db.sql.Query(`SELECT id,repo_id,name,event_type,detail_type,bus_name,description FROM events WHERE repo_id=? ORDER BY name`, repoID)
	defer rows2.Close()
	for rows2.Next() {
		var e Event
		rows2.Scan(&e.ID, &e.RepoID, &e.Name, &e.EventType, &e.DetailType, &e.BusName, &e.Description)
		snap.Events = append(snap.Events, e)
	}

	// Models (with fields + associations)
	rows3, _ := db.sql.Query(`SELECT id,repo_id,name,table_name,dialect FROM models WHERE repo_id=? ORDER BY name`, repoID)
	defer rows3.Close()
	for rows3.Next() {
		var m Model
		rows3.Scan(&m.ID, &m.RepoID, &m.Name, &m.TableName, &m.Dialect)
		m.Fields = getModelFields(db.sql, m.ID)
		m.Associations = getModelAssociations(db.sql, m.ID)
		snap.Models = append(snap.Models, m)
	}

	// External APIs
	rows4, _ := db.sql.Query(`SELECT id,repo_id,name,url,method,auth_type,description FROM external_apis WHERE repo_id=? ORDER BY name`, repoID)
	defer rows4.Close()
	for rows4.Next() {
		var a ExternalAPI
		rows4.Scan(&a.ID, &a.RepoID, &a.Name, &a.URL, &a.Method, &a.AuthType, &a.Description)
		snap.ExternalAPIs = append(snap.ExternalAPIs, a)
	}

	// DB Connections
	rows5, _ := db.sql.Query(`SELECT id,repo_id,dialect,host_var,pool_min,pool_max,pool_idle FROM db_connections WHERE repo_id=?`, repoID)
	defer rows5.Close()
	for rows5.Next() {
		var d DBConnection
		rows5.Scan(&d.ID, &d.RepoID, &d.Dialect, &d.HostVar, &d.PoolMin, &d.PoolMax, &d.PoolIdle)
		snap.DBConnections = append(snap.DBConnections, d)
	}

	// Config Vars
	rows6, _ := db.sql.Query(`SELECT id,repo_id,key,source,description FROM config_vars WHERE repo_id=? ORDER BY key`, repoID)
	defer rows6.Close()
	for rows6.Next() {
		var c ConfigVar
		rows6.Scan(&c.ID, &c.RepoID, &c.Key, &c.Source, &c.Description)
		snap.ConfigVars = append(snap.ConfigVars, c)
	}

	// Notes
	rows7, _ := db.sql.Query(`SELECT id,entity_type,entity_id,repo_id,content,created_at FROM notes WHERE repo_id=? ORDER BY created_at DESC`, repoID)
	defer rows7.Close()
	for rows7.Next() {
		var n Note
		rows7.Scan(&n.ID, &n.EntityType, &n.EntityID, &n.RepoID, &n.Content, &n.CreatedAt)
		snap.Notes = append(snap.Notes, n)
	}

	// Deployment units (from import-arch)
	rows8, _ := db.sql.Query(`SELECT id,repo_id,name,description,namespace,replicas_min,replicas_max,consumer_group,deprecated FROM deployment_units WHERE repo_id=? ORDER BY name`, repoID)
	defer rows8.Close()
	for rows8.Next() {
		var u DeploymentUnit
		var dep int
		rows8.Scan(&u.ID, &u.RepoID, &u.Name, &u.Description, &u.Namespace, &u.ReplicasMin, &u.ReplicasMax, &u.ConsumerGroup, &dep)
		u.Deprecated = dep == 1
		snap.DeploymentUnits = append(snap.DeploymentUnits, u)
	}

	// Topic enrichments (from import-arch)
	rows9, _ := db.sql.Query(`SELECT id,repo_id,deployment_unit,topic,direction,serialization,consumer_group,key_description FROM topic_enrichments WHERE repo_id=? ORDER BY direction,topic`, repoID)
	defer rows9.Close()
	for rows9.Next() {
		var t TopicEnrichment
		rows9.Scan(&t.ID, &t.RepoID, &t.DeploymentUnit, &t.Topic, &t.Direction, &t.Serialization, &t.ConsumerGroup, &t.KeyDescription)
		snap.TopicEnrichments = append(snap.TopicEnrichments, t)
	}

	// DD monitors (from dd-enrich / set-dd-monitor)
	rows10, _ := db.sql.Query(`SELECT id,repo_id,monitor_id,name,type,status,url,fetched_at FROM dd_monitors WHERE repo_id=? ORDER BY name`, repoID)
	defer rows10.Close()
	for rows10.Next() {
		var m DDMonitor
		rows10.Scan(&m.ID, &m.RepoID, &m.MonitorID, &m.Name, &m.Type, &m.Status, &m.URL, &m.FetchedAt)
		snap.DDMonitors = append(snap.DDMonitors, m)
	}

	// Service metrics (from dd-metrics)
	rows11, _ := db.sql.Query(`SELECT id,repo_id,metric_name,category,fetched_at FROM service_metrics WHERE repo_id=? ORDER BY category,metric_name`, repoID)
	defer rows11.Close()
	for rows11.Next() {
		var m ServiceMetric
		rows11.Scan(&m.ID, &m.RepoID, &m.MetricName, &m.Category, &m.FetchedAt)
		snap.ServiceMetrics = append(snap.ServiceMetrics, m)
	}

	// Chart snapshots with resources and env vars
	snapRows, _ := db.sql.Query(`SELECT id,repo_id,env,image_tag,app_version,captured_at,COALESCE(kube_context,''),COALESCE(namespace,'') FROM chart_snapshots WHERE repo_id=? ORDER BY app_version`, repoID)
	defer snapRows.Close()
	for snapRows.Next() {
		var cs ChartSnapshot
		snapRows.Scan(&cs.ID, &cs.RepoID, &cs.Env, &cs.ImageTag, &cs.AppVersion, &cs.CapturedAt, &cs.KubeContext, &cs.Namespace)
		// Resources
		rRows, _ := db.sql.Query(`SELECT id,snapshot_id,repo_id,container,cpu_request,cpu_limit,mem_request,mem_limit,heap_size,replicas_min,replicas_max FROM chart_resources WHERE snapshot_id=? ORDER BY container`, cs.ID)
		for rRows.Next() {
			var r ChartResources
			rRows.Scan(&r.ID, &r.SnapshotID, &r.RepoID, &r.Container, &r.CPURequest, &r.CPULimit, &r.MemRequest, &r.MemLimit, &r.HeapSize, &r.ReplicasMin, &r.ReplicasMax)
			cs.Resources = append(cs.Resources, r)
		}
		rRows.Close()
		// Env vars
		eRows, _ := db.sql.Query(`SELECT id,snapshot_id,repo_id,key,value FROM chart_env_vars WHERE snapshot_id=? ORDER BY key`, cs.ID)
		for eRows.Next() {
			var e ChartEnvVar
			eRows.Scan(&e.ID, &e.SnapshotID, &e.RepoID, &e.Key, &e.Value)
			cs.EnvVars = append(cs.EnvVars, e)
		}
		eRows.Close()
		snap.ChartSnapshots = append(snap.ChartSnapshots, cs)
	}

	return snap, nil
}

func getModelFields(sqldb *sql.DB, modelID string) []ModelField {
	rows, _ := sqldb.Query(`SELECT id,model_id,name,type,nullable,primary_key,unique_field FROM model_fields WHERE model_id=? ORDER BY name`, modelID)
	defer rows.Close()
	var fields []ModelField
	for rows.Next() {
		var f ModelField
		var nullable, pk, uniq int
		rows.Scan(&f.ID, &f.ModelID, &f.Name, &f.Type, &nullable, &pk, &uniq)
		f.Nullable, f.PrimaryKey, f.Unique = nullable == 1, pk == 1, uniq == 1
		fields = append(fields, f)
	}
	return fields
}

func getModelAssociations(sqldb *sql.DB, modelID string) []ModelAssociation {
	rows, _ := sqldb.Query(`SELECT id,model_id,assoc_type,target_model,foreign_key FROM model_associations WHERE model_id=?`, modelID)
	defer rows.Close()
	var assocs []ModelAssociation
	for rows.Next() {
		var a ModelAssociation
		rows.Scan(&a.ID, &a.ModelID, &a.AssocType, &a.TargetModel, &a.ForeignKey)
		assocs = append(assocs, a)
	}
	return assocs
}

// ListRepos returns all indexed repos.
func ListRepos(db *DB) ([]Repo, error) {
	rows, err := db.sql.Query(`SELECT id,name,path,lang,framework,COALESCE(owner,''),COALESCE(last_indexed_at,''),COALESCE(secondary_lang,''),COALESCE(secondary_framework,''),COALESCE(ci_platform,'') FROM repos ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var repos []Repo
	for rows.Next() {
		var r Repo
		rows.Scan(&r.ID, &r.Name, &r.Path, &r.Lang, &r.Framework, &r.Owner, &r.LastIndexedAt, &r.SecondaryLang, &r.SecondaryFramework, &r.CIPlatform)
		repos = append(repos, r)
	}
	return repos, nil
}

// QueryRows executes a raw SQL query and returns column names + rows as string slices.
func QueryRows(db *DB, query string, args ...any) ([]string, [][]string, error) {
	rows, err := db.sql.Query(query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, nil, err
	}

	var data [][]string
	for rows.Next() {
		vals := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, nil, err
		}
		row := make([]string, len(cols))
		for i, v := range vals {
			if v == nil {
				row[i] = "NULL"
			} else {
				row[i] = fmt.Sprintf("%v", v)
			}
		}
		data = append(data, row)
	}
	return cols, data, rows.Err()
}

// SetOwner updates the owner field of an already-indexed repo without re-indexing.
func SetOwner(db *DB, repoName, owner string) error {
	repoID := slugID(repoName)
	res, err := db.sql.Exec(`UPDATE repos SET owner=? WHERE id=?`, owner, repoID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("repo %q not found (run: wtb repo index %s)", repoName, repoName)
	}
	return nil
}

// GetDeploymentUnits returns deployment units for a repo.
func GetDeploymentUnits(db *DB, repoName string) ([]DeploymentUnit, error) {
	repoID := slugID(repoName)
	rows, err := db.sql.Query(`SELECT id,repo_id,name,description,namespace,replicas_min,replicas_max,consumer_group,deprecated FROM deployment_units WHERE repo_id=? ORDER BY name`, repoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var units []DeploymentUnit
	for rows.Next() {
		var u DeploymentUnit
		var dep int
		rows.Scan(&u.ID, &u.RepoID, &u.Name, &u.Description, &u.Namespace, &u.ReplicasMin, &u.ReplicasMax, &u.ConsumerGroup, &dep)
		u.Deprecated = dep == 1
		units = append(units, u)
	}
	return units, nil
}

// GetTopicEnrichments returns architecture-derived topic metadata for a repo.
func GetTopicEnrichments(db *DB, repoName string) ([]TopicEnrichment, error) {
	repoID := slugID(repoName)
	rows, err := db.sql.Query(`SELECT id,repo_id,deployment_unit,topic,direction,serialization,consumer_group,key_description FROM topic_enrichments WHERE repo_id=? ORDER BY direction,topic`, repoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var topics []TopicEnrichment
	for rows.Next() {
		var t TopicEnrichment
		rows.Scan(&t.ID, &t.RepoID, &t.DeploymentUnit, &t.Topic, &t.Direction, &t.Serialization, &t.ConsumerGroup, &t.KeyDescription)
		topics = append(topics, t)
	}
	return topics, nil
}

// AddNote attaches a note to an entity.
func AddNote(db *DB, entityType, entityID, repoName, content string) error {
	repoID := slugID(repoName)
	id := slugID(fmt.Sprintf("%s-%s-%s-%d", repoID, entityType, entityID, time.Now().UnixNano()))
	_, err := db.sql.Exec(`INSERT INTO notes(id,entity_type,entity_id,repo_id,content,created_at) VALUES(?,?,?,?,?,?)`,
		id, entityType, entityID, repoID, content, time.Now().Format(time.RFC3339))
	return err
}

// RenderTable renders cols+rows as a plain text table for human reading.
func RenderTable(cols []string, rows [][]string) string {
	if len(cols) == 0 {
		return "(no results)\n"
	}
	widths := make([]int, len(cols))
	for i, c := range cols {
		widths[i] = len(c)
	}
	for _, row := range rows {
		for i, v := range row {
			if len(v) > widths[i] {
				widths[i] = len(v)
			}
		}
	}

	var sb strings.Builder
	sep := "+"
	for _, w := range widths {
		sep += strings.Repeat("-", w+2) + "+"
	}
	sb.WriteString(sep + "\n")

	header := "|"
	for i, c := range cols {
		header += fmt.Sprintf(" %-*s |", widths[i], c)
	}
	sb.WriteString(header + "\n" + sep + "\n")

	for _, row := range rows {
		line := "|"
		for i, v := range row {
			line += fmt.Sprintf(" %-*s |", widths[i], v)
		}
		sb.WriteString(line + "\n")
	}
	sb.WriteString(sep + "\n")
	return sb.String()
}

// snapshotToTable converts a snapshot field to cols+rows for --table rendering.
func SnapshotSection(snap *RepoSnapshot, section string) ([]string, [][]string) {
	switch section {
	case "handlers":
		cols := []string{"name", "trigger_type", "trigger_detail", "timeout", "concurrency", "vpc"}
		var rows [][]string
		for _, h := range snap.Handlers {
			rows = append(rows, []string{h.Name, h.TriggerType, h.TriggerDetail, fmt.Sprint(h.Timeout), fmt.Sprint(h.Concurrency), fmt.Sprint(h.VPC)})
		}
		return cols, rows
	case "models":
		cols := []string{"name", "table_name", "dialect", "fields", "associations"}
		var rows [][]string
		for _, m := range snap.Models {
			var fields []string
			for _, f := range m.Fields {
				fields = append(fields, f.Name)
			}
			var assocs []string
			for _, a := range m.Associations {
				assocs = append(assocs, a.AssocType+":"+a.TargetModel)
			}
			rows = append(rows, []string{m.Name, m.TableName, m.Dialect, strings.Join(fields, ","), strings.Join(assocs, ",")})
		}
		return cols, rows
	case "apis":
		cols := []string{"name", "url", "method", "auth_type"}
		var rows [][]string
		for _, a := range snap.ExternalAPIs {
			rows = append(rows, []string{a.Name, a.URL, a.Method, a.AuthType})
		}
		return cols, rows
	case "events":
		cols := []string{"name", "event_type", "detail_type", "bus_name"}
		var rows [][]string
		for _, e := range snap.Events {
			rows = append(rows, []string{e.Name, e.EventType, e.DetailType, e.BusName})
		}
		return cols, rows
	case "config":
		cols := []string{"key", "source", "description"}
		var rows [][]string
		for _, c := range snap.ConfigVars {
			rows = append(rows, []string{c.Key, c.Source, c.Description})
		}
		return cols, rows
	}
	return nil, nil
}
