package journey

// BuiltinJourneys retorna as definições de journeys built-in da Workflow Platform
func BuiltinJourneys() []*JourneyDefinition {
	return []*JourneyDefinition{
		specBuilderJourney(),
		backlogRefinerJourney(),
		techAdvisorJourney(),
		deepDiveExplorerJourney(),
	}
}

// specBuilderJourney — Construção guiada de especificações (Socratic)
// Baseado no wtb-spec-builder (7 fases)
func specBuilderJourney() *JourneyDefinition {
	return &JourneyDefinition{
		Name:        "spec-builder",
		Title:       "Build a Specification",
		Description: "Guided Socratic flow to build project specifications from scratch. Asks progressive questions about context, scale, features, integrations, and NFRs.",
		Phases: []Phase{
			{
				ID: "context", Title: "Project Context", Order: 1,
				Description: "Understand what the project does, why it exists, and who uses it",
				Questions: []Question{
					{
						ID: "what", Prompt: "What does the system do?",
						WhyAsk: "Understanding the core purpose helps structure all subsequent decisions",
						Type: "multiline", Required: true,
						Placeholder: "Describe the main purpose of the system in 2-3 sentences",
					},
					{
						ID: "why", Prompt: "Why is this project needed?",
						WhyAsk: "The business motivation guides prioritization and scope decisions",
						Type: "multiline", Required: true,
						Placeholder: "What problem does it solve? What happens without it?",
					},
					{
						ID: "who", Prompt: "Who are the primary users?",
						WhyAsk: "User profiles shape UX decisions, access control, and feature priorities",
						Type: "text", Required: true,
						Placeholder: "e.g., Internal team (50 devs), External customers (10k+), Admin users",
					},
				},
			},
			{
				ID: "scale", Title: "Scale Estimation", Order: 2,
				Description: "Estimate volume and frequency to inform architecture decisions",
				Questions: []Question{
					{
						ID: "users_volume", Prompt: "How many concurrent users do you expect?",
						WhyAsk: "User volume directly impacts infrastructure, database, and caching decisions",
						Type: "select", Required: true,
						Options: []string{"< 100", "100 - 1,000", "1,000 - 10,000", "10,000 - 100,000", "100,000+"},
					},
					{
						ID: "data_volume", Prompt: "What's the expected data volume?",
						WhyAsk: "Data volume affects database choice, indexing strategy, and backup procedures",
						Type: "select", Required: false,
						Options: []string{"< 1 GB", "1 - 10 GB", "10 - 100 GB", "100 GB - 1 TB", "1 TB+"},
						Default: "1 - 10 GB",
					},
				},
			},
			{
				ID: "features", Title: "Core Features", Order: 3,
				Description: "Define the key features and functionalities",
				Questions: []Question{
					{
						ID: "core_features", Prompt: "What are the 3-5 most important features?",
						WhyAsk: "Core features become epics in the backlog, driving the project structure",
						Type: "multiline", Required: true,
						Placeholder: "List the main features, one per line",
					},
					{
						ID: "mvp_scope", Prompt: "What's the minimum viable scope for the first release?",
						WhyAsk: "MVP definition prevents scope creep and focuses the team on delivering value quickly",
						Type: "multiline", Required: true,
						Placeholder: "Which features are must-have vs nice-to-have?",
					},
				},
			},
			{
				ID: "integrations", Title: "Integrations", Order: 4,
				Description: "Identify external systems and dependencies",
				Questions: []Question{
					{
						ID: "external_systems", Prompt: "Which external systems does this integrate with?",
						WhyAsk: "Integration points are often the highest-risk areas and need careful planning",
						Type: "multiline", Required: false,
						Placeholder: "e.g., Payment gateway (Stripe), Auth (Auth0), Email (SendGrid)",
						Default: "None for MVP",
					},
					{
						ID: "data_sources", Prompt: "What existing data sources or APIs will be consumed?",
						WhyAsk: "Data dependencies affect the data model and may introduce external SLAs",
						Type: "text", Required: false,
						Default: "None",
					},
				},
			},
			{
				ID: "product", Title: "Product Implications", Order: 5,
				Description: "Understand product decisions that affect technical implementation",
				Questions: []Question{
					{
						ID: "retroactivity", Prompt: "Are there retroactive requirements (migrate existing data)?",
						WhyAsk: "Retroactive changes are 3-5x more expensive than greenfield — better to plan upfront",
						Type: "select", Required: true,
						Options: []string{"No retroactivity needed", "Minor data migration", "Full backward compatibility required"},
					},
					{
						ID: "cost_constraints", Prompt: "Are there budget or infrastructure constraints?",
						WhyAsk: "Budget constraints guide technology choices (managed vs self-hosted, cloud tier, etc.)",
						Type: "text", Required: false,
						Placeholder: "e.g., Max $500/month infra, must use existing AWS account",
						Default: "No specific constraints",
					},
				},
			},
			{
				ID: "nfrs", Title: "Non-Functional Requirements", Order: 6,
				Description: "Define quality attributes: performance, security, reliability",
				Questions: []Question{
					{
						ID: "performance", Prompt: "What are the performance requirements?",
						WhyAsk: "Performance targets shape architecture choices (sync vs async, caching, CDN)",
						Type: "text", Required: false,
						Placeholder: "e.g., < 200ms API response, < 3s page load",
						Default: "Standard web performance (< 500ms API, < 3s page load)",
					},
					{
						ID: "security", Prompt: "What security requirements apply?",
						WhyAsk: "Security requirements may add authentication, encryption, or compliance overhead",
						Type: "select", Required: true,
						Options: []string{"Basic (auth + HTTPS)", "Standard (+ RBAC + audit)", "High (+ encryption at rest + LGPD/GDPR)", "Critical (+ pen testing + SOC2)"},
					},
					{
						ID: "availability", Prompt: "What availability level is needed?",
						WhyAsk: "Availability targets determine redundancy, monitoring, and incident response needs",
						Type: "select", Required: false,
						Options: []string{"Best effort (dev/staging)", "99% (< 7h downtime/month)", "99.9% (< 45min/month)", "99.99% (< 5min/month)"},
						Default: "99% (< 7h downtime/month)",
					},
				},
			},
		},
	}
}

