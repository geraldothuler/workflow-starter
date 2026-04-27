  let BACKLOG_DATA = null;

  // ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  // i18n (v3.3)
  // ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  const I18N = {
    'pt-BR': {
      heroEyebrow: 'Workflow · Backlog Técnico',
      loadingKpis: 'Carregando KPIs...',
      loadingEpics: 'Carregando épicos...',
      loadingGeneric: 'Carregando...',
      errorLoading: 'Erro ao carregar dados',
      navOverview: 'Visão Geral',
      epicsSectionTitle: 'Épicos do Projeto',
      epicsSectionSubtitle: 'Visão geral dos principais componentes técnicos e suas entregas.',
      timelineSectionTitle: 'Linha do Tempo',
      timelineSectionSubtitle: 'Distribuição de épicos ao longo dos sprints planejados.',
      summaryTitle: 'Resumo Executivo',
      nextStepsTitle: 'Próximos Passos',
      planningSectionTitle: 'Planejamento',
      planningSectionSubtitle: 'Análise de esforço e distribuição por épico',
      metricsSectionTitle: 'Insights de Geração',
      metricsSectionSubtitle: 'Métricas do pipeline de geração e otimização',
      roadmapSectionTitle: 'Roadmap',
      roadmapSectionSubtitle: 'Milestones e entregas planejadas',
      // Planning labels
      totalSP: 'Total Story Points',
      estimateDays: 'Estimativa (Dias)',
      velocity: 'Velocity',
      spsPerSprint: 'SPs por sprint',
      stories: 'histórias',
      days: 'dias',
      sprints: 'sprints',
      ofTotal: 'do total',
      // Summary
      summaryStatEpics: 'Épicos',
      summaryStatStories: 'Histórias',
      summaryStatPoints: 'Story Points',
      summaryStatCriteria: 'Critérios',
      summaryTextTemplate: 'Backlog técnico completo com {epics} épicos principais, totalizando {stories} histórias técnicas e {points} story points. Cada história possui critérios de aceite mensuráveis para garantir qualidade na entrega.',
      nextStep1: 'Revisar priorização com stakeholders técnicos',
      nextStep2: 'Alocar time de desenvolvimento aos épicos',
      nextStep3: 'Iniciar sprint planning com histórias de alta prioridade',
      nextStep4: 'Configurar ambientes de desenvolvimento e CI/CD',
      // Deep dive
      ddpBadge: 'Deep Dive Contextualizado',
      ddpInThisStory: 'Nesta História',
      ddpWhatIs: 'O que é',
      ddpWhyInStory: 'Por que nesta história',
      ddpWhyInProject: 'Por que no projeto',
      ddpRelatedPatterns: 'Patterns Relacionados',
      ddpConfiguration: 'Configuração',
      ddpRecommendedPatterns: 'Padrões Recomendados',
      ddpTechnicalDecisions: 'Decisões Técnicas',
      ddpIn: 'em',
      // Epic detail
      epicStories: 'Histórias',
      epicPoints: 'SPs',
      // Metrics
      metricsTechsExtracted: 'Techs Extraídas',
      metricsTrivialFiltered: 'Triviais Filtradas',
      metricsLLMSaved: 'Chamadas LLM Economizadas',
      metricsTotalCost: 'Custo Total',
      metricsClassification: 'Classificação',
      // Effort/planning data unavailable
      effortNotAvailable: 'Dados de esforço não disponíveis',
      noMilestones: 'Nenhum milestone definido',
      milestoneStoryPoints: 'Story Points',
      milestoneEstimate: 'Estimativa',
      milestoneValue: 'Valor',
      timelineEpic: 'Épico',
      // Deep Dives section (overview)
      deepDivesSectionTitle: 'Deep Dives',
      deepDivesSectionSubtitle: 'Documentação técnica detalhada das tecnologias e padrões do projeto.',
      deepDivesCount: 'tecnologias documentadas',
      deepDiveScope: 'Escopo',
      deepDiveScopeEpic: 'Épico',
      deepDiveScopeStory: 'História',
      deepDiveScopeGlobal: 'Global',
      deepDiveStoryIndicator: 'deep dives'
    },
    'en': {
      heroEyebrow: 'Workflow · Technical Backlog',
      loadingKpis: 'Loading KPIs...',
      loadingEpics: 'Loading epics...',
      loadingGeneric: 'Loading...',
      errorLoading: 'Error loading data',
      navOverview: 'Overview',
      epicsSectionTitle: 'Project Epics',
      epicsSectionSubtitle: 'Overview of main technical components and their deliverables.',
      timelineSectionTitle: 'Timeline',
      timelineSectionSubtitle: 'Distribution of epics across planned sprints.',
      summaryTitle: 'Executive Summary',
      nextStepsTitle: 'Next Steps',
      planningSectionTitle: 'Planning',
      planningSectionSubtitle: 'Effort analysis and distribution by epic',
      metricsSectionTitle: 'Generation Insights',
      metricsSectionSubtitle: 'Generation pipeline metrics and optimization',
      roadmapSectionTitle: 'Roadmap',
      roadmapSectionSubtitle: 'Milestones and planned deliverables',
      // Planning labels
      totalSP: 'Total Story Points',
      estimateDays: 'Estimate (Days)',
      velocity: 'Velocity',
      spsPerSprint: 'SPs per sprint',
      stories: 'stories',
      days: 'days',
      sprints: 'sprints',
      ofTotal: 'of total',
      // Summary
      summaryStatEpics: 'Epics',
      summaryStatStories: 'Stories',
      summaryStatPoints: 'Story Points',
      summaryStatCriteria: 'Criteria',
      summaryTextTemplate: 'Complete technical backlog with {epics} main epics, totaling {stories} technical stories and {points} story points. Each story has measurable acceptance criteria to ensure delivery quality.',
      nextStep1: 'Review prioritization with technical stakeholders',
      nextStep2: 'Assign development team to epics',
      nextStep3: 'Start sprint planning with high-priority stories',
      nextStep4: 'Set up development environments and CI/CD',
      // Deep dive
      ddpBadge: 'Contextual Deep Dive',
      ddpInThisStory: 'In This Story',
      ddpWhatIs: 'What is it',
      ddpWhyInStory: 'Why in this story',
      ddpWhyInProject: 'Why in this project',
      ddpRelatedPatterns: 'Related Patterns',
      ddpConfiguration: 'Configuration',
      ddpRecommendedPatterns: 'Recommended Patterns',
      ddpTechnicalDecisions: 'Technical Decisions',
      ddpIn: 'in',
      // Epic detail
      epicStories: 'Stories',
      epicPoints: 'SPs',
      // Metrics
      metricsTechsExtracted: 'Techs Extracted',
      metricsTrivialFiltered: 'Trivial Filtered',
      metricsLLMSaved: 'LLM Calls Saved',
      metricsTotalCost: 'Total Cost',
      metricsClassification: 'Classification',
      // Effort/planning data unavailable
      effortNotAvailable: 'Effort data not available',
      noMilestones: 'No milestones defined',
      milestoneStoryPoints: 'Story Points',
      milestoneEstimate: 'Estimate',
      milestoneValue: 'Value',
      timelineEpic: 'Epic',
      // Deep Dives section (overview)
      deepDivesSectionTitle: 'Deep Dives',
      deepDivesSectionSubtitle: 'Detailed technical documentation of project technologies and patterns.',
      deepDivesCount: 'documented technologies',
      deepDiveScope: 'Scope',
      deepDiveScopeEpic: 'Epic',
      deepDiveScopeStory: 'Story',
      deepDiveScopeGlobal: 'Global',
      deepDiveStoryIndicator: 'deep dives'
    }
  };

  function getLang() {
    if (BACKLOG_DATA && BACKLOG_DATA.meta && BACKLOG_DATA.meta.lang) {
      return BACKLOG_DATA.meta.lang;
    }
    return 'pt-BR';
  }

  function t(key) {
    var lang = getLang();
    var dict = I18N[lang] || I18N['pt-BR'];
    return dict[key] || I18N['pt-BR'][key] || key;
  }

  function applyI18n() {
    // Atualizar lang do HTML
    document.documentElement.lang = getLang();

    // Section headers via IDs
    var mappings = {
      'heroEyebrow': 'heroEyebrow',
      'navOverview': 'navOverview',
      'epicsSectionTitle': 'epicsSectionTitle',
      'epicsSectionSubtitle': 'epicsSectionSubtitle',
      'timelineSectionTitle': 'timelineSectionTitle',
      'timelineSectionSubtitle': 'timelineSectionSubtitle',
      'summaryTitle': 'summaryTitle',
      'nextStepsTitle': 'nextStepsTitle',
      'planningSectionTitle': 'planningSectionTitle',
      'planningSectionSubtitle': 'planningSectionSubtitle',
      'metricsSectionTitle': 'metricsSectionTitle',
      'metricsSectionSubtitle': 'metricsSectionSubtitle',
      'roadmapSectionTitle': 'roadmapSectionTitle',
      'roadmapSectionSubtitle': 'roadmapSectionSubtitle',
      'deepDivesSectionTitle': 'deepDivesSectionTitle',
      'deepDivesSectionSubtitle': 'deepDivesSectionSubtitle'
    };

    Object.keys(mappings).forEach(function(elId) {
      var el = document.getElementById(elId);
      if (el) {
        el.textContent = t(mappings[elId]);
      }
    });
  }

  // Ícones por épico
  const EPIC_ICONS = {
    'E1': { icon: '🚀', bg: '#e8f5ee' },
    'E2': { icon: '🔄', bg: '#fdf8eb' },
    'E3': { icon: '📡', bg: '#edf8f3' },
    'E4': { icon: '🔧', bg: '#fdf0ef' },
    'E5': { icon: '📊', bg: '#f0f4ff' }
  };

  // Fetch data from API
  async function loadBacklog() {
    try {
      // Modo estático: dados já estão embedded no HTML
      if (window.STATIC_MODE && window.BACKLOG_DATA) {
        console.log('📦 Workflow Lens - Static Mode');
        console.log('   Loading from embedded data...');
        BACKLOG_DATA = window.BACKLOG_DATA;
        applyI18n();
        renderBacklog();
        renderTimeline();
        renderSummary();
        return;
      }

      // Modo servidor: fetch da API
      console.log('🌐 Workflow Lens - Server Mode');
      console.log('   Loading from API...');
      const response = await fetch('/api/backlog');
      BACKLOG_DATA = await response.json();
      applyI18n();
      renderBacklog();
      renderTimeline();
      renderSummary();
    } catch (error) {
      console.error('Erro ao carregar backlog:', error);
      document.getElementById('heroTitle').textContent = t('errorLoading');
      document.getElementById('kpiGrid').innerHTML = '<div class="loading">' + t('errorLoading') + '</div>';
    }
  }

  function renderBacklog() {
    const data = BACKLOG_DATA;

    // Render hero
    document.getElementById('heroTitle').textContent = data.meta.title;
    document.getElementById('heroSubtitle').textContent = data.meta.subtitle;

    // Render KPIs
    document.getElementById('kpiGrid').innerHTML = data.meta.kpis.map(kpi =>
      '<div class="kpi-card">' +
        '<div class="kpi-label">' + kpi.label + '</div>' +
        '<div class="kpi-value">' + kpi.value + '</div>' +
        '<div class="kpi-sub">' + kpi.sub + '</div>' +
      '</div>'
    ).join('');

    // Render epic cards
    document.getElementById('epicGrid').innerHTML = Object.entries(data.epics).map(([epicId, epic]) => {
      const iconData = EPIC_ICONS[epic.id] || { icon: '📦', bg: '#f0f0f0' };
      const progressPct = epic.priority === 'high' ? 75 : epic.priority === 'medium' ? 50 : 33;
      const progressClass = epic.priority === 'high' ? 'high' : epic.priority === 'medium' ? 'med' : 'low';

      return '<div class="epic-card" data-epic-id="' + epicId + '" onclick="showEpicDetail(\'' + epicId + '\')">' +
        '<div class="epic-head">' +
          '<div class="epic-icon" style="background:' + iconData.bg + '">' + iconData.icon + '</div>' +
          '<div class="epic-info">' +
            '<h3>' + epic.title + '</h3>' +
            '<p>' + epic.summary + '</p>' +
          '</div>' +
          '<div class="epic-arrow">→</div>' +
        '</div>' +
        '<div class="epic-progress">' +
          '<div class="epic-progress-fill ' + progressClass + '" style="width:' + progressPct + '%"></div>' +
        '</div>' +
      '</div>';
    }).join('');
  }

  function renderTimeline() {
    const epics = Object.values(BACKLOG_DATA.epics);
    const totalSprints = 8;

    document.getElementById('timelineSection').style.display = 'block';

    const gridCols = '200px ' + 'repeat(' + totalSprints + ', 1fr)';
    document.getElementById('timelineHeader').style.gridTemplateColumns = gridCols;

    let headerHtml = '<span>' + t('timelineEpic') + '</span>';
    for (let i = 1; i <= totalSprints; i++) {
      headerHtml += '<span>S' + i + '</span>';
    }
    document.getElementById('timelineHeader').innerHTML = headerHtml;

    let bodyHtml = '';
    epics.forEach((epic, idx) => {
      const startSprint = Math.floor((idx * totalSprints) / epics.length) + 1;
      const endSprint = Math.floor(((idx + 1) * totalSprints) / epics.length);
      const barClass = epic.priority === 'high' ? 'high' : epic.priority === 'medium' ? 'med' : 'low';
      const dotColor = epic.priority === 'high' ? 'var(--risk-high)' : epic.priority === 'medium' ? 'var(--risk-med)' : 'var(--accent)';

      bodyHtml += '<div class="timeline-row" style="grid-template-columns:' + gridCols + '">';
      bodyHtml += '<div class="tl-label">';
      bodyHtml += '<span class="tl-dot" style="background:' + dotColor + '"></span>';
      bodyHtml += epic.title;
      bodyHtml += '</div>';

      for (let i = 1; i <= totalSprints; i++) {
        if (i >= startSprint && i <= endSprint) {
          bodyHtml += '<div class="tl-cell"><div class="tl-bar ' + barClass + '"></div></div>';
        } else {
          bodyHtml += '<div class="tl-cell"></div>';
        }
      }
      bodyHtml += '</div>';
    });

    document.getElementById('timelineBody').innerHTML = bodyHtml;
  }

  function renderSummary() {
    const data = BACKLOG_DATA;
    const epics = Object.values(data.epics);

    document.getElementById('summaryCard').style.display = 'block';

    const totalStories = epics.reduce((sum, e) => sum + e.stories.length, 0);
    const totalPoints = epics.reduce((sum, e) => sum + e.stories.reduce((s, st) => s + st.effort, 0), 0);
    const totalCriteria = epics.reduce((sum, e) => sum + e.stories.reduce((s, st) => s + st.acceptance_criteria.length, 0), 0);

    document.getElementById('summaryStats').innerHTML =
      '<div><div class="s-stat-val">' + epics.length + '</div><div class="s-stat-lbl">' + t('summaryStatEpics') + '</div></div>' +
      '<div><div class="s-stat-val">' + totalStories + '</div><div class="s-stat-lbl">' + t('summaryStatStories') + '</div></div>' +
      '<div><div class="s-stat-val">' + totalPoints + '</div><div class="s-stat-lbl">' + t('summaryStatPoints') + '</div></div>' +
      '<div><div class="s-stat-val">' + totalCriteria + '</div><div class="s-stat-lbl">' + t('summaryStatCriteria') + '</div></div>';

    document.getElementById('summaryText').textContent =
      t('summaryTextTemplate')
        .replace('{epics}', epics.length)
        .replace('{stories}', totalStories)
        .replace('{points}', totalPoints);

    document.getElementById('nextSteps').innerHTML =
      '<div class="step"><span class="step-num">1</span>' + t('nextStep1') + '</div>' +
      '<div class="step"><span class="step-num">2</span>' + t('nextStep2') + '</div>' +
      '<div class="step"><span class="step-num">3</span>' + t('nextStep3') + '</div>' +
      '<div class="step"><span class="step-num">4</span>' + t('nextStep4') + '</div>';
  }

  function renderDeepDivesSection() {
    var section = document.getElementById('deepDivesSection');
    var container = document.getElementById('deepDivesGrid');
    if (!section || !container) return;

    var dives = BACKLOG_DATA.deep_dives;
    if (!dives || Object.keys(dives).length === 0) return;

    section.style.display = 'block';

    // Deduplicate by term (contextualized keys like "E2.1:JWT" share term "JWT")
    var seen = {};
    Object.keys(dives).forEach(function(key) {
      var dive = dives[key];
      var term = dive.term || (key.includes(':') ? key.split(':')[1] : key);
      if (!seen[term]) {
        seen[term] = { key: key, dive: dive, term: term };
      }
    });

    var html = '';
    Object.keys(seen).forEach(function(term) {
      var entry = seen[term];
      var dive = entry.dive;
      var cls = dive.classification || '';
      var clsBadge = cls ? '<span class="classification-badge cls-' + cls + '">' + cls + '</span>' : '';
      var scopeLabel = '';
      if (dive.scope === 'story') scopeLabel = t('deepDiveScopeStory');
      else if (dive.scope === 'epic') scopeLabel = t('deepDiveScopeEpic');
      else scopeLabel = t('deepDiveScopeGlobal');

      var storyCode = dive.story_id || '';
      var whatIs = dive.what_is || '';
      if (whatIs.length > 80) whatIs = whatIs.substring(0, 80) + '...';

      html += '<div class="dd-overview-card" onclick="showDeepDive(\'' + term + '\', \'' + storyCode + '\')">';
      html += '<div class="dd-overview-header">';
      html += '<span class="dd-overview-term">📚 ' + term + '</span>';
      html += clsBadge;
      html += '</div>';
      if (whatIs) {
        html += '<p class="dd-overview-desc">' + whatIs + '</p>';
      }
      html += '<div class="dd-overview-meta">';
      if (scopeLabel) {
        html += '<span class="dd-overview-scope">' + scopeLabel + '</span>';
      }
      if (storyCode) {
        html += '<span class="dd-overview-story">' + storyCode + '</span>';
      }
      html += '</div>';
      html += '</div>';
    });

    container.innerHTML = html;
  }

  function showEpicList() {
    document.getElementById('viewEpicList').style.display = 'block';
    document.getElementById('viewEpicDetail').style.display = 'none';
    document.getElementById('viewEpicDetail').classList.remove('active');
    document.getElementById('navOverview').classList.add('active');
    document.getElementById('navEpic').style.display = 'none';
    document.getElementById('navSep').style.display = 'none';
  }

  // Helper: get classification for a deep dive matching a tech
  function getClassification(tech, storyCode) {
    var contextKey = storyCode + ':' + tech;
    var dive = BACKLOG_DATA.deep_dives[contextKey] || BACKLOG_DATA.deep_dives[tech];
    if (dive && dive.classification) {
      return dive.classification;
    }
    return null;
  }

  function showEpicDetail(epicId) {
    const epic = BACKLOG_DATA.epics[epicId];
    if (!epic) return;

    document.getElementById('viewEpicList').style.display = 'none';
    document.getElementById('navOverview').classList.remove('active');
    document.getElementById('navEpic').textContent = epic.title;
    document.getElementById('navEpic').style.display = 'inline-block';
    document.getElementById('navSep').style.display = 'inline';

    let html = '<div class="epic-detail-header">';
    html += '<button class="back-btn" onclick="showEpicList()">←</button>';
    html += '<h2 style="font-family:\'DM Serif Display\',serif;font-size:26px;color:#1a2633;">' + epic.title + '</h2>';
    html += '</div>';

    // Epic stats bar (v3.3)
    var totalSPs = epic.stories.reduce(function(sum, s) { return sum + s.effort; }, 0);
    var priorityClass = epic.priority === 'high' ? 'eds-priority-high' : epic.priority === 'medium' ? 'eds-priority-medium' : 'eds-priority-low';
    var priorityLabel = epic.priority || 'medium';

    html += '<div class="epic-detail-stats">';
    html += '<span class="eds-item">' + epic.stories.length + ' ' + t('epicStories') + '</span>';
    html += '<span class="eds-sep">·</span>';
    html += '<span class="eds-item">' + totalSPs + ' ' + t('epicPoints') + '</span>';
    html += '<span class="eds-sep">·</span>';
    html += '<span class="eds-item ' + priorityClass + '">' + priorityLabel + '</span>';
    html += '</div>';

    html += '<div class="epic-context"><p>' + epic.description + '</p></div>';

    html += '<div class="story-list">';
    epic.stories.forEach((story, idx) => {
      var ddCount = countStoryDeepDives(story);

      html += '<div class="story-card">';
      html += '<div class="story-head" onclick="toggleStory(this)">';
      html += '<div class="story-number">' + story.code + '</div>';
      html += '<div class="story-summary">';
      html += '<h4>' + story.title + '</h4>';
      html += '<p>' + (story.what.length > 80 ? story.what.substring(0, 80) + '...' : story.what) + '</p>';
      if (ddCount > 0) {
        html += '<span class="story-dd-indicator">📚 ' + ddCount + ' ' + t('deepDiveStoryIndicator') + '</span>';
      }
      html += '</div>';
      html += '<div class="story-expand-icon">▼</div>';
      html += '</div>';
      html += '<div class="story-detail"><div class="story-detail-inner">';
      html += '<div class="story-what">' + story.what + '</div>';
      html += '<ul class="checklist">';
      story.acceptance_criteria.forEach((ac) => {
        html += '<li>';
        html += '<span class="check-icon">✔</span>';
        html += '<span>' + ac + '</span>';

        // Classification badges (v3.3 - replaces fake GP/NFR/SPEC)
        const techsInCriteria = extractTechs(ac);
        techsInCriteria.forEach(tech => {
          const contextKey = story.code + ':' + tech;
          const hasDive = BACKLOG_DATA.deep_dives[contextKey] || BACKLOG_DATA.deep_dives[tech];

          if (hasDive) {
            var cls = hasDive.classification || '';
            var clsBadge = cls ? '<span class="classification-badge cls-' + cls + '">' + cls + '</span>' : '';
            html += '<span class="tech-tag" onclick="showDeepDive(\'' + tech + '\', \'' + story.code + '\')" title="Deep dive: ' + tech + '">';
            html += '📚 ' + tech;
            html += clsBadge;
            html += '</span>';
          }
        });

        html += '</li>';
      });
      html += '</ul>';

      // Tags no rodapé com classification badges (uses story.tags + deep dive matching)
      var allTechs = getStoryTechs(story);
      if (allTechs.length > 0) {
        html += '<div class="tag-row">';

        allTechs.forEach((tech) => {
          const contextKey = story.code + ':' + tech;
          const hasDive = BACKLOG_DATA.deep_dives[contextKey] || BACKLOG_DATA.deep_dives[tech];

          if (hasDive) {
            var cls = hasDive.classification || '';
            var clsBadge = cls ? ' <span class="classification-badge cls-' + cls + '" style="margin-left:6px">' + cls + '</span>' : '';
            html += '<span class="tag has-dive" onclick="showDeepDive(\'' + tech + '\', \'' + story.code + '\')">';
            html += tech;
            html += clsBadge;
            html += '</span>';
          } else {
            html += '<span class="tag">' + tech + '</span>';
          }
        });
        html += '</div>';
      }

      html += '</div></div>';
      html += '</div>';
    });
    html += '</div>';

    // Deep Dives Glossary for this epic (visible summary)
    var epicDDCount = 0;
    epic.stories.forEach(function(s) { epicDDCount += countStoryDeepDives(s); });
    if (epicDDCount > 0) {
      html += '<div class="epic-dd-summary">';
      html += '<h3 class="epic-dd-summary-title">📚 ' + t('deepDivesSectionTitle') + '</h3>';
      html += '<div class="epic-dd-tags">';
      var epicTechsSeen = new Set();
      epic.stories.forEach(function(s) {
        var techs = getStoryTechs(s);
        techs.forEach(function(tech) {
          var contextKey = s.code + ':' + tech;
          var hasDive = BACKLOG_DATA.deep_dives[contextKey] || BACKLOG_DATA.deep_dives[tech];
          if (hasDive && !epicTechsSeen.has(tech)) {
            epicTechsSeen.add(tech);
            var cls = hasDive.classification || '';
            var clsBadge = cls ? ' <span class="classification-badge cls-' + cls + '">' + cls + '</span>' : '';
            html += '<span class="tag has-dive" onclick="showDeepDive(\'' + tech + '\', \'' + s.code + '\')">';
            html += tech + clsBadge;
            html += '</span>';
          }
        });
      });
      html += '</div>';
      html += '</div>';
    }

    const detailView = document.getElementById('viewEpicDetail');
    detailView.innerHTML = html;
    detailView.style.display = 'block';
    detailView.classList.add('active');
  }

  function extractTechs(text) {
    const availableTechs = new Set();

    Object.keys(BACKLOG_DATA.deep_dives || {}).forEach(key => {
      const tech = key.includes(':') ? key.split(':')[1] : key;
      availableTechs.add(tech);
    });

    const textLower = text.toLowerCase();
    return Array.from(availableTechs).filter(tech =>
      textLower.includes(tech.toLowerCase())
    );
  }

  // Merge story.tags with extractTechs for complete tech coverage
  function getStoryTechs(story) {
    var techs = new Set();

    // 1. From story.tags (primary - already identified by generator)
    if (story.tags && story.tags.length > 0) {
      story.tags.forEach(function(tag) {
        if (typeof tag === 'string') techs.add(tag);
      });
    }

    // 2. From deep_dives text matching (augments with any missed techs)
    var textTechs = extractTechs(story.what + ' ' + story.acceptance_criteria.join(' '));
    textTechs.forEach(function(t) { techs.add(t); });

    return Array.from(techs);
  }

  // Count deep dives relevant to a story
  function countStoryDeepDives(story) {
    if (!BACKLOG_DATA.deep_dives || Object.keys(BACKLOG_DATA.deep_dives).length === 0) return 0;
    var count = 0;
    var techs = getStoryTechs(story);
    techs.forEach(function(tech) {
      var contextKey = story.code + ':' + tech;
      if (BACKLOG_DATA.deep_dives[contextKey] || BACKLOG_DATA.deep_dives[tech]) {
        count++;
      }
    });
    return count;
  }

  function getStoryTitle(storyId) {
    var epics = BACKLOG_DATA.epics;
    for (var key in epics) {
      var epic = epics[key];
      for (var i = 0; i < epic.stories.length; i++) {
        if (epic.stories[i].code === storyId || epic.stories[i].id === storyId) {
          return epic.stories[i].title;
        }
      }
    }
    return null;
  }

  function showDeepDive(tech, storyCode) {
    const contextKey = storyCode + ':' + tech;
    let dive = BACKLOG_DATA.deep_dives[contextKey] || BACKLOG_DATA.deep_dives[tech];

    if (!dive) return;

    let title = tech;
    if (dive.story_id) {
      const storyTitle = getStoryTitle(dive.story_id);
      if (storyTitle) {
        title += ' ' + t('ddpIn') + ' ' + dive.story_id + ' - ' + storyTitle;
      } else {
        title += ' ' + t('ddpIn') + ' ' + dive.story_id;
      }
    }
    document.getElementById('ddpTitle').textContent = title;

    let html = '<div class="ddp-badge">' + t('ddpBadge') + '</div>';

    // Classification badge in panel
    if (dive.classification) {
      html += '<div class="ddp-classification">';
      html += '<span class="classification-badge cls-' + dive.classification + '">' + dive.classification + '</span>';
      if (dive.scope) {
        html += '<span class="ddp-scope">' + dive.scope + '</span>';
      }
      html += '</div>';
    }

    if (dive.what_in_this_story) {
      html += '<div class="ddp-section">';
      html += '<h4>' + t('ddpInThisStory') + '</h4>';
      html += '<p>' + dive.what_in_this_story + '</p>';
      html += '</div>';
    }

    if (dive.what_is) {
      html += '<div class="ddp-section">';
      html += '<h4>' + t('ddpWhatIs') + '</h4>';
      html += '<p>' + dive.what_is + '</p>';
      html += '</div>';
    }

    if (dive.why_in_this_story) {
      html += '<div class="ddp-section">';
      html += '<h4>' + t('ddpWhyInStory') + '</h4>';
      html += '<p>' + dive.why_in_this_story + '</p>';
      html += '</div>';
    } else if (dive.why_here) {
      html += '<div class="ddp-section">';
      html += '<h4>' + t('ddpWhyInProject') + '</h4>';
      html += '<p>' + dive.why_here + '</p>';
      html += '</div>';
    }

    if (dive.source_patterns && dive.source_patterns.length > 0) {
      html += '<div class="ddp-section">';
      html += '<h4>' + t('ddpRelatedPatterns') + '</h4>';
      html += '<ul class="ddp-list">';
      dive.source_patterns.forEach(p => {
        html += '<li><strong>' + p + '</strong></li>';
      });
      html += '</ul>';
      html += '</div>';
    }

    if (dive.configuration) {
      html += '<div class="ddp-section">';
      html += '<h4>' + t('ddpConfiguration') + '</h4>';
      html += '<p>' + dive.configuration + '</p>';
      html += '</div>';
    }

    if (dive.patterns && dive.patterns.length > 0) {
      html += '<div class="ddp-section">';
      html += '<h4>' + t('ddpRecommendedPatterns') + '</h4>';
      html += '<ul class="ddp-list">';
      dive.patterns.forEach(p => {
        html += '<li>' + p + '</li>';
      });
      html += '</ul>';
      html += '</div>';
    }

    if (dive.decisions && dive.decisions.length > 0) {
      html += '<div class="ddp-section">';
      html += '<h4>' + t('ddpTechnicalDecisions') + '</h4>';
      html += '<ul class="ddp-list">';
      dive.decisions.forEach(d => {
        html += '<li>' + d + '</li>';
      });
      html += '</ul>';
      html += '</div>';
    }

    document.getElementById('ddpBody').innerHTML = html;
    document.getElementById('deepDivePanel').classList.add('open');
    document.getElementById('ddpOverlay').classList.add('visible');
    document.body.style.overflow = 'hidden';
  }

  function closeDeepDive() {
    document.getElementById('deepDivePanel').classList.remove('open');
    document.getElementById('ddpOverlay').classList.remove('visible');
    document.body.style.overflow = '';
  }

  function toggleStory(el) {
    el.closest('.story-card').classList.toggle('open');
  }

  // Intersection Observer para fade-in
  function initFadeInObserver() {
    const observer = new IntersectionObserver((entries) => {
      entries.forEach(entry => {
        if (entry.isIntersecting) {
          entry.target.classList.add('visible');
        }
      });
    }, {
      threshold: 0.1,
      rootMargin: '0px 0px -50px 0px'
    });

    document.querySelectorAll('.fade-in').forEach(el => {
      observer.observe(el);
    });
  }

  // Load on page load
  loadBacklog().then(() => {
    renderDeepDivesSection();
    renderPlanningDashboard();
    renderMilestonesRoadmap();
    renderMetricsDashboard();

    setTimeout(initFadeInObserver, 100);
  });

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// PLANNING & EFFORT RENDERING (v3.2)
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

