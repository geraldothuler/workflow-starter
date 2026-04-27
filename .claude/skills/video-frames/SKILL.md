# Skill: video-frames

Extrai frames-chave de um vídeo MP4 para investigação visual de bugs — salva PNGs e lê
cada um com o Read tool para análise inline.

## Quando usar

- "extrai frames do vídeo"
- "analisa o vídeo"
- "vídeo com bug, quero ver os frames"
- Qualquer MP4 ou arquivo de vídeo enviado como evidência de bug/ticket

## Protocolo

### Passo 1 — probe do vídeo

```bash
ffprobe -v quiet -print_format json -show_streams -show_format "<video_path>" | \
  jq '{duration: .format.duration, fps: .streams[0].r_frame_rate, resolution: "\(.streams[0].width)x\(.streams[0].height)"}'
```

Informa: duração, FPS, resolução. Essencial para calibrar a estratégia de extração.

### Passo 2 — escolher estratégia

| Cenário | Estratégia |
|---------|-----------|
| Vídeo curto (< 30s) | Extrair 1 frame/segundo: `-vf fps=1` |
| Vídeo médio (30s–3min) | Extrair ~10–20 frames equidistantes: `-vf fps=1/N` |
| Bug pontual (timestamp conhecido) | Extrair ±5s ao redor do ponto: `-ss <ts> -t 10 -vf fps=2` |
| Longo + só I-frames | Extrair key frames: `-vf "select=eq(pict_type\,I)"` |

Default quando sem contexto: **~12 frames equidistantes** (fps=1/5 para vídeos até 1min).

### Passo 3 — extrair

```bash
OUTDIR=$(mktemp -d /tmp/frames_XXXXXX)

# Exemplo — 12 frames equidistantes:
ffmpeg -i "<video_path>" -vf fps=1/5 -q:v 2 "$OUTDIR/frame_%04d.png" -y 2>/dev/null

# Listar frames extraídos:
ls -1 "$OUTDIR/"
```

Flags importantes:
- `-q:v 2` → qualidade alta (escala 1–31, menor = melhor)
- `-y` → sobrescreve sem prompt
- `frame_%04d.png` → nomeação sequencial

### Passo 4 — ler e analisar

Para cada frame, usar o **Read tool** com o caminho absoluto. Claude vê a imagem diretamente.

```
Read(file_path="$OUTDIR/frame_0001.png")
Read(file_path="$OUTDIR/frame_0002.png")
...
```

Analisar em paralelo quando possível (múltiplos Read simultâneos).

### Passo 5 — descrever findings

Para cada frame relevante:
- Timestamp aproximado (frame N / fps → segundos)
- O que está visível / diferente do esperado
- Comportamento de UI, erro na tela, estado inesperado

Ao final: síntese de qual frame/intervalo contém o bug e hipótese de causa.

### Passo 6 — limpar (opcional)

```bash
rm -rf "$OUTDIR"
```

## Exemplo completo

```bash
# Ticket: charts não carregam no dashboard
VIDEO="/tmp/bug_dashboard.mp4"
OUTDIR=$(mktemp -d /tmp/frames_XXXXXX)

# Probe
ffprobe -v quiet -print_format json -show_streams "$VIDEO" | \
  jq '.format.duration, .streams[0].r_frame_rate'

# Extrair — vídeo de 45s, ~9 frames
ffmpeg -i "$VIDEO" -vf fps=1/5 -q:v 2 "$OUTDIR/frame_%04d.png" -y 2>/dev/null

ls "$OUTDIR/"
# → frame_0001.png frame_0002.png ... frame_0009.png
```

Em seguida: Read de cada frame para análise visual inline.

## Dicas

- **Bug de carregamento**: extrair o frame exato onde o spinner para ou o erro aparece — usar `-ss` se o timestamp é conhecido
- **Regressão visual**: comparar frames do vídeo com screenshot de referência lado a lado
- **Bug de transição**: fps=2 na janela do evento captura estados intermediários
- **Vídeo longo sem timestamp**: extrair key frames (I-frames) primeiro para mapear o conteúdo rapidamente, depois refinar na região suspeita