// backlogRefinerJourney — Refinamento interativo de épicos e histórias
func backlogRefinerJourney() *JourneyDefinition {
	return &JourneyDefinition{
		Name:        "backlog-refiner",
		Title:       "Refine Your Backlog",
		Description: "Interactively refine epics and stories with guided questions about scope, dependencies, and acceptance criteria.",
		Phases: []Phase{
			{
				ID: "scope", Title: "Scope Review", Order: 1,
				Description: "Review and validate the scope of each epic",
				Questions: []Question{
					{
						ID: "epic_review", Prompt: "Which epic do you want to refine?",
						WhyAsk: "Focusing on one epic at a time prevents context switching and improves quality",
						Type: "text", Required: true,
						Placeholder: "Epic name or ID (e.g., E1: Backend API)",
					},
					{
						ID: "scope_concerns", Prompt: "Are there any scope concerns with this epic?",
						WhyAsk: "Early scope identification prevents scope creep during implementation",
						Type: "multiline", Required: false,
						Placeholder: "e.g., Too broad, missing edge cases, unclear boundaries",
					},
				},
			},
			{
				ID: "dependencies", Title: "Dependencies", Order: 2,
				Description: "Identify and map dependencies between stories",
				Questions: []Question{
					{
						ID: "blockers", Prompt: "Are there any blocking dependencies?",
						WhyAsk: "Blocking dependencies determine the critical path and sprint planning order",
						Type: "multiline", Required: false,
						Placeholder: "e.g., Story X must complete before Story Y",
						Default: "No blocking dependencies identified",
					},
					{
						ID: "external_deps", Prompt: "Any external team or service dependencies?",
						WhyAsk: "External dependencies often cause delays — better to identify early and plan mitigations",
						Type: "text", Required: false,
						Default: "None",
					},
				},
			},
			{
				ID: "criteria", Title: "Acceptance Criteria", Order: 3,
				Description: "Strengthen acceptance criteria for each story",
				Questions: []Question{
					{
						ID: "missing_criteria", Prompt: "Are there stories with weak or missing acceptance criteria?",
						WhyAsk: "Clear acceptance criteria reduce ambiguity and prevent rework",
						Type: "multiline", Required: false,
						Placeholder: "List stories that need better acceptance criteria",
					},
					{
						ID: "edge_cases", Prompt: "What edge cases should be covered?",
						WhyAsk: "Edge cases are the most common source of bugs in production",
						Type: "multiline", Required: false,
						Placeholder: "e.g., Empty states, error handling, concurrent access, large payloads",
					},
				},
			},
		},
	}
}

