// pkg/validation/examples/cobli.go
package examples

// CobliExample represents an example from Cobli
type CobliExample struct {
	Domain      string
	Description string
	Why         string
	Details     map[string]string
}

// Event Sourcing Examples
var EventSourcingExamples = []CobliExample{
	{
		Domain:      "Telemetria",
		Description: "VehiclePositionUpdated",
		Why:         "Rastrear 500K veículos em tempo real",
		Details: map[string]string{
			"Campos":    "vehicle_id, lat, lng, timestamp, speed, heading",
			"Volume":    "500K eventos/segundo",
			"Retenção":  "2 anos (compliance)",
			"Latência":  "< 50ms p95",
		},
	},
	{
		Domain:      "Manutenção",
		Description: "MaintenanceScheduled",
		Why:         "Histórico completo para audit",
		Details: map[string]string{
			"Campos":   "vehicle_id, type, scheduled_date, parts_needed",
			"Volume":   "~1K eventos/dia",
			"Retenção": "7 anos (compliance)",
			"Uso":      "Rastreabilidade total de manutenções",
		},
	},
	{
		Domain:      "Operação",
		Description: "RouteOptimized",
		Why:         "Otimização contínua de rotas",
		Details: map[string]string{
			"Campos":  "route_id, waypoints, distance, eta, fuel_estimate",
			"Volume":  "~50K eventos/dia",
			"Impacto": "Redução 15% consumo combustível",
		},
	},
}

// Stack Examples
var StackExamples = map[string]CobliExample{
	"backend": {
		Domain:      "Backend API",
		Description: "Go + Gin + Kafka + ScyllaDB",
		Why:         "Performance para streaming IoT",
		Details: map[string]string{
			"Linguagem":     "Go 1.21+",
			"Framework":     "Gin (HTTP router)",
			"Messaging":     "Kafka (streaming)",
			"Write DB":      "ScyllaDB (alta escrita)",
			"Read DB":       "PostgreSQL (queries complexas)",
			"Performance":   "< 200ms p95 API, < 50ms streaming",
			"Throughput":    "500K eventos/segundo",
			"Concorrência":  "Goroutines (nativo)",
			"Deploy":        "Binário único",
		},
	},
	"frontend": {
		Domain:      "Frontend Dashboard",
		Description: "TypeScript + React + Redux",
		Why:         "Dashboard complexo com real-time",
		Details: map[string]string{
			"Linguagem":    "TypeScript 5.0+",
			"Framework":    "React 18",
			"State":        "Redux Toolkit + RTK Query",
			"Real-time":    "WebSocket + EventSource",
			"UI":           "Material-UI customizado",
			"Performance":  "< 3s first load",
			"Update":       "< 100ms latência",
		},
	},
	"mobile": {
		Domain:      "Mobile Apps",
		Description: "React Native",
		Why:         "Time pequeno, código compartilhado",
		Details: map[string]string{
			"Platforms":     "iOS + Android",
			"Framework":     "React Native 0.72+",
			"State":         "Redux Toolkit",
			"Offline":       "Redux Persist + AsyncStorage",
			"Maps":          "Google Maps SDK",
			"Push":          "Firebase Cloud Messaging",
			"Compartilhado": "70% código comum",
			"Time":          "2 devs mobile",
		},
	},
	"iot": {
		Domain:      "IoT Firmware",
		Description: "C/C++ embarcado",
		Why:         "Performance e tamanho em hardware limitado",
		Details: map[string]string{
			"Linguagem":  "C/C++",
			"Hardware":   "ARM Cortex-M",
			"Protocolo":  "MQTT over 4G",
			"Bateria":    "Otimizado para duração",
			"Update":     "OTA (over-the-air)",
			"Devices":    "500K ativos",
		},
	},
}

// NFR Examples
var NFRExamples = []CobliExample{
	{
		Domain:      "Performance",
		Description: "Latência e throughput",
		Details: map[string]string{
			"API REST":        "< 200ms p95",
			"Streaming":       "< 50ms p95",
			"Dashboard load":  "< 3s",
			"Update time":     "< 100ms",
			"Throughput API":  "10K req/s",
			"Throughput data": "500K eventos/s",
		},
	},
	{
		Domain:      "Escala",
		Description: "Volumes de produção",
		Details: map[string]string{
			"Dispositivos ativos":   "500K",
			"Eventos/dia":           "10M processados",
			"Usuários simultâneos":  "100K",
			"Dados armazenados":     "50TB",
			"Crescimento":           "3x nos últimos 2 anos",
			"Projeção":              "1.5M devices em 2 anos",
		},
	},
	{
		Domain:      "Disponibilidade",
		Description: "SLA e redundância",
		Details: map[string]string{
			"SLA":              "99.9% uptime",
			"RTO":              "< 30 minutos",
			"RPO":              "< 5 minutos",
			"Deployment":       "Multi-AZ (3 zonas)",
			"Backup":           "Contínuo + snapshots diários",
			"Disaster Recovery": "Region failover < 1h",
		},
	},
}

// Pattern Examples
var PatternExamples = map[string]CobliExample{
	"event_sourcing": {
		Domain:      "Event Sourcing + CQRS",
		Description: "Separação write/read com eventos",
		Why:         "Rastreabilidade total e performance",
		Details: map[string]string{
			"Write side":  "Kafka + ScyllaDB (append-only)",
			"Read side":   "PostgreSQL (materialized views)",
			"Sync":        "Kafka Connect + Debezium",
			"Benefits":    "Audit completo, replay, temporal queries",
			"Tradeoff":    "Complexidade aumentada",
			"Quando usar": "Audit critical, high write volume",
		},
	},
	"api_gateway": {
		Domain:      "API Gateway Pattern",
		Description: "Entrada unificada para microservices",
		Why:         "Roteamento, auth, rate limiting",
		Details: map[string]string{
			"Implementação": "Envoy Proxy",
			"Features":      "Auth, rate limit, circuit breaker",
			"Performance":   "< 5ms overhead",
			"HA":            "Multi-instance com LB",
		},
	},
}

// GetEventSourcingExample returns an example based on domain
func GetEventSourcingExample(domain string) *CobliExample {
	for _, ex := range EventSourcingExamples {
		if ex.Domain == domain {
			return &ex
		}
	}
	return &EventSourcingExamples[0] // Default to first
}

// GetStackExample returns stack example
func GetStackExample(stack string) *CobliExample {
	if ex, ok := StackExamples[stack]; ok {
		return &ex
	}
	return nil
}

// GetNFRExample returns NFR example
func GetNFRExample(nfrType string) *CobliExample {
	for _, ex := range NFRExamples {
		if ex.Domain == nfrType {
			return &ex
		}
	}
	return &NFRExamples[0] // Default
}

// FormatExample formats an example for display
func FormatExample(ex *CobliExample) string {
	var result string
	
	result += "🏢 Exemplo Cobli: " + ex.Domain + "\n"
	result += "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n"
	result += ex.Description + "\n\n"
	result += "Por quê? " + ex.Why + "\n\n"
	
	if len(ex.Details) > 0 {
		result += "Detalhes:\n"
		for key, value := range ex.Details {
			result += "  • " + key + ": " + value + "\n"
		}
	}
	
	return result
}
