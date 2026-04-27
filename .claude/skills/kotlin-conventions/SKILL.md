---
name: kotlin-conventions
description: Kotlin/Gradle conventions for Cobli repos (fusca, iris, webhook-sender). Apply automatically when working with .kt files, build.gradle.kts, or Spring Boot modules.
user-invocable: false
---

# Kotlin Conventions — Cobli Repos

## Commit sequence — mandatory

`ktlint → unit tests → intTests → commit`. Never skip a step.

| Repo | Java | Command |
|------|------|---------|
| fusca, iris | 21.0.7-tem | `./gradlew ktlintCheck test intTest` |
| webhook-sender | 17.0.16-tem | `./gradlew :webhook-sender:ktlintCheck :webhook-sender:test` |

```bash
# Padrão: usar sdkman para trocar Java antes de rodar Gradle
source ~/.sdkman/bin/sdkman-init.sh && sdk use java 21.0.7-tem

# Depois rodar normalmente
cd <repo> && ./gradlew ktlintCheck test intTest

# Alternativa direta (Bash tool — single command chain):
source ~/.sdkman/bin/sdkman-init.sh && sdk use java 21.0.7-tem && cd <repo> && ./gradlew ktlintCheck test intTest
```

**Regra:** sempre usar `sdk use java <version>` via sdkman — nunca setar `JAVA_HOME` manualmente. Verificar versão disponível com `sdk list java | grep installed` se houver dúvida.

Pre-existing failure is not a skip pass — confirm with `git stash && ./gradlew test && git stash pop`, then fix regardless.

## ktlint — most common violations

```kotlin
// multiline-expression-wrapping: lambda must start on new line
val x =              // ✓
    mockk<Foo> {
        every { bar() } returns baz
    }
val x = mockk<Foo> { // ✗ ktlint violation
```

Also: `${var}` → `$var` in string templates (redundant curly braces).

## Mock framework — read build.gradle.kts first

- `io.mockk:mockk` → MockK
- `org.mockito.kotlin` → Mockito

**MockK + Java types returning MutableIterator:**
```kotlin
every { iterator() } returns mutableListOf(row).iterator()  // ✓
every { iterator() } returns listOf(row).iterator()         // ✗ type mismatch
```

## fusca-shared — hard Cassandra dependency

Any module using `fusca-shared` needs Cassandra running — even if the module doesn't use it directly. `fusca-shared` has concrete `@Repository` beans with `CassandraOperations` in constructors.

**New fusca module checklist:**
1. `SessionProviderImpl` returning `null` (copy from `fusca-identification-token`)
2. `application-local.properties` with `spring.cassandra.*` → localhost:9042
3. Application test in `src/intTest/kotlin` (not `src/test/kotlin`)
4. `spring.profiles.active=local` in `application.properties`
5. `spring.jpa.open-in-view=false` — mandatory
6. CircleCI: parameter + skip-test + test + deploy + monitor workflows in `continue_config.yml`

**intTest startup — run with docker compose:**
```bash
colima start  # if "Cannot connect to Docker daemon"
docker compose up -d postgres kafka cassandra
```

**Error sequence without services:**
1. `PSQLException: Connection refused` → start postgres
2. `AllNodesFailedException` → start cassandra
3. `NoSuchBeanDefinitionException: SessionProvider` → add SessionProviderImpl
4. `NoSuchBeanDefinitionException: CassandraOperations` → don't exclude Cassandra from autoconfigure

## OOM diagnosis heuristics

- Exit code 3 = JVM OOM (heap exhausted, `-Xmx` reached)
- Exit code 137 = container OOMKill (cgroup limit)
- Restarts every N exact minutes → suspect cron/scheduled job
- Always `GROUP BY status` before filtering — never assume status names without checking DB