// techAdvisorJourney — Escolha guiada de stack tecnológica
func techAdvisorJourney() *JourneyDefinition {
	return &JourneyDefinition{
		Name:        "tech-advisor",
		Title:       "Technology Stack Advisor",
		Description: "Guided exploration to choose the right technology stack based on project requirements, team skills, and constraints.",
		Phases: []Phase{
			{
				ID: "team", Title: "Team Profile", Order: 1,
				Description: "Understand team skills and experience",
				Questions: []Question{
					{
						ID: "team_size", Prompt: "What's the team size?",
						WhyAsk: "Team size affects technology complexity tolerance and coordination overhead",
						Type: "select", Required: true,
						Options: []string{"Solo developer", "2-3 developers", "4-8 developers", "9+ developers"},
					},
					{
						ID: "team_experience", Prompt: "What languages/frameworks does the team know well?",
						WhyAsk: "Leveraging existing skills reduces ramp-up time and increases delivery confidence",
						Type: "multiline", Required: true,
						Placeholder: "e.g., Go, TypeScript, React, PostgreSQL, Docker",
					},
				},
			},
			{
				ID: "constraints", Title: "Technical Constraints", Order: 2,
				Description: "Identify constraints that limit technology choices",
				Questions: []Question{
					{
						ID: "hosting", Prompt: "Where will this be hosted?",
						WhyAsk: "Hosting environment constrains runtime, databases, and deployment strategies",
						Type: "select", Required: true,
						Options: []string{"AWS", "GCP", "Azure", "On-premise", "Hybrid", "No preference"},
					},
					{
						ID: "existing_stack", Prompt: "Is there an existing tech stack that must be maintained?",
						WhyAsk: "Compatibility with existing systems may constrain choices but reduces integration risk",
						Type: "text", Required: false,
						Default: "Greenfield - no existing stack",
					},
				},
			},
			{
				ID: "priorities", Title: "Technical Priorities", Order: 3,
				Description: "Rank what matters most for the technology decision",
				Questions: []Question{
					{
						ID: "top_priority", Prompt: "What's the #1 technical priority?",
						WhyAsk: "The top priority becomes the primary decision driver when trade-offs are needed",
						Type: "select", Required: true,
						Options: []string{"Performance", "Developer productivity", "Scalability", "Time to market", "Long-term maintainability"},
					},
					{
						ID: "avoid", Prompt: "What should we explicitly avoid?",
						WhyAsk: "Anti-requirements are just as important as requirements for making good choices",
						Type: "text", Required: false,
						Placeholder: "e.g., No vendor lock-in, no Java, no microservices for MVP",
						Default: "No specific constraints",
					},
				},
			},
		},
	}
}

// deepDiveExplorerJourney — Exploração interativa de deep dives
func deepDiveExplorerJourney() *JourneyDefinition {
	return &JourneyDefinition{
		Name:        "deep-dive-explorer",
		Title:       "Deep Dive Explorer",
		Description: "Explore technologies in your backlog interactively. Ask contextual questions about architecture decisions, patterns, and trade-offs.",
		Phases: []Phase{
			{
				ID: "focus", Title: "Focus Area", Order: 1,
				Description: "Choose what technology or area to explore",
				Questions: []Question{
					{
						ID: "technology", Prompt: "Which technology do you want to explore?",
						WhyAsk: "Focusing on one technology allows deeper, more actionable insights",
						Type: "text", Required: true,
						Placeholder: "e.g., PostgreSQL, Docker, JWT, React",
					},
					{
						ID: "context", Prompt: "What's the specific context or concern?",
						WhyAsk: "Context-specific exploration generates more relevant recommendations than generic docs",
						Type: "multiline", Required: true,
						Placeholder: "e.g., Choosing between PostgreSQL and MongoDB for our user data",
					},
				},
			},
			{
				ID: "depth", Title: "Exploration Depth", Order: 2,
				Description: "Define what aspects to explore deeper",
				Questions: []Question{
					{
						ID: "aspects", Prompt: "Which aspects are most important to explore?",
						WhyAsk: "Prioritizing aspects ensures we focus on decisions that have the highest impact",
						Type: "select", Required: true,
						Options: []string{"Architecture patterns", "Performance trade-offs", "Security implications", "Operational complexity", "Cost analysis"},
					},
					{
						ID: "experience_level", Prompt: "What's your team's experience with this technology?",
						WhyAsk: "Experience level determines how much foundational vs advanced content is useful",
						Type: "select", Required: true,
						Options: []string{"Never used", "Basic knowledge", "Intermediate", "Advanced"},
					},
				},
			},
		},
	}
}
