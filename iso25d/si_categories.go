package iso25d

// siCategories maps SI slug → short category tag for agent-facing descriptions.
var siCategories = map[string]string{
	// Databases — relational
	"clickhouse":   "relational database / OLAP",
	"cockroachlabs": "relational database",
	"mariadb":      "relational database",
	"mysql":        "relational database",
	"oracle":       "relational database",
	"planetscale":  "relational database",
	"postgresql":   "relational database",
	"sqlite":       "relational database",
	"supabase":     "relational database / BaaS",

	// Databases — NoSQL / document
	"apachecassandra": "NoSQL database",
	"apachecouchdb":   "NoSQL database",
	"couchbase":       "NoSQL database",
	"firebase":        "NoSQL database / BaaS",
	"mongodb":         "NoSQL database",
	"neo4j":           "graph database",

	// Databases — in-memory / cache / key-value
	"redis": "cache / key-value store",

	// Databases — search
	"elastic":       "search engine",
	"elasticsearch": "search engine",
	"opensearch":    "search engine",

	// Databases — time-series
	"influxdb":       "time-series database",
	"victoriametrics": "time-series database / observability",

	// Databases — analytical / warehouse / lake
	"databricks": "data lakehouse platform",
	"dbt":        "data transformation / analytics engineering",
	"duckdb":     "analytical database / OLAP",
	"snowflake":  "data warehouse",
	"trino":      "distributed SQL query engine",

	// Message queue / streaming
	"apachekafka":  "message queue / streaming",
	"apachepulsar": "message queue / streaming",
	"rabbitmq":     "message queue",

	// ETL / data integration / orchestration
	"airbyte":      "ETL / data integration",
	"apacheairflow": "workflow orchestration",
	"apacheflink":  "stream processing / ETL",
	"apachehadoop": "distributed data processing",
	"apachehive":   "data warehouse / big data SQL",
	"apachespark":  "distributed data processing",
	"temporal":     "workflow orchestration",

	// Container runtime / orchestration
	"docker":     "container runtime",
	"kubernetes": "container orchestration",
	"podman":     "container runtime",

	// Infrastructure as code / provisioning
	"ansible":    "infrastructure as code / configuration management",
	"helm":       "Kubernetes package manager",
	"terraform":  "infrastructure as code",
	"vagrant":    "development environment provisioning",

	// Cloud providers / hosting
	"alibabacloud": "cloud provider",
	"digitalocean": "cloud provider",
	"googlecloud":  "cloud provider",
	"heroku":       "PaaS / cloud hosting",
	"hetzner":      "cloud / bare-metal hosting",
	"netlify":      "JAMstack / edge hosting",
	"openstack":    "open-source cloud platform",
	"ovh":          "cloud provider",
	"proxmox":      "hypervisor / virtualization platform",
	"railway":      "PaaS / cloud hosting",
	"vercel":       "frontend cloud / edge hosting",

	// CDN / edge / networking
	"caddy":        "web server / reverse proxy",
	"cloudflare":   "CDN / edge / DDoS protection",
	"consul":       "service mesh / service discovery",
	"envoyproxy":   "service proxy / service mesh",
	"istio":        "service mesh",
	"nginx":        "web server / reverse proxy / load balancer",
	"traefikproxy": "reverse proxy / load balancer",

	// Object storage
	"minio": "object storage",

	// Observability / monitoring / logging / tracing
	"datadog":       "observability / monitoring",
	"grafana":       "observability / dashboards",
	"kibana":        "log analytics / observability",
	"newrelic":      "observability / APM",
	"opentelemetry": "observability / distributed tracing",
	"pagerduty":     "incident management / alerting",
	"prometheus":    "metrics / monitoring",
	"sentry":        "error tracking / application monitoring",
	"splunk":        "logging / SIEM / observability",

	// ML frameworks / AI platforms
	"huggingface":  "ML / LLM platform",
	"jupyter":      "interactive computing / data science",
	"keras":        "ML framework",
	"langchain":    "LLM application framework",
	"mlflow":       "ML experiment tracking / model registry",
	"modal":        "cloud compute / model serving",
	"numpy":        "numerical computing library",
	"ollama":       "local LLM runtime",
	"openai":       "LLM / AI platform",
	"pandas":       "data analysis library",
	"pytorch":      "ML framework",
	"replicate":    "model serving / ML cloud",
	"scikitlearn":  "ML library",
	"tensorflow":   "ML framework",
	"anthropic":    "LLM / AI platform",

	// CI/CD
	"argo":          "GitOps / CI/CD",
	"bitbucket":     "version control / CI/CD",
	"circleci":      "CI/CD",
	"githubactions": "CI/CD",
	"gitlab":        "DevOps / version control / CI/CD",
	"jenkins":       "CI/CD",

	// Version control
	"git":    "version control",
	"gitea":  "self-hosted version control",
	"github": "version control / code hosting",

	// Identity / auth / secrets
	"auth0":  "identity / auth",
	"okta":   "identity / auth",
	"vault":  "secret management",

	// API / developer tools
	"graphql":          "API query language",
	"openapiinitiative": "API specification",
	"postman":          "API testing / development",
	"swagger":          "API documentation",
	"twilio":           "communications API",

	// Web frameworks / runtimes
	"angular":      "web framework (frontend)",
	"astro":        "web framework (static / SSR)",
	"bun":          "JavaScript runtime / package manager",
	"deno":         "JavaScript / TypeScript runtime",
	"django":       "web framework (Python)",
	"express":      "web framework (Node.js)",
	"fastapi":      "web framework (Python)",
	"flask":        "web framework (Python)",
	"laravel":      "web framework (PHP)",
	"nextdotjs":    "web framework (React / SSR)",
	"nodedotjs":    "JavaScript runtime",
	"nuxt":         "web framework (Vue / SSR)",
	"react":        "UI library (frontend)",
	"remix":        "web framework (React / SSR)",
	"rubyonrails":  "web framework (Ruby)",
	"spring":       "application framework (Java)",
	"svelte":       "web framework (frontend)",
	"vite":         "frontend build tool",
	"vuedotjs":     "web framework (frontend)",
	"rolldown":     "JavaScript bundler",

	// Programming languages / runtimes
	"c":          "programming language",
	"cplusplus":  "programming language",
	"dotnet":     "programming language / runtime",
	"go":         "programming language",
	"javascript": "programming language",
	"kotlin":     "programming language",
	"openjdk":    "programming language / runtime",
	"oxc":        "JavaScript toolchain",
	"php":        "programming language",
	"python":     "programming language",
	"ruby":       "programming language",
	"rust":       "programming language",
	"swift":      "programming language",
	"typescript": "programming language",
	"zig":        "programming language",

	// Operating systems / Linux distros
	"alpinelinux": "Linux distribution (container-optimized)",
	"archlinux":   "Linux distribution",
	"debian":      "Linux distribution",
	"freebsd":     "operating system",
	"linux":       "operating system",
	"redhat":      "Linux distribution / enterprise OS",
	"ubuntu":      "Linux distribution",

	// Data formats / standards
	"json": "data format",

	// Messaging / communication
	"discord":  "team messaging / community platform",
	"slack":    "team messaging",
	"telegram": "messaging platform",

	// Package registry / artifact
	"apache": "open-source software foundation",

	// Payments
	"stripe": "payment processing API",
}

// siDescription returns a human- and agent-readable description for the given
// Simple Icons slug. If a category is known, it is appended for semantic context.
func siDescription(slug string) string {
	title := siTitle(slug)
	if cat, ok := siCategories[slug]; ok {
		return title + " logo — " + cat + " (Simple Icons, CC0)"
	}
	return title + " logo (Simple Icons, CC0)"
}