function renderPlanningDashboard() {
  const container = document.getElementById('planning-dashboard');
  if (!container) return;

  const effort = BACKLOG_DATA.effort;
  if (!effort) {
    container.innerHTML = '<p style="color: var(--ink-muted);">' + t('effortNotAvailable') + '</p>';
    return;
  }

  var overviewHTML = '<div class="effort-overview">' +
    '<div class="effort-card">' +
      '<div class="effort-card-label">' + t('totalSP') + '</div>' +
      '<div class="effort-card-value">' + effort.total_sps + '</div>' +
      '<div class="effort-card-sub">' + effort.total_stories + ' ' + t('stories') + '</div>' +
    '</div>' +
    '<div class="effort-card">' +
      '<div class="effort-card-label">' + t('estimateDays') + '</div>' +
      '<div class="effort-card-value">' + effort.total_days + '</div>' +
      '<div class="effort-card-sub">~' + Math.ceil(effort.total_days / 10) + ' ' + t('sprints') + '</div>' +
    '</div>' +
    '<div class="effort-card">' +
      '<div class="effort-card-label">' + t('velocity') + '</div>' +
      '<div class="effort-card-value">' + effort.velocity + '</div>' +
      '<div class="effort-card-sub">' + t('spsPerSprint') + '</div>' +
    '</div>' +
  '</div>';

  var epicsHTML = '<div class="epic-effort-list">';

  if (effort.by_epic) {
    var sortedEpics = Object.entries(effort.by_epic).sort(function(a, b) {
      return b[1].sps - a[1].sps;
    });

    sortedEpics.forEach(function(entry) {
      var epicId = entry[0];
      var epicEffort = entry[1];
      var epic = BACKLOG_DATA.epics[epicId];
      var title = epic ? epic.title : epicId;
      var percentage = epicEffort.percentage || 0;

      epicsHTML += '<div class="epic-effort-item">' +
        '<div class="epic-effort-header">' +
          '<div class="epic-effort-title">' + title + '</div>' +
          '<div class="epic-effort-sps">' + epicEffort.sps + ' SPs</div>' +
        '</div>' +
        '<div class="effort-progress-bar">' +
          '<div class="effort-progress-fill" style="width: ' + percentage + '%"></div>' +
        '</div>' +
        '<div class="epic-effort-meta">' +
          epicEffort.stories + ' ' + t('stories') + ' · ~' + epicEffort.days + ' ' + t('days') + ' · ' + percentage.toFixed(1) + '% ' + t('ofTotal') +
        '</div>' +
      '</div>';
    });
  }

  epicsHTML += '</div>';

  container.innerHTML = overviewHTML + epicsHTML;

  setTimeout(function() {
    document.querySelectorAll('.effort-progress-fill').forEach(function(fill) {
      fill.style.width = fill.style.width;
    });
  }, 100);
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// METRICS DASHBOARD (v3.3)
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

function renderMetricsDashboard() {
  var section = document.getElementById('metricsSection');
  var container = document.getElementById('metrics-dashboard');
  if (!section || !container) return;

  var metrics = BACKLOG_DATA.metrics;
  if (!metrics) return;

  // Show section
  section.style.display = 'block';

  var html = '<div class="metrics-grid">';

  // Card 1: Techs Extracted
  html += '<div class="metrics-card">' +
    '<div class="metrics-card-icon">🔍</div>' +
    '<div class="metrics-card-value">' + metrics.total_techs_extracted + '</div>' +
    '<div class="metrics-card-label">' + t('metricsTechsExtracted') + '</div>' +
  '</div>';

  // Card 2: Trivial Filtered
  html += '<div class="metrics-card">' +
    '<div class="metrics-card-icon">🚫</div>' +
    '<div class="metrics-card-value">' + metrics.trivial_filtered + '</div>' +
    '<div class="metrics-card-label">' + t('metricsTrivialFiltered') + '</div>' +
  '</div>';

  // Card 3: LLM Calls Saved
  var savedPct = metrics.reduction_percent ? metrics.reduction_percent.toFixed(0) + '%' : '0%';
  html += '<div class="metrics-card metrics-card-highlight">' +
    '<div class="metrics-card-icon">⚡</div>' +
    '<div class="metrics-card-value">' + metrics.llm_calls_saved + '</div>' +
    '<div class="metrics-card-label">' + t('metricsLLMSaved') + ' (' + savedPct + ')</div>' +
  '</div>';

  // Card 4: Total Cost
  var cost = metrics.total_cost ? '$' + metrics.total_cost.toFixed(2) : '$0.00';
  html += '<div class="metrics-card">' +
    '<div class="metrics-card-icon">💰</div>' +
    '<div class="metrics-card-value">' + cost + '</div>' +
    '<div class="metrics-card-label">' + t('metricsTotalCost') + '</div>' +
  '</div>';

  html += '</div>';

  // Classification breakdown
  if (metrics.classification_stats) {
    html += '<div class="metrics-classification">';
    html += '<h4>' + t('metricsClassification') + '</h4>';
    html += '<div class="metrics-cls-tags">';
    var order = ['critical', 'specific', 'standard', 'trivial'];
    order.forEach(function(cls) {
      var count = metrics.classification_stats[cls] || 0;
      if (count > 0) {
        html += '<span class="metrics-cls-tag cls-' + cls + '">' + cls + ': ' + count + '</span>';
      }
    });
    html += '</div>';
    html += '</div>';
  }

  container.innerHTML = html;
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// MILESTONES ROADMAP RENDERING (v3.2)
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

function renderMilestonesRoadmap() {
  const container = document.getElementById('milestones-roadmap');
  if (!container) return;

  const milestones = BACKLOG_DATA.milestones;
  if (!milestones || milestones.length === 0) {
    container.innerHTML = '<p style="color: var(--ink-muted);">' + t('noMilestones') + '</p>';
    return;
  }

  var html = '<div class="milestones-timeline">';

  milestones.forEach(function(milestone, index) {
    var badgeLabel = milestone.id || ('M' + (index + 1));
    var epicTags = '';

    if (milestone.epic_ids) {
      milestone.epic_ids.forEach(function(epicId) {
        var epic = BACKLOG_DATA.epics[epicId];
        var title = epic ? epic.title : epicId;
        epicTags += '<span class="milestone-epic-tag">' + epicId + ': ' + title + '</span>';
      });
    }

    html += '<div class="milestone-item">' +
      '<div class="milestone-marker"></div>' +
      '<div class="milestone-content">' +
        '<div class="milestone-header">' +
          '<h3 class="milestone-title">' + milestone.title + '</h3>' +
          '<span class="milestone-badge">' + badgeLabel + '</span>' +
        '</div>';

    if (milestone.description) {
      html += '<div class="milestone-description">' + milestone.description + '</div>';
    }

    html += '<div class="milestone-meta">' +
      '<div class="milestone-meta-item">' +
        '<div class="milestone-meta-label">' + t('milestoneStoryPoints') + '</div>' +
        '<div class="milestone-meta-value">' + (milestone.total_sps || 0) + '</div>' +
      '</div>' +
      '<div class="milestone-meta-item">' +
        '<div class="milestone-meta-label">' + t('milestoneEstimate') + '</div>' +
        '<div class="milestone-meta-value">' + (milestone.days_estimate || 0) + ' ' + t('days') + '</div>' +
      '</div>' +
    '</div>';

    if (milestone.value_prop) {
      html += '<div style="margin-top: 12px; padding: 12px; background: var(--accent-light); border-radius: 6px; font-size: 13px; color: var(--ink);">' +
        '<strong>💡 ' + t('milestoneValue') + ':</strong> ' + milestone.value_prop +
      '</div>';
    }

    if (epicTags) {
      html += '<div class="milestone-epics">' + epicTags + '</div>';
    }

    html += '</div></div>';
  });

  html += '</div>';
  container.innerHTML = html;
}
